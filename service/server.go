package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	cl "github.com/zauberhaus/rest2dhcp/client"
	"gopkg.in/yaml.v3"
)

var (
	accept = regexp.MustCompile(`application/[json|yaml]`)

	ipExp  = "{ip:\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}}"
	macExp = "{mac:(?:[0-9A-Fa-f]{2}[:-]){5}(?:[0-9A-Fa-f]{2})}"

	hostnameExp = regexp.MustCompile("^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$")

	renewPath = []string{
		fmt.Sprintf("/{hostname}/%s/%s", macExp, ipExp),
	}

	releasePath = []string{
		fmt.Sprintf("/{hostname}/%s/%s", macExp, ipExp),
	}

	leasePath = []string{
		fmt.Sprintf("/{hostname}"),
		fmt.Sprintf("/{hostname}/%s", macExp),
	}
)

type Server struct {
	http.Server
	client *cl.Client
	Done   chan bool
}

func NewServer(addr string) *Server {
	server := Server{}
	server.Addr = addr
	server.Done = make(chan bool)

	server.client = cl.NewClient(nil, nil, cl.AutoDetect)

	router := mux.NewRouter().StrictSlash(true)
	router.Use(server.ContentMiddleware)
	server.setup(router)

	server.Handler = handlers.CombinedLoggingHandler(os.Stderr, router)

	return &server
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
	for _, p := range renewPath {
		router.
			Methods("GET").
			Path(p).
			HandlerFunc(s.renew)
	}

	for _, p := range releasePath {
		router.
			Methods("DELETE").
			Path(p).
			HandlerFunc(s.release)
	}

	for _, p := range leasePath {
		router.
			Methods("GET").
			Path(p).
			HandlerFunc(s.get)
	}
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	contentType := r.Context().Value(Content).(ContentType)
	s.write(w, result, contentType)
}

func (s *Server) renew(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	contentType := r.Context().Value(Content).(ContentType)
	s.write(w, result, contentType)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lease := <-s.client.Release(ctx, query.Hostname, query.Mac.HardwareAddr, query.IP)

	if lease != nil {
		HttpError(w, http.StatusRequestTimeout)
		return
	}

	w.Write([]byte("Ok."))
}

func (s *Server) write(w http.ResponseWriter, value interface{}, t ContentType) error {

	switch t {
	case JSON:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		return err
	case XML:
		data, err := xml.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set("Content-Type", "application/xml")
		_, err = w.Write(data)
		return err
	case YAML:
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}

		w.Header().Set("Content-Type", "text/yaml")
		_, err = w.Write(data)
		return err
	}

	return fmt.Errorf("Unknown content format: %v", t)
}

func (s *Server) ContentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := r.Header.Get("Accept")

		if strings.Contains(content, "application/json") {
			ctx := context.WithValue(r.Context(), Content, JSON)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, "application/yaml") {
			ctx := context.WithValue(r.Context(), Content, YAML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, "application/xml") {
			ctx := context.WithValue(r.Context(), Content, XML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			HttpError(w, http.StatusUnsupportedMediaType)
		}
	})
}

func HttpError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}
