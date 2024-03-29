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

//go:generate esc -o doc.go -ignore /doc/.*map -pkg service ../doc ../api

package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/google/gopacket/layers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/logger"
	"gopkg.in/yaml.v3"
)

const (
	contentType = "Content-Type"
)

var (
	hostnameExp = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

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

type Server interface {
	Init(ctx context.Context, args ...interface{})
	Start(ctx context.Context) chan bool
	Done() chan bool

	Hostname() string
	Port() uint16
}

// serverImpl provides the REST service
type serverImpl struct {
	http.Server
	client   dhcp.DHCPClient
	kube     kubernetes.KubeClient
	done     chan bool
	info     *client.Version
	config   *ServerConfig
	hostname string
	port     uint16
	logger   logger.Logger

	listenAll bool
}

// NewServer creates a new Server object
func NewServer(l logger.Logger) Server {

	return &serverImpl{
		logger: l,
		done:   make(chan bool),
	}
}

func (s *serverImpl) Init(ctx context.Context, args ...interface{}) {
	var config *ServerConfig
	var version *client.Version

	if len(args) > 0 {
		tmp, ok := args[0].(*ServerConfig)
		if ok {
			config = tmp
		}
	}

	if len(args) > 1 {
		tmp, ok := args[1].(*client.Version)
		if ok {
			version = tmp
		}
	}

	s.init(ctx, config, version)
}

// NewServer creates a new Server object
func (s *serverImpl) init(ctx context.Context, config *ServerConfig, version *client.Version) {

	if config.Verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		s.logger.Debug("Enable debug log level")
	} else if config.Quiet {
		zerolog.SetGlobalLevel(zerolog.Disabled)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if config.Hostname != "" {
		s.hostname = config.Hostname
	} else {
		s.listenAll = true
		hostname, err := os.Hostname()
		if err != nil {
			s.logger.Errorf("Error get hostname: %v", err)
		}

		s.hostname = hostname
	}

	s.port = config.Port

	if config.BaseURL == "" {
		config.BaseURL = fmt.Sprintf("http://%v:%v", s.hostname, s.port)
	}

	s.config = config
	s.Addr = fmt.Sprintf("%v:%v", config.Hostname, config.Port)

	var resolver dhcp.IPResolver = s.getIPResolver(ctx, config)

	if s.client == nil {
		s.client = dhcp.NewClient(resolver, nil, config.Mode, config.DHCPTimeout, config.Retry, s.logger)
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Use(s.PrometheusMiddleware)
	router.Use(s.CounterMiddleware)

	if !config.Quiet {
		router.Use(logger.NewLogMiddleware(s.hostname, s.logger))
	}

	router.Use(s.ContentMiddleware)

	s.Handler = router

	if version != nil {
		s.info = version

		if version.GitVersion == "" {
			version.GitVersion = "dev"
			version.GitTreeState = "dirty"
		}

		s.logger.Infof("Start server\n\n%v\n", s.info)
		s.logger.Debugf("Config:\n\n%v\n", s.config)
	}

	// Manipulate modtime of swagger file to invalidate cache
	file, ok := _escData["/api/swagger.yaml"]
	if ok && version != nil {
		file.modtime = time.Now().Unix()
		data, err := decode(file.compressed)
		if err == nil {
			old := string(data)
			new := strings.Replace(old, "version: \"1.0.0\"", "version: \""+version.GitVersion+"\"", 1)

			if s.config.BaseURL != "" {
				new = strings.Replace(new, "host: \"localhost:8080\"", fmt.Sprintf("host: \"%v\"", s.config.BaseURL), 1)
			} else {
				new = strings.Replace(new, "host: \"localhost:8080\"", fmt.Sprintf("host: \"%v:%v\"", s.hostname, s.port), 1)
			}

			if old != new {
				file.size = int64(len(new))
				file.compressed = encode([]byte(new))
			}
		}
	}

	s.setup(router)
}

func (s *serverImpl) Hostname() string {
	return s.hostname
}

func (s *serverImpl) Port() uint16 {
	return s.port
}

// Start starts the server
// * @param ctx - context for a graceful shutdown
func (s *serverImpl) Start(ctx context.Context) chan bool {
	rc := make(chan bool, 1)

	go func() {
		<-s.client.Start()

		go func() {
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Fatalf("Error listen: %s", err)
			}
		}()

		go func() {
			var url string
			if s.listenAll {
				url = fmt.Sprintf("http://localhost:%v/health", s.port)
			} else {
				url = fmt.Sprintf("http://%v:%v/health", s.hostname, s.port)
			}
			for {
				time.Sleep(10 * time.Millisecond)

				_, err := http.Get(url)
				if err == nil {
					s.logger.Info("Server listen on ", s.Addr)
					close(rc)
					break
				}
			}
		}()

		<-ctx.Done()
		s.client.Stop()

		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.Shutdown(ctx2); err != nil {
			s.logger.Infof("Server Shutdown Failed:%+v", err)
		}

		s.logger.Info("Server stopped")

		close(s.done)
	}()

	return rc
}

func (s *serverImpl) Done() chan bool {
	return s.done
}

func (s *serverImpl) setup(router *mux.Router) {
	router.
		Name("/swagger.yaml").
		Methods("GET").
		Path("/api/swagger.yaml").
		Handler(http.FileServer(FS(false)))

	router.
		Name("/api").
		Methods("GET").
		Path("/api").
		HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.RequestURI, s.config.BaseURL) {
				http.Redirect(res, req, "/doc/", http.StatusFound)
			} else {
				s.logger.Infof("Redirect to %v", fmt.Sprintf("%s/doc/", s.config.BaseURL))
				http.Redirect(res, req, fmt.Sprintf("%s/doc/", s.config.BaseURL), http.StatusFound)
			}
		})

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

	router.
		Name("/doc").
		Methods("GET").
		PathPrefix("/doc/").
		Handler(http.FileServer(FS(false)))

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

	router.
		Name("/").
		Methods("GET").
		Path("/").
		Handler(RedirectHandler("/doc/", http.StatusFound))

}

