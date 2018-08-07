package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"

	"github.com/negbie/graferse/handlers"
	"github.com/negbie/graferse/metrics"
	"github.com/negbie/graferse/types"
)

var (
	proxyAddr   = flag.String("proxy_addr", ":8080", "Reverse proxy listen address")
	grafanaURL  = flag.String("grafana_url", "http://localhost:3000", "Grafana URL")
	grafanaUser = flag.String("grafana_user", "admin", "Grafana Authproxy Username")
	withMetric  = flag.Bool("metric", false, "Expose prometheus metrics")
	timeout     = flag.Duration("timeout", 1, "HTTP read, write timeout in minutes")
	readOnly    = flag.Bool("readonly", true, "Don't allow changes inside Grafana")
	cert        = flag.String("cert", "", "SSL certificate path")
	key         = flag.String("key", "", "SSL private Key path")
)

func main() {
	flag.Parse()
	var httpTimeout time.Duration

	url, err := url.Parse(*grafanaURL)
	if err != nil {
		log.Fatal("Please provide a valid Grafana URL like http://localhost:3000", err)
	}

	if *timeout > 0 {
		httpTimeout = *timeout * time.Minute
	} else {
		httpTimeout = 1 * time.Minute
	}

	var graferseHandler http.HandlerFunc
	var forwardingNotifiers []handlers.HTTPNotifier

	loggingNotifier := handlers.LoggingNotifier{}
	reverseProxy := types.NewHTTPClientReverseProxy(url, httpTimeout)

	if *withMetric {
		metricsOptions := metrics.BuildMetricsOptions()
		metrics.RegisterMetrics(metricsOptions)
		prometheusNotifier := handlers.PrometheusFunctionNotifier{
			Metrics: &metricsOptions,
		}
		forwardingNotifiers = []handlers.HTTPNotifier{loggingNotifier, prometheusNotifier}
	} else {
		forwardingNotifiers = []handlers.HTTPNotifier{loggingNotifier}
	}

	urlResolver := handlers.SingleHostBaseURLResolver{BaseURL: *grafanaURL, Username: *grafanaUser}
	var basicURLResolver handlers.BaseURLResolver

	//TODO flag!
	if true {
		basicURLResolver = urlResolver
	} else {
		basicURLResolver = handlers.FunctionAsHostBaseURLResolver{FunctionSuffix: "example.com"}
	}

	graferseHandler = handlers.MakeForwardingProxyHandler(reverseProxy, forwardingNotifiers, basicURLResolver)

	r := mux.NewRouter()
	r.Handle("/logout", http.RedirectHandler("/", http.StatusMovedPermanently)).Methods(http.MethodGet)
	if *readOnly {
		r.Methods("GET").PathPrefix("/").HandlerFunc(graferseHandler)
	} else {
		r.PathPrefix("/").HandlerFunc(graferseHandler)
	}
	r.Methods("POST").PathPrefix("/api/tsdb/").HandlerFunc(graferseHandler)

	if *withMetric {
		metricsHandler := metrics.PrometheusHandler()
		r.Methods("GET").PathPrefix("/metrics").Handler(metricsHandler)
	}

	s := &http.Server{
		Addr:           *proxyAddr,
		ReadTimeout:    httpTimeout,
		WriteTimeout:   httpTimeout,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes, // 1MB - can be overridden by setting Server.MaxHeaderBytes.
		Handler:        r,
	}

	if *cert != "" && *key != "" {
		log.Fatal(s.ListenAndServeTLS(*cert, *key))
	} else {
		log.Fatal(s.ListenAndServe())
	}
}
