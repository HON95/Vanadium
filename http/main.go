package http

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/db"
	"dev.hon.one/vanadium/util"
)

// StartServer - Start HTTP server in the background.
func StartServer(waitGroup *sync.WaitGroup, shutdown *util.ShutdownChannelDistributor) {
	shutdownChannel := make(chan bool, 1)
	if !shutdown.AddListener(shutdownChannel) {
		return
	}
	waitGroup.Add(1)

	// Configure
	var mainServeMux http.ServeMux
	mainServeMux.HandleFunc("/", handleOtherRequest)
	mainServeMux.HandleFunc("/metrics", handleMetricsRequest)
	server := &http.Server{
		Addr:    common.GlobalConfig.HTTPEndpoint,
		Handler: &mainServeMux,
	}

	// Run
	var shutdownContextCancel context.CancelFunc = nil
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("HTTP server failed")
		}
		// Cancel shutdown timer
		if shutdownContextCancel != nil {
			shutdownContextCancel()
		}
		log.Info("HTTP server stopped")
		waitGroup.Done()
	}()

	// Shutdown
	go func() {
		select {
		case <-shutdownChannel:
			var shutdownContext context.Context
			shutdownContext, shutdownContextCancel = context.WithTimeout(context.Background(), 5*time.Second)
			server.Shutdown(shutdownContext)
		}
	}()

	log.Infof("HTTP server started: %v", common.GlobalConfig.HTTPEndpoint)
}

func handleOtherRequest(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/" {
		fmt.Fprintf(response, "%s version %s by %s.\n", common.AppName, common.AppVersion, common.AppAuthor)
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
	util.NewExporterMetric(registry, common.PrometheusNamespace, common.AppVersion)

	// TODO metrics
	db.FetchRecentScrapeEntries()

	// Delegare final handling to Prometheus
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(response, request)
}
