package core

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/util"
)

// RunHTTPServer - Run HTTP server.
func RunHTTPServer() {
	log.WithFields(log.Fields{
		"endpoint": common.Config.HTTPEndpoint,
	}).Info("HTTP server listening")
	var mainServeMux http.ServeMux
	mainServeMux.HandleFunc("/", handleOtherRequest)
	mainServeMux.HandleFunc("/metrics", handleMetricsRequest)
	if err := http.ListenAndServe(common.Config.HTTPEndpoint, &mainServeMux); err != nil {
		log.WithError(err).Fatal("HTTP server failed")
	}
}

func handleOtherRequest(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/" {
		fmt.Fprintf(response, "%s version %s by %s.\n", AppName, AppVersion, AppAuthor)
		fmt.Fprintf(response, "\nPaths:\n")
		fmt.Fprintf(response, "- Metrics: /metrics\n")
	} else {
		message := fmt.Sprintf("404 - Page not found.\n")
		http.Error(response, message, 404)
	}
}

func handleMetricsRequest(response http.ResponseWriter, request *http.Request) {
	log.WithFields(log.Fields{
		"endpoint": "metrics",
		"client":   request.RemoteAddr,
		"url":      request.URL,
	}).Trace("Request")

	// Build registry with data
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())
	util.NewExporterMetric(registry, common.PrometheusNamespace, AppVersion)

	// TODO metrics

	// Delegare final handling to Prometheus
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(response, request)
}
