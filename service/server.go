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
	cl "github.com/zauberhaus/rest2dhcp/client"
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

	ServiceVersion *VersionInfo
)

type Server struct {
	http.Server
	client  *cl.Client
	timeout time.Duration
	Done    chan bool
}

func NewServer(local net.IP, remote net.IP, mode cl.ConnectionType, addr string, timeout time.Duration) *Server {
	server := Server{}
	server.Addr = addr
	server.Done = make(chan bool)
	server.timeout = timeout

	server.client = cl.NewClient(local, remote, mode)

	router := mux.NewRouter().StrictSlash(true)
	router.Use(server.ContentMiddleware)
	router.Use(server.CounterMiddleware)
	router.Use(server.PrometheusMiddleware)
	server.setup(router)

	server.Handler = handlers.CombinedLoggingHandler(os.Stderr, router)

	return &server
}

func RunServer(cmd *cobra.Command, args []string) {

	remote, _ := cmd.Flags().GetIP("server")
	local, _ := cmd.Flags().GetIP("client")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	listen, _ := cmd.Flags().GetString("listen")

	mode := cl.DefaultRelay

	modetxt, _ := cmd.Flags().GetString("mode")
	switch modetxt {
	case "auto":
		mode = cl.AutoDetect
	case "relay":
		mode = cl.DefaultRelay
	case "fritzbox":
		mode = cl.Fritzbox
	case "android":
		mode = cl.BrokenRelay
	default:
		fmt.Printf("Unknown DHCP mode: %s", modetxt)
		cmd.Usage()
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	server := NewServer(local, remote, mode, listen, timeout)
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
	contentType := ContentType(r.Context().Value(Content).(string))
	s.write(w, ServiceVersion, contentType)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	result := Result{
		Lease{
			Hostname: query.Hostname,
			Mac:      MAC{lease.ClientHWAddr},
			IP:       lease.YourClientIP.String(),
			Renew:    lease.GetRenewalTime(),
			Expire:   lease.GetExpireTime(),
		},
	}

	contentType := ContentType(r.Context().Value(Content).(string))
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

	result := Result{
		Lease{
			Hostname: query.Hostname,
			Mac:      MAC{lease.ClientHWAddr},
			IP:       lease.YourClientIP.String(),
			Renew:    lease.GetRenewalTime(),
			Expire:   lease.GetExpireTime(),
		},
	}

	contentType := ContentType(r.Context().Value(Content).(string))
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

func (s *Server) write(w http.ResponseWriter, value interface{}, t ContentType) error {

	switch t {
	case JSON:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set("Content-Type", JSON)
		_, err = w.Write(data)
		return err
	case XML:
		data, err := xml.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set("Content-Type", XML)
		_, err = w.Write(data)
		return err
	case YAML:
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", YAML)
		_, err = w.Write(data)
		return err
	}

	return fmt.Errorf("Unknown content format: %v", t)
}

func (s *Server) ContentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := r.Header.Get("Accept")

		if strings.Contains(content, JSON) {
			ctx := context.WithValue(r.Context(), Content, JSON)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, YAML) {
			ctx := context.WithValue(r.Context(), Content, YAML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, XML) {
			ctx := context.WithValue(r.Context(), Content, XML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if content == "" || content == "*/*" {
			ctx := context.WithValue(r.Context(), Content, YAML)
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
