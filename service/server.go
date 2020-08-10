package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"gopkg.in/yaml.v3"
)

var (
	accept = regexp.MustCompile(`application/[json|yaml]`)

	ipExp  = "{ip:\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}}"
	macExp = "{mac:(?:[0-9A-Fa-f]{2}[:-]){5}(?:[0-9A-Fa-f]{2})}"

	hostnameExp = regexp.MustCompile("^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$")

	renewPath = map[string]string{
		"renew": fmt.Sprintf("/ip/{hostname}/%s/%s", macExp, ipExp),
	}

	releasePath = map[string]string{
		"release": fmt.Sprintf("/ip/{hostname}/%s/%s", macExp, ipExp),
	}

	leasePath = map[string]string{
		"get":        fmt.Sprintf("/ip/{hostname}"),
		"getWithMac": fmt.Sprintf("/ip/{hostname}/%s", macExp),
	}

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "rest2dhcp_http_duration_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"action"})

	httpCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rest2dhcp_http_counter",
		Help: "Counter for HTTP requests",
	}, []string{"code"})

	Version *client.Version
)

type Server struct {
	http.Server
	client  *dhcp.Client
	timeout time.Duration
	Done    chan bool
}

func NewServer(local net.IP, remote net.IP, relay net.IP, mode dhcp.ConnectionType, addr string, timeout time.Duration) *Server {
	server := Server{}
	server.Addr = addr
	server.Done = make(chan bool)
	server.timeout = timeout

	server.client = dhcp.NewClient(local, remote, relay, mode)

	router := mux.NewRouter().StrictSlash(true)
	router.Use(server.ContentMiddleware)
	router.Use(server.CounterMiddleware)
	router.Use(server.PrometheusMiddleware)
	server.setup(router)

	server.Handler = handlers.CombinedLoggingHandler(os.Stderr, router)

	log.Printf("Version:\n%v\n", Version)

	Version.DHCPServer = server.client.GetDHCPServerIP()
	Version.RelayIP = server.client.GetDHCPRelayIP()
	Version.Mode = server.client.GetDHCPRelayMode()

	return &server
}

func RunServer(cmd *cobra.Command, args []string) {

	remote, _ := cmd.Flags().GetIP("server")
	local, _ := cmd.Flags().GetIP("client")
	relay, _ := cmd.Flags().GetIP("relay")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	listen, _ := cmd.Flags().GetString("listen")

	mode := dhcp.DefaultRelay

	modetxt, _ := cmd.Flags().GetString("mode")
	err := mode.Parse(modetxt)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
		cmd.Usage()
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	server := NewServer(local, remote, relay, mode, listen, timeout)
	server.Start(ctx)

	signal := <-done
	log.Printf("Got %v", signal.String())

	cancel()

	<-server.Done
	log.Printf("Done.")
}

func (s *Server) Start(ctx context.Context) {
	go func() {
		s.client.Start()

		go func() {
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}()

		log.Print("Server Started")
		<-ctx.Done()
		s.client.Stop()

		if err := s.Shutdown(ctx); err != nil {
			log.Fatalf("Server Shutdown Failed:%+v", err)
		}

		log.Print("Server stopped")

		close(s.Done)
	}()
}

func (s *Server) setup(router *mux.Router) {
	router.
		Name("version").
		Methods("GET").
		Path("/version").
		HandlerFunc(s.version)

	router.
		Name("/metrics").
		Methods("GET").
		Path("/metrics").
		Handler(promhttp.Handler())

	for k, p := range renewPath {
		router.
			Name(k).
			Methods("GET").
			Path(p).
			HandlerFunc(s.renew)
	}

	for k, p := range releasePath {
		router.
			Name(k).
			Methods("DELETE").
			Path(p).
			HandlerFunc(s.release)
	}

	for k, p := range leasePath {
		router.
			Name(k).
			Methods("GET").
			Path(p).
			HandlerFunc(s.get)
	}

}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	contentType := client.ContentType(r.Context().Value(Content).(string))

	s.write(w, &client.VersionInfo{ServiceVersion: Version}, contentType)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	lease := <-s.client.GetLease(ctx, query.Hostname, query.Mac.HardwareAddr)

	if lease == nil {
		HttpError(w, http.StatusRequestTimeout)
		return
	}

	if !lease.Ok() {
		http.Error(w, lease.Error().Error(), http.StatusBadRequest)
		return
	}

	result := client.Result{
		Lease: client.NewLease(query.Hostname, *lease.DHCP4),
	}

	contentType := client.ContentType(r.Context().Value(Content).(string))
	s.write(w, result, contentType)
}

func (s *Server) renew(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	lease := <-s.client.ReNew(ctx, query.Hostname, query.Mac.HardwareAddr, query.IP)

	if lease == nil {
		HttpError(w, http.StatusRequestTimeout)
		return
	}

	if !lease.Ok() {
		http.Error(w, lease.Error().Error(), http.StatusBadRequest)
		return
	}

	result := client.Result{
		Lease: client.NewLease(query.Hostname, *lease.DHCP4),
	}

	contentType := client.ContentType(r.Context().Value(Content).(string))
	s.write(w, result, contentType)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	lease := <-s.client.Release(ctx, query.Hostname, query.Mac.HardwareAddr, query.IP)

	if lease != nil {
		HttpError(w, http.StatusRequestTimeout)
		return
	}

	w.Write([]byte("Ok.\n"))
}

func (s *Server) write(w http.ResponseWriter, value interface{}, t client.ContentType) error {

	switch t {
	case client.JSON:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set("Content-Type", client.JSON)
		_, err = w.Write(data)
		return err
	case client.XML:
		data, err := xml.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set("Content-Type", client.XML)
		_, err = w.Write(data)
		return err
	case client.YAML:
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", client.YAML)
		_, err = w.Write(data)
		return err
	}

	return fmt.Errorf("Unknown content format: %v", t)
}

func (s *Server) ContentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := r.Header.Get("Accept")

		if strings.Contains(content, client.JSON) {
			ctx := context.WithValue(r.Context(), Content, client.JSON)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, client.YAML) {
			ctx := context.WithValue(r.Context(), Content, client.YAML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, client.XML) {
			ctx := context.WithValue(r.Context(), Content, client.XML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if content == "" || content == "*/*" {
			ctx := context.WithValue(r.Context(), Content, client.YAML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			HttpError(w, http.StatusUnsupportedMediaType)
		}
	})
}

func (s *Server) PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(route.GetName()))
		next.ServeHTTP(w, r)
		timer.ObserveDuration()
	})
}

func (s *Server) CounterMiddleware(next http.Handler) http.Handler {
	return promhttp.InstrumentHandlerCounter(httpCounter, next)
}

func HttpError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}
