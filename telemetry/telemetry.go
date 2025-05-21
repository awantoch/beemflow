package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "beemflow_http_requests_total",
			Help: "Total number of HTTP requests received.",
		},
		[]string{"handler", "method", "code"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "beemflow_http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler", "method"},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// Init sets up tracing exporter based on config.
// Supported exporters: "stdout", "jaeger".
func Init(cfg *config.Config) {
	// Setup tracer provider
	serviceName := "beemflow"
	if cfg.Tracing != nil && cfg.Tracing.ServiceName != "" {
		serviceName = cfg.Tracing.ServiceName
	}
	res, _ := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	var tp *sdktrace.TracerProvider
	switch {
	case cfg.Tracing != nil && cfg.Tracing.Exporter == "jaeger":
		endpoint := cfg.Tracing.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:14268/api/traces"
		}
		exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpoint)))
		if err == nil {
			tp = sdktrace.NewTracerProvider(
				sdktrace.WithBatcher(exp),
				sdktrace.WithResource(res),
			)
		}
	default: // stdout fallback
		exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
	}
	if tp != nil {
		otel.SetTracerProvider(tp)
	}
}

// WrapHandler applies tracing, Prometheus metrics, and otelhttp middleware.
func WrapHandler(name string, next http.Handler) http.Handler {
	// Trace + context propagation
	h := otelhttp.NewHandler(next, name)
	// Metrics middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{w, 200}
		h.ServeHTTP(rw, r)
		dur := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(name, r.Method, fmt.Sprintf("%d", rw.status)).Inc()
		httpRequestDuration.WithLabelValues(name, r.Method).Observe(dur)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsHandler returns the Prometheus metrics endpoint handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
