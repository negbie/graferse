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
	timeout     = flag.Int("timeout", 16, "HTTP read, write timeout in seconds")
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
		httpTimeout = time.Duration(*timeout) * time.Second
	} else {
		httpTimeout = 16 * time.Second
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

	if *readOnly {
		r.Methods("GET").Path("/").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/dashboards").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/api/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/api/dashboards/").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/api/dashboards/tags").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/api/dashboards/home").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/api/dashboard/snapshots").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/api/dashboards/uid/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/avatar/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/d/{id}/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/public/build/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/public/fonts/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/public/fonts/{name}/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/public/img/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/public/img/{name}/{.*}").HandlerFunc(graferseHandler)
		r.Methods("GET").Path("/logout").HandlerFunc(redirect)
	} else {
		r.PathPrefix("/").HandlerFunc(graferseHandler)
	}
	r.Methods("POST").PathPrefix("/api/tsdb/").HandlerFunc(graferseHandler)

	if *withMetric {
		metricsHandler := metrics.PrometheusHandler()
		r.Methods("GET").Path("/metrics").Handler(metricsHandler)
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

func redirect(w http.ResponseWriter, req *http.Request) {
	//target := "https://" + req.Host + req.URL.Path
	target := "https://www.google.com"
	if len(req.URL.RawQuery) > 0 {
		target += "?" + req.URL.RawQuery
	}
	log.Printf("redirect to: %s", target)
	http.Redirect(w, req, target, http.StatusTemporaryRedirect)
}
