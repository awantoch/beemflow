package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/dsl"
	beemengine "github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/registry"
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

// initTracerFromConfig sets up OpenTelemetry tracing based on config.
// Tracing config example:
//
//	"tracing": {
//	  "exporter": "otlp", // or "stdout"
//	  "endpoint": "http://localhost:4318", // OTLP endpoint
//	  "serviceName": "beemflow"
//	}
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

func StartServer(cfg *config.Config) error {
	initTracerFromConfig(cfg)

	// Create the HTTP mux
	mux := http.NewServeMux()

	// Serve static files
	registry.RegisterRoute(mux, "GET", "/", constants.InterfaceDescStaticAssets, http.FileServer(http.Dir(".")).ServeHTTP)

	// Create the service
	svc := api.NewFlowService()

	// Attach all API handlers
	api.AttachHTTPHandlers(mux, svc)

	// Register metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Dependency injection: construct all dependencies for the engine
	var err error
	store, err := api.GetStoreFromConfig(cfg)
	if err != nil {
		return err
	}

	adapters := beemengine.NewDefaultAdapterRegistry(context.Background())
	templ := dsl.NewTemplater()
	var bus event.EventBus
	if cfg.Event != nil {
		bus, err = event.NewEventBusFromConfig(cfg.Event)
		if err != nil {
			utils.WarnCtx(context.Background(), "Failed to create event bus: %v, using in-memory fallback", "error", err)
			bus = event.NewInProcEventBus()
		}
	} else {
		bus = event.NewInProcEventBus()
	}
	var blobStore blob.BlobStore
	blobConfig := (*blob.BlobConfig)(nil)
	if cfg.Blob != nil {
		// Convert config.BlobConfig to blob.BlobConfig if types differ
		blobConfig = &blob.BlobConfig{
			Driver: cfg.Blob.Driver,
			Bucket: cfg.Blob.Bucket,
		}
	}
	blobStore, err = blob.NewDefaultBlobStore(context.Background(), blobConfig)
	if err != nil {
		utils.WarnCtx(context.Background(), "Failed to create blob store: %v, using nil fallback", "error", err)
		blobStore = nil
	}

	// Create engine and store it for proper cleanup
	engine := beemengine.NewEngine(adapters, templ, bus, blobStore, store)

	// Cleanup function to properly close all resources
	cleanup := func() {
		if err := engine.Close(); err != nil {
			utils.Error("Failed to close engine: %v", err)
		}
		if store != nil {
			if closer, ok := store.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					utils.Error("Failed to close storage: %v", err)
				}
			}
		}
		if blobStore != nil {
			if closer, ok := blobStore.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					utils.Error("Failed to close blob store: %v", err)
				}
			}
		}
	}

	addr := ":3333"
	if cfg.HTTP != nil {
		host := cfg.HTTP.Host
		port := cfg.HTTP.Port
		if port != 0 {
			if host == "" {
				host = "0.0.0.0"
			}
			addr = fmt.Sprintf("%s:%d", host, port)
		}
	}
	wrappedMux := otelhttp.NewHandler(requestIDMiddleware(metricsMiddleware("root", mux)), "http.root")

	server := &http.Server{
		Addr:              addr,
		Handler:           wrappedMux,
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
			cleanup()
			return err
		}
		cleanup()
		utils.Info("HTTP server shutdown complete.")
		return nil
	case err := <-errChan:
		cleanup()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.Error("HTTP server error: %v", err)
			return err
		}
		return nil
	}
}

// UpdateRunEvent updates the event for a run.
// Used for tests and directly accesses the storage layer.
func UpdateRunEvent(id uuid.UUID, newEvent map[string]any) error {
	// Get storage from config
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
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
