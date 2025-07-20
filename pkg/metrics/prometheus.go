package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusHandler returns an HTTP handler for Prometheus metrics endpoint
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// StartPrometheusServer starts a Prometheus metrics server on the specified address
func StartPrometheusServer(addr string) error {
	http.Handle("/metrics", PrometheusHandler())
	return http.ListenAndServe(addr, nil)
}