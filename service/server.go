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
	"github.com/zauberhaus/rest2dhcp/background"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/logger"
	"gopkg.in/yaml.v3"

	kube "k8s.io/client-go/kubernetes"
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

// RestServer provides the REST service
type RestServer struct {
	http.Server
	client   dhcp.DHCPClient
	done     chan bool
	info     *client.Version
	config   *ServerConfig
	hostname string
	port     uint16
	logger   logger.Logger

	clientset kube.Interface
}

// NewServer creates a new Server object
func NewServer(l logger.Logger) background.Server {

	return &RestServer{
		logger: l,
		done:   make(chan bool),
	}
}

func (s *RestServer) Init(ctx context.Context, args ...interface{}) {
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
func (s *RestServer) init(ctx context.Context, config *ServerConfig, version *client.Version) {

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
		hostname, err := os.Hostname()
		if err != nil {
			s.logger.Errorf("Error get hostname: %v", err)
		}

		s.hostname = hostname
	}

	s.port = config.Port
	s.config = config
	s.Addr = fmt.Sprintf("%v:%v", config.Hostname, config.Port)

	var resolver dhcp.IPResolver = nil

	if config.KubeConfig != nil && len(config.KubeConfig.Service) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := s.getKubeClient(config.KubeConfig.Config, s.logger)
		if err == nil {
			resolver = dhcp.NewKubernetesExternalIPResolver(config.Local, config.Remote, config.KubeConfig, client, s.logger)
			extIP, err := resolver.GetRelayIP(ctx)
			if err == nil {
				config.Relay = extIP
			} else {
				s.logger.Errorf("Get relay ip: %v", err)
				if config.Relay != nil {
					resolver = dhcp.NewStaticIPResolver(config.Local, config.Relay, config.Relay, s.logger)
				} else {
					resolver = nil
				}
			}
		} else {
			s.logger.Errorf("Kubernetes client configuration: %v", err)
			resolver = dhcp.NewStaticIPResolver(config.Local, config.Remote, config.Relay, s.logger)
		}
	} else {
		resolver = dhcp.NewStaticIPResolver(config.Local, config.Remote, config.Relay, s.logger)
	}

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
			new2 := strings.Replace(new, "host: \"localhost:8080\"", fmt.Sprintf("host: \"%v:%v\"", s.hostname, s.port), 1)
			if old != new2 {
				file.size = int64(len(new2))
				file.compressed = encode([]byte(new2))
			}
		}
	}

	s.setup(router)
}

func (s *RestServer) Hostname() string {
	return s.hostname
}

func (s *RestServer) Port() uint16 {
	return s.port
}

// Start starts the server
// * @param ctx - context for a graceful shutdown
func (s *RestServer) Start(ctx context.Context) chan bool {
	rc := make(chan bool, 1)

	go func() {
		<-s.client.Start()

		go func() {
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Fatalf("Error listen: %s\n", err)
			}
		}()

		s.logger.Info("Server listen on ", s.Addr)
		close(rc)
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

func (s *RestServer) Done() chan bool {
	return s.done
}

func (s *RestServer) setup(router *mux.Router) {
	if s.hostname != "localhost" {
		router.
			Host("localhost").
			Methods("GET").
			PathPrefix("/doc/").
			HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				http.Redirect(res, req, fmt.Sprintf("http://%s:%v/%s", s.hostname, s.port, req.URL), http.StatusFound)
			})

		router.
			Name("/api").
			Methods("GET").
			PathPrefix("/api").
			HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				if s.port != 80 {
					http.Redirect(res, req, fmt.Sprintf("http://%s:%v/doc/", s.hostname, s.port), http.StatusFound)
				} else {
					http.Redirect(res, req, fmt.Sprintf("http://%s:%v/doc/", s.hostname, s.port), http.StatusFound)
				}
			})
	} else {
		router.
			Name("/api").
			Methods("GET").
			PathPrefix("/api").
			Handler(RedirectHandler("/doc/", http.StatusTemporaryRedirect))
	}

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
		Name("/swagger.yaml").
		Methods("GET").
		Path("/api/swagger.yaml").
		Handler(http.FileServer(FS(false)))

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
}

func (s *RestServer) version(w http.ResponseWriter, r *http.Request) {
	contentType := client.ContentType(r.Context().Value(Content).(string))

	s.write(w, s.info, contentType)
}

func (s *RestServer) lease(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	lease := <-s.client.GetLease(ctx, query.Hostname, net.HardwareAddr(query.Mac))

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

	result := NewLease(query.Hostname, *lease)
	contentType := client.ContentType(r.Context().Value(Content).(string))

	s.write(w, result, contentType)
}

func (s *RestServer) renew(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	lease := <-s.client.Renew(ctx, query.Hostname, net.HardwareAddr(query.Mac), query.IP)

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

	result := NewLease(query.Hostname, *lease)
	contentType := client.ContentType(r.Context().Value(Content).(string))
	s.write(w, result, contentType)
}

func (s *RestServer) release(w http.ResponseWriter, r *http.Request) {
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

func (s *RestServer) write(w http.ResponseWriter, value interface{}, t client.ContentType) error {

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

func (s *RestServer) getKubeClient(kubeconfig string, logger logger.Logger) (kubernetes.KubeClient, error) {
	if s.clientset != nil {
		return kubernetes.NewTestKubeClient(s.clientset, s.logger)
	} else {
		return kubernetes.NewKubeClient(kubeconfig, s.logger)
	}
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
