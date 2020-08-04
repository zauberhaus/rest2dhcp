package main

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
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	cl "github.com/zauberhaus/rest2dhcp/client"
	"gopkg.in/yaml.v3"
)

var (
	client *cl.Client
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

type Key byte

const (
	Content Key = iota
)

type ContentType byte

const (
	Unknown ContentType = iota
	JSON
	YAML
	XML
)

type Query struct {
	Hostname string `json:"hostname" xml:"hostname"`
	Mac      MAC    `json:"mac" xml:"mac"`
	IP       net.IP `json:"ip" xml:"ip"`
}

func NewQuery(r *http.Request) (*Query, error) {
	query := Query{}

	vars := mux.Vars(r)
	hostname, ok := vars["hostname"]
	if ok {
		if hostnameExp.MatchString(hostname) {
			query.Hostname = hostname
		} else {
			return nil, fmt.Errorf("Invalid hostname")
		}
	}

	mac, ok := vars["mac"]
	if ok {
		tmp, err := net.ParseMAC(mac)
		if err == nil {
			query.Mac = MAC{tmp}
		}
	}

	ip, ok := vars["ip"]
	if ok {
		tmp := net.ParseIP(ip)
		if tmp == nil {
			return nil, fmt.Errorf("Invalid IP format")
		}

		query.IP = tmp
	}

	return &query, nil
}

type Lease struct {
	XMLName  xml.Name  `xml:"lease" json:"-" yaml:"-"`
	Hostname string    `json:"hostname" xml:"hostname"`
	Mac      MAC       `json:"mac" xml:"mac"`
	Cid      string    `json:"client-id,omitempty" xml:"client-id,omitempty" yaml:"client-id,omitempty"`
	IP       string    `json:"ip" xml:"ip"`
	Renew    time.Time `json:"renew" xml:"renew"`
	Expire   time.Time `json:"expire" xml:"expire"`
}

type Result struct {
	Lease `json:"lease" xml:"lease"`
}

type MAC struct {
	net.HardwareAddr
}

func (m MAC) Unmarshal(value *yaml.Node) error {
	mac, err := net.ParseMAC(value.Value)
	if err == nil {
		m.HardwareAddr = mac
	}

	return err
}

func (m MAC) UnmarshalJSON(b []byte) error {
	var txt string
	json.Unmarshal(b, &txt)

	mac, err := net.ParseMAC(txt)
	if err == nil {
		m.HardwareAddr = mac
	}

	return err
}

func (m MAC) MarshalYAML() (interface{}, error) {
	return m.String(), nil
}

func (m MAC) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m MAC) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(m.String(), start)
}

func setupRouter(router *mux.Router) {
	for _, p := range renewPath {
		router.
			Methods("GET").
			Path(p).
			HandlerFunc(renew)
	}

	for _, p := range releasePath {
		router.
			Methods("DELETE").
			Path(p).
			HandlerFunc(release)
	}

	for _, p := range leasePath {
		router.
			Methods("GET").
			Path(p).
			HandlerFunc(get)
	}
}

func get(w http.ResponseWriter, r *http.Request) {

	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lease := <-client.GetLease(ctx, query.Hostname, query.Mac.HardwareAddr)

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
	write(w, result, contentType)
}

func renew(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lease := <-client.ReNew(ctx, query.Hostname, query.Mac.HardwareAddr, query.IP)

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
	write(w, result, contentType)
}

func release(w http.ResponseWriter, r *http.Request) {
	query, err := NewQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lease := <-client.Release(ctx, query.Hostname, query.Mac.HardwareAddr, query.IP)

	if lease != nil {
		HttpError(w, http.StatusRequestTimeout)
		return
	}

	w.Write([]byte("Ok."))
}

func write(w http.ResponseWriter, value interface{}, t ContentType) error {
	w.Header().Set("Content-Type", "application/yaml")

	switch t {
	case JSON:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		_, err = w.Write(data)
		return err
	case XML:
		data, err := xml.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, byte('\n'))

		_, err = w.Write(data)
		return err
	case YAML:
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}

		_, err = w.Write(data)
		return err
	}

	return fmt.Errorf("Unknown content format: %v", t)
}

func ContentMiddleware(next http.Handler) http.Handler {
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

func main() {

	client = cl.NewClient(nil, nil, cl.AutoDetect)
	client.Start()

	router := mux.NewRouter().StrictSlash(true)
	router.Use(ContentMiddleware)

	setupRouter(router)

	log.Fatal(http.ListenAndServe(":8080", handlers.CombinedLoggingHandler(os.Stdout, router)))
}
