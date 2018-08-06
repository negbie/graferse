package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"

	"github.com/negbie/graferse/handlers"
	"github.com/negbie/graferse/metrics"
	"github.com/negbie/graferse/types"
)

func main() {
	domain := "https://github.com"
	url, err := url.Parse(domain)
	if err != nil {
		log.Fatal("If functions_provider_url is provided, then it should be a valid URL.", err)
	}

	timeout := 1 * time.Minute
	metricsOptions := metrics.BuildMetricsOptions()
	metrics.RegisterMetrics(metricsOptions)

	var graferseHandler http.HandlerFunc

	reverseProxy := types.NewHTTPClientReverseProxy(url, timeout)

	loggingNotifier := handlers.LoggingNotifier{}
	prometheusNotifier := handlers.PrometheusFunctionNotifier{
		Metrics: &metricsOptions,
	}
	forwardingNotifiers := []handlers.HTTPNotifier{loggingNotifier, prometheusNotifier}

	urlResolver := handlers.SingleHostBaseURLResolver{BaseURL: domain}
	var functionURLResolver handlers.BaseURLResolver

	//TODO flag!
	if false {
		functionURLResolver = handlers.FunctionAsHostBaseURLResolver{FunctionSuffix: "example.com"}
	} else {
		functionURLResolver = urlResolver
	}

	graferseHandler = handlers.MakeForwardingProxyHandler(reverseProxy, forwardingNotifiers, functionURLResolver)

	r := mux.NewRouter()

	r.HandleFunc("/test", graferseHandler)

	metricsHandler := metrics.PrometheusHandler()
	r.Handle("/metrics", metricsHandler)

	tcpPort := 8080

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", tcpPort),
		ReadTimeout:    1 * time.Minute,
		WriteTimeout:   1 * time.Minute,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes, // 1MB - can be overridden by setting Server.MaxHeaderBytes.
		Handler:        r,
	}
	//TODO TLS!
	log.Fatal(s.ListenAndServe())
}
