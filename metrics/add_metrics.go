package metrics

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
)

func makeClient() http.Client {
	// Fine-tune the client to fail fast.
	return http.Client{}
}

// AddMetricsHandler wraps a http.HandlerFunc with Prometheus metrics
func AddMetricsHandler(handler http.HandlerFunc, prometheusQuery PrometheusQueryFetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// log.Printf("Calling upstream for function info\n")

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		upstreamCall := recorder.Result()

		if upstreamCall.Body == nil {
			log.Println("Upstream call had empty body.")
			return
		}

		defer upstreamCall.Body.Close()

		if recorder.Code != http.StatusOK {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("Error pulling metrics from provider/backend. Status code: %d", recorder.Code)))
			return
		}
	}
}
