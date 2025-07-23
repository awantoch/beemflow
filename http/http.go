package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	api "github.com/awantoch/beemflow/core"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
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
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

// StartServer starts the HTTP server with minimal setup - all the heavy lifting
// is now done by the unified operations system
func StartServer(cfg *config.Config) error {
	// Initialize tracing
	initTracerFromConfig(cfg)

	// Create HTTP mux
	mux := http.NewServeMux()

	// Serve static files from ./static/ directory under /static/*
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Register system endpoints (health, spec) that don't follow the operation pattern
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"healthy"}`)); err != nil {
			utils.Error("Failed to write health check response: %v", err)
		}
	})

	// Register Slack events route (conversational BeemBeem)
	mux.HandleFunc("/slack/events", SlackEventsHandler())

	// Generate and register all operation handlers
	api.GenerateHTTPHandlers(mux)

	// Register metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Initialize all dependencies (this could be moved to a separate DI package)
	cleanup, err := api.InitializeDependencies(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// Determine server address
	addr := getServerAddress(cfg)

	// Create wrapped handler with middleware
	wrappedMux := otelhttp.NewHandler(
		requestIDMiddleware(
			metricsMiddleware("root", mux),
		),
		"http.root",
	)

	// Start server with graceful shutdown
	return startServerWithGracefulShutdown(addr, wrappedMux)
}

// getServerAddress determines the server address from config
func getServerAddress(cfg *config.Config) string {
	addr := ":3333" // default
	if cfg.HTTP != nil && cfg.HTTP.Port != 0 {
		host := cfg.HTTP.Host
		if host == "" {
			host = "0.0.0.0"
		}
		addr = fmt.Sprintf("%s:%d", host, cfg.HTTP.Port)
	}
	return addr
}

// startServerWithGracefulShutdown starts the HTTP server and handles graceful shutdown
func startServerWithGracefulShutdown(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Channel to listen for errors from ListenAndServe
	errChan := make(chan error, 1)
	go func() {
		utils.Info("HTTP server starting on %s", addr)
		errChan <- server.ListenAndServe()
	}()

	// Listen for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		utils.Info("Received signal %v, shutting down HTTP server...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			utils.Error("HTTP server shutdown error: %v", err)
			return err
		}
		utils.Info("HTTP server shutdown complete.")
		return nil
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.Error("HTTP server error: %v", err)
			return err
		}
		return nil
	}
}

// initTracerFromConfig sets up OpenTelemetry tracing based on config.
func initTracerFromConfig(cfg *config.Config) {
	var tp *trace.TracerProvider
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
	switch {
	case cfg.Tracing == nil || cfg.Tracing.Exporter == "stdout":
		exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
		tp = trace.NewTracerProvider(
			trace.WithBatcher(exp),
			trace.WithResource(res),
		)
	case cfg.Tracing.Exporter == "otlp":
		endpoint := cfg.Tracing.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:4318"
		}
		exp, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpoint(endpoint), otlptracehttp.WithInsecure())
		if err == nil {
			tp = trace.NewTracerProvider(
				trace.WithBatcher(exp),
				trace.WithResource(res),
			)
		}
	default:
		exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
		tp = trace.NewTracerProvider(
			trace.WithBatcher(exp),
			trace.WithResource(res),
		)
	}
	if tp != nil {
		otel.SetTracerProvider(tp)
	}
}

// requestIDMiddleware generates a request ID for each request and stores it in the context.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx := utils.WithRequestID(r.Context(), reqID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(w, r)
	})
}

// metricsMiddleware instruments HTTP handlers for Prometheus.
func metricsMiddleware(handlerName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(handlerName, r.Method, fmt.Sprintf("%d", rw.status)).Inc()
		httpRequestDuration.WithLabelValues(handlerName, r.Method).Observe(duration)
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

// ============================================================================
// TEST UTILITIES (consolidated from test_utils.go)
// ============================================================================

// UpdateRunEvent updates the event for a run.
// Used for tests and directly accesses the storage layer.
func UpdateRunEvent(id uuid.UUID, newEvent map[string]any) error {
	// Get storage from config
	cfg, err := config.LoadConfig(constants.ConfigFileName)
	if err != nil {
		return utils.Errorf("failed to load config: %v", err)
	}

	// Get the configured storage using the helper function from api package
	store, err := api.GetStoreFromConfig(cfg)
	if err != nil {
		return err
	}

	// Get the run
	run, err := store.GetRun(context.Background(), id)
	if err != nil {
		return utils.Errorf("run not found")
	}

	// Update the event
	run.Event = newEvent

	// Save the updated run
	return store.SaveRun(context.Background(), run)
}
