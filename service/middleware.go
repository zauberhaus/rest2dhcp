package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zauberhaus/rest2dhcp/client"
)

// ContentMiddleware returns a gorilla mux middleware to check the Accept HTTP header and set a context value
func (s *serverImpl) ContentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/doc") {
			next.ServeHTTP(w, r)
			return
		}

		content := r.Header.Get("Accept")

		if strings.Contains(content, string(client.JSON)) {
			ctx := context.WithValue(r.Context(), Content, client.JSON)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, string(client.YAML)) {
			ctx := context.WithValue(r.Context(), Content, client.YAML)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else if strings.Contains(content, string(client.XML)) {
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
func (s *serverImpl) PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(route.GetName()))
		next.ServeHTTP(w, r)
		timer.ObserveDuration()

	})
}

// CounterMiddleware returns a gorilla mux middleware to count requests as prometheus metrics
func (s *serverImpl) CounterMiddleware(next http.Handler) http.Handler {
	return promhttp.InstrumentHandlerCounter(httpCounter, next)
}
