/*
Copyright © 2020 Dirk Lembke <dirk@lembke.nz>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"regexp"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"gopkg.in/yaml.v3"
)

var (
	accept = regexp.MustCompile(`application/[json|yaml]`)

	hostnameExp = regexp.MustCompile("^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$")

	renewPath = map[string]string{
		"renew": "/ip/{hostname}/{mac}/{ip}",
	}

	releasePath = map[string]string{
		"release": "/ip/{hostname}/{mac}/{ip}",
	}

	leasePath = map[string]string{
		"get":        "/ip/{hostname}",
		"getWithMac": "/ip/{hostname}/{mac}",
	}

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "rest2dhcp_http_duration_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"action"})

	httpCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rest2dhcp_http_counter",
		Help: "Counter for HTTP requests",
	}, []string{"code"})

	// Version contains the build and version info
	Version *client.Version
)

// Server provides the REST service
type Server struct {
	http.Server
	client *dhcp.Client
	Done   chan bool
	Info   *client.Version
	Config *ServerConfig
}

// NewServer creates a new Server object
func NewServer(config *ServerConfig, version *client.Version) *Server {
	server := Server{}
	server.Config = config
	server.Addr = config.Listen
	server.Done = make(chan bool)
	//server.timeout = config.Timeout

	server.client = dhcp.NewClient(config.Local, config.Remote, config.Relay, config.Mode, config.DHCPTimeout, config.Retry)

	router := mux.NewRouter().StrictSlash(true)
	router.Use(server.ContentMiddleware)
	router.Use(server.CounterMiddleware)
	router.Use(server.PrometheusMiddleware)
	server.setup(router)

	server.Handler = handlers.CombinedLoggingHandler(os.Stderr, router)

	if version != nil {
		version.DHCPServer = server.client.GetDHCPServerIP()
		version.RelayIP = server.client.GetDHCPRelayIP()
		version.Mode = server.client.GetDHCPRelayMode()
		server.Info = version
		log.Printf("Version:\n%v\n", server.Info)
		log.Printf("Config:\n%v\n", server.Config)
	}

	return &server
}

// Start starts the server
// * @param ctx - context for a graceful shutdown
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
			HandlerFunc(s.lease)
	}

}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	contentType := client.ContentType(r.Context().Value(Content).(string))

	s.write(w, s.Info, contentType)
}

func (s *Server) lease(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Config.Timeout)
	defer cancel()

	lease := <-s.client.GetLease(ctx, query.Hostname, net.HardwareAddr(query.Mac))

	if lease == nil {
		httpError(w, http.StatusRequestTimeout)
		return
	}

	if !lease.Ok() {
		http.Error(w, lease.Error().Error(), http.StatusBadRequest)
		return
	}

	result := client.NewLease(query.Hostname, *lease.DHCP4)
	contentType := client.ContentType(r.Context().Value(Content).(string))

	s.write(w, result, contentType)
}

func (s *Server) renew(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Config.Timeout)
	defer cancel()

	lease := <-s.client.ReNew(ctx, query.Hostname, net.HardwareAddr(query.Mac), query.IP)

	if lease == nil {
		httpError(w, http.StatusRequestTimeout)
		return
	}

	if !lease.Ok() {
		http.Error(w, lease.Error().Error(), http.StatusBadRequest)
		return
	}

	result := client.NewLease(query.Hostname, *lease.DHCP4)
	contentType := client.ContentType(r.Context().Value(Content).(string))
	s.write(w, result, contentType)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Config.Timeout)
	defer cancel()

	lease := <-s.client.Release(ctx, query.Hostname, net.HardwareAddr(query.Mac), query.IP)

	if lease != nil {
		httpError(w, http.StatusRequestTimeout)
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

// ContentMiddleware returns a gorilla mux middleware to check the Accept HTTP header and set a context value
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
			httpError(w, http.StatusUnsupportedMediaType)
		}
	})
}

// PrometheusMiddleware returns a gorilla mux middleware to measure the execution time as prometheus metrics
func (s *Server) PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(route.GetName()))
		next.ServeHTTP(w, r)
		timer.ObserveDuration()
	})
}

// CounterMiddleware returns a gorilla mux middleware to count requests as prometheus metrics
func (s *Server) CounterMiddleware(next http.Handler) http.Handler {
	return promhttp.InstrumentHandlerCounter(httpCounter, next)
}

func httpError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}