func (s *serverImpl) version(w http.ResponseWriter, r *http.Request) {
	contentType := r.Context().Value(Content).(client.ContentType)

	s.write(w, s.info, contentType)
}

func (s *serverImpl) lease(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	lease := <-s.client.GetLease(ctx, query.Hostname, net.HardwareAddr(query.Mac))

	s.response(r.Context(), query, w, lease)
}

func (s *serverImpl) renew(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	lease := <-s.client.Renew(ctx, query.Hostname, net.HardwareAddr(query.Mac), query.IP)

	s.response(r.Context(), query, w, lease)
}

func (s *serverImpl) release(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	err = <-s.client.Release(ctx, query.Hostname, net.HardwareAddr(query.Mac), query.IP)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	w.Write([]byte("Ok.\n"))
}

func (s *serverImpl) response(ctx context.Context, query *Query, w http.ResponseWriter, lease *dhcp.Lease) {
	if lease == nil {
		httpError(w, http.StatusRequestTimeout)
		return
	}
	if !lease.Ok() {
		if lease.DHCP4 != nil {
			if lease.GetMsgType() == layers.DHCPMsgTypeNak {
				http.Error(w, lease.Error().Error(), http.StatusNotAcceptable)
				return
			}
		}
		http.Error(w, lease.Error().Error(), http.StatusBadRequest)
		return
	}
	result := NewLease(query.Hostname, lease)
	contentType := ctx.Value(Content).(client.ContentType)
	s.write(w, result, contentType)
}

func (s *serverImpl) getIPResolver(ctx context.Context, config *ServerConfig) dhcp.IPResolver {
	if config.KubeConfig != nil && len(config.KubeConfig.Service) > 0 {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if s.kube == nil {
			client, err := kubernetes.NewKubeClient(config.KubeConfig.Config, s.logger)
			if err == nil {
				s.kube = client
			} else {
				s.logger.Errorf("Kubernetes client configuration: %v", err)
			}
		}

		if s.kube != nil {
			resolver := dhcp.NewKubernetesExternalIPResolver(config.Local, config.Remote, config.KubeConfig, s.kube, s.logger)
			extIP, err := resolver.GetRelayIP(ctx)
			if err == nil {
				config.Relay = extIP
				return resolver
			} else {
				s.logger.Errorf("Auto detect relay IP from Kubernetes failed: %v", err)
				if config.Relay != nil {
					return dhcp.NewStaticIPResolver(config.Local, config.Relay, config.Relay, s.logger)
				} else {
					s.logger.Fatalf("No valid replay IP")
					return nil
				}
			}
		} else {
			return dhcp.NewStaticIPResolver(config.Local, config.Remote, config.Relay, s.logger)
		}
	} else {
		return dhcp.NewStaticIPResolver(config.Local, config.Remote, config.Relay, s.logger)
	}
}

func (s *serverImpl) write(w http.ResponseWriter, value interface{}, t client.ContentType) error {

	switch t {
	case client.JSON:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set(contentType, string(client.JSON))
		_, err = w.Write(data)
		return err
	case client.XML:
		data, err := xml.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		w.Header().Set(contentType, string(client.XML))
		_, err = w.Write(data)
		return err
	case client.YAML:
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}

		w.Header().Set(contentType, string(client.YAML))
		_, err = w.Write(data)
		return err
	}

	return fmt.Errorf("unknown content format: %v", t)
}

func httpError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

func decode(data string) ([]byte, error) {
	b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(data))
	gr, err := gzip.NewReader(b64)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(gr)
}

func encode(data []byte) string {
	var buf bytes.Buffer
	gr, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	gr.Write(data)
	gr.Close()

	var buf2 bytes.Buffer
	b64 := base64.NewEncoder(base64.StdEncoding, &buf2)
	b64.Write(buf.Bytes())
	b64.Close()

	return buf2.String()
}
