// Copyright (c) OpenFaaS Project 2018. All rights reserved.
// Copyright (c) Eugen Biegler 2018. All rights reserved.

package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/negbie/graferse/metrics"
	"github.com/negbie/graferse/types"
	"github.com/prometheus/client_golang/prometheus"
)

// HTTPNotifier notify about HTTP request/response
type HTTPNotifier interface {
	Notify(method string, URL string, statusCode int, duration time.Duration)
}

// BaseURLResolver URL resolver for upstream requests
type BaseURLResolver interface {
	Resolve(r *http.Request) (URL, username string)
}

// MakeForwardingProxyHandler create a handler which forwards HTTP requests
func MakeForwardingProxyHandler(proxy *types.HTTPClientReverseProxy, notifiers []HTTPNotifier, baseURLResolver BaseURLResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL, user := baseURLResolver.Resolve(r)

		requestURL := r.URL.Path

		start := time.Now()

		statusCode, err := forwardRequest(w, r, proxy.Client, baseURL, user, requestURL, proxy.Timeout)

		seconds := time.Since(start)
		if err != nil {
			log.Printf("error with upstream request to: %s, %s\n", requestURL, err.Error())
		}
		for _, notifier := range notifiers {
			notifier.Notify(r.Method, requestURL, statusCode, seconds)
		}
	}
}

func buildUpstreamRequest(r *http.Request, url, user string) *http.Request {

	if len(r.URL.RawQuery) > 0 {
		url = fmt.Sprintf("%s?%s", url, r.URL.RawQuery)
	}

	upstreamReq, _ := http.NewRequest(r.Method, url, nil)
	copyHeaders(upstreamReq.Header, &r.Header)

	upstreamReq.Header["X-Forwarded-For"] = []string{r.RemoteAddr}
	upstreamReq.Header["X-WEBAUTH-USER"] = []string{user}

	if r.Body != nil {
		upstreamReq.Body = r.Body
	}

	return upstreamReq
}

func forwardRequest(w http.ResponseWriter, r *http.Request, proxyClient *http.Client, baseURL, user, requestURL string, timeout time.Duration) (int, error) {

	upstreamReq := buildUpstreamRequest(r, baseURL+requestURL, user)
	if upstreamReq.Body != nil {
		defer upstreamReq.Body.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, resErr := proxyClient.Do(upstreamReq.WithContext(ctx))
	if resErr != nil {
		badStatus := http.StatusBadGateway
		w.WriteHeader(badStatus)
		return badStatus, resErr
	}

	if res.Body != nil {
		resBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return 500, err
		}
		err = res.Body.Close()
		if err != nil {
			return 500, err
		}
		resBody = bytes.Replace(resBody, []byte("$your_template_variable"), []byte("goes_in_here"), -1)
		body := ioutil.NopCloser(bytes.NewReader(resBody))
		res.Body = body
		res.ContentLength = int64(len(resBody))
		res.Header.Set("Content-Length", strconv.Itoa(len(resBody)))

		defer res.Body.Close()
	}

	copyHeaders(w.Header(), &res.Header)

	// Write status code
	w.WriteHeader(res.StatusCode)

	if res.Body != nil {
		// Copy the body over
		io.CopyBuffer(w, res.Body, nil)
	}

	return res.StatusCode, nil
}

func copyHeaders(destination http.Header, source *http.Header) {
	for k, v := range *source {
		vClone := make([]string, len(v))
		copy(vClone, v)
		(destination)[k] = vClone
	}
}

// PrometheusFunctionNotifier records metrics to Prometheus
type PrometheusFunctionNotifier struct {
	Metrics *metrics.MetricOptions
}

// Notify records metrics in Prometheus
func (p PrometheusFunctionNotifier) Notify(method string, URL string, statusCode int, duration time.Duration) {
	seconds := duration.Seconds()
	serviceName := getServiceName(URL)

	p.Metrics.GatewayFunctionsHistogram.
		WithLabelValues(serviceName).
		Observe(seconds)

	code := strconv.Itoa(statusCode)

	p.Metrics.GatewayFunctionInvocation.
		With(prometheus.Labels{"function_name": serviceName, "code": code}).
		Inc()
}

func getServiceName(urlValue string) string {
	var serviceName string
	forward := "/function/"
	if strings.HasPrefix(urlValue, forward) {
		serviceName = urlValue[len(forward):]
	}
	return strings.Trim(serviceName, "/")
}

// LoggingNotifier notifies a log about a request
type LoggingNotifier struct {
}

// Notify a log about a request
func (LoggingNotifier) Notify(method string, URL string, statusCode int, duration time.Duration) {
	log.Printf("Forwarded [%s] to %s - [%d] - %f seconds", method, URL, statusCode, duration.Seconds())
}

// SingleHostBaseURLResolver resolves URLs against a single BaseURL
type SingleHostBaseURLResolver struct {
	BaseURL  string
	Username string
}

// Resolve the base URL for a request
func (s SingleHostBaseURLResolver) Resolve(r *http.Request) (URL, username string) {

	baseURL := s.BaseURL

	if strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL[0 : len(baseURL)-1]
	}
	return baseURL, s.Username
}

// FunctionAsHostBaseURLResolver resolves URLs using a function from the URL as a host
type FunctionAsHostBaseURLResolver struct {
	FunctionSuffix string
	Username       string
}

// Resolve the base URL for a request
func (f FunctionAsHostBaseURLResolver) Resolve(r *http.Request) (URL, username string) {
	svcName := getServiceName(r.URL.Path)

	const watchdogPort = 8080
	var suffix string
	if len(f.FunctionSuffix) > 0 {
		suffix = "." + f.FunctionSuffix
	}

	return fmt.Sprintf("http://%s%s:%d", svcName, suffix, watchdogPort), f.Username
}
