package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "showcase_http_requests_total",
			Help: "Total number of HTTP requests handled by the application.",
		},
		[]string{"path", "method", "status_code"},
	)

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "showcase_http_request_duration_seconds",
		Help:    "Histogram of request latencies.",
		Buckets: prometheus.DefBuckets,
	},
		[]string{"path", "method"},
	)

	activeRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "showcase_active_requests",
		Help: "Number of active requests currently being processed.",
	})
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(statusCode int) {
	sr.statusCode = statusCode
	sr.ResponseWriter.WriteHeader(statusCode)
}

func NewStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		activeRequests.Inc()
		defer activeRequests.Dec()

		sr := NewStatusRecorder(w)

		next.ServeHTTP(sr, r)

		duration := time.Since(start)
		path := r.URL.Path
		method := r.Method
		statusCode := fmt.Sprintf("%d", sr.statusCode)

		httpRequestsTotal.With(prometheus.Labels{
			"path":        path,
			"method":      method,
			"status_code": statusCode,
		}).Inc()

		httpRequestDuration.With(prometheus.Labels{
			"path":   path,
			"method": method,
		}).Observe(duration.Seconds())

		log.Printf(
			"Metrics: path=%s method=%s status=%s duration=%v",
			path,
			method,
			statusCode,
			duration,
		)
	})
}
