package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/docs"
	beemengine "github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/templater"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	runsMu            sync.Mutex
	runs              = make(map[uuid.UUID]*model.Run)
	runTokens         = make(map[string]uuid.UUID) // token -> runID
	eng               *beemengine.Engine
	svc               = api.NewFlowService()
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
//	  "exporter": "jaeger", // or "stdout", "otlp"
//	  "endpoint": "http://localhost:14268/api/traces", // Jaeger/OTLP endpoint
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
	case cfg.Tracing.Exporter == "jaeger":
		endpoint := cfg.Tracing.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:14268/api/traces"
		}
		exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpoint)))
		if err == nil {
			tp = trace.NewTracerProvider(
				trace.WithBatcher(exp),
				trace.WithResource(res),
			)
		}
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
		ctx := logger.WithRequestID(r.Context(), reqID)
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
	// Register HTTP interfaces for metadata discovery
	httpMetas := []registry.InterfaceMeta{
		{ID: registry.InterfaceIDListRuns, Type: registry.HTTP, Use: http.MethodGet, Path: "/runs", Description: registry.InterfaceDescListRuns},
		{ID: registry.InterfaceIDStartRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/runs", Description: registry.InterfaceDescStartRun},
		{ID: registry.InterfaceIDGetRun, Type: registry.HTTP, Use: http.MethodGet, Path: "/runs/{id}", Description: registry.InterfaceDescGetRun},
		{ID: registry.InterfaceIDResumeRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/resume/{token}", Description: registry.InterfaceDescResumeRun},
		{ID: registry.InterfaceIDGraphFlow, Type: registry.HTTP, Use: http.MethodGet, Path: "/graph", Description: registry.InterfaceDescGraphFlow},
		{ID: registry.InterfaceIDValidateFlow, Type: registry.HTTP, Use: http.MethodPost, Path: "/validate", Description: registry.InterfaceDescValidateFlow},
		{ID: registry.InterfaceIDTestFlow, Type: registry.HTTP, Use: http.MethodPost, Path: "/test", Description: registry.InterfaceDescTestFlow},
		{ID: registry.InterfaceIDAssistantChat, Type: registry.HTTP, Use: http.MethodPost, Path: "/assistant/chat", Description: registry.InterfaceDescAssistantChat},
		{ID: registry.InterfaceIDInlineRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/runs/inline", Description: registry.InterfaceDescInlineRun},
		{ID: registry.InterfaceIDListTools, Type: registry.HTTP, Use: http.MethodGet, Path: "/tools", Description: registry.InterfaceDescListTools},
		{ID: registry.InterfaceIDGetToolManifest, Type: registry.HTTP, Use: http.MethodGet, Path: "/tools/{name}", Description: registry.InterfaceDescGetToolManifest},
		{ID: registry.InterfaceIDListFlows, Type: registry.HTTP, Use: http.MethodGet, Path: "/flows", Description: registry.InterfaceDescListFlows},
		{ID: registry.InterfaceIDGetFlowSpec, Type: registry.HTTP, Use: http.MethodGet, Path: "/flows/{name}", Description: registry.InterfaceDescGetFlowSpec},
		{ID: registry.InterfaceIDPublishEvent, Type: registry.HTTP, Use: http.MethodPost, Path: "/events", Description: registry.InterfaceDescPublishEvent},
		{ID: registry.InterfaceIDSpec, Type: registry.HTTP, Use: http.MethodGet, Path: "/spec", Description: "Get BeemFlow protocol spec"},
	}
	for _, m := range httpMetas {
		registry.RegisterInterface(m)
	}
	// Serve static files (e.g., index.html) from project root at '/'
	mux := http.NewServeMux()
	// Serve static files
	registry.RegisterRoute(mux, "GET", "/", registry.InterfaceDescStaticAssets, http.FileServer(http.Dir(".")).ServeHTTP)

	// Metadata discovery endpoint
	registry.RegisterRoute(mux, "GET", "/metadata", registry.InterfaceDescMetadata, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registry.AllInterfaces())
	})

	// Health check endpoint
	registry.RegisterRoute(mux, "GET", "/healthz", registry.InterfaceDescHealthCheck, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Dependency injection: construct all dependencies at the top
	var err error
	var store storage.Storage
	if cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return logger.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return logger.Errorf("failed to initialize storage: %w", err)
		}
	} else {
		// Default to SQLite
		sqliteStore, err := storage.NewSqliteStorage(config.DefaultSQLiteDSN)
		if err != nil {
			logger.WarnCtx(context.Background(), "Failed to create default sqlite storage: %v, using in-memory fallback", "error", err)
			store = storage.NewMemoryStorage()
		} else {
			store = sqliteStore
		}
	}

	adapters := beemengine.NewDefaultAdapterRegistry(context.Background())
	templ := templater.NewTemplater()
	var bus event.EventBus
	if cfg.Event != nil {
		bus, err = event.NewEventBusFromConfig(cfg.Event)
		if err != nil {
			logger.WarnCtx(context.Background(), "Failed to create event bus: %v, using in-memory fallback", "error", err)
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
		logger.WarnCtx(context.Background(), "Failed to create blob store: %v, using nil fallback", "error", err)
		blobStore = nil
	}

	eng = beemengine.NewEngine(adapters, templ, bus, blobStore, store)
	mux.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		if eng == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		if r.Method == http.MethodGet {
			runsListHandler(w, r)
		} else {
			runsHandler(w, r)
		}
	})
	mux.HandleFunc("/runs/", func(w http.ResponseWriter, r *http.Request) {
		if eng == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		runStatusHandler(w, r)
	})
	mux.HandleFunc("/resume/", func(w http.ResponseWriter, r *http.Request) {
		resumeHandler(w, r)
	})
	mux.HandleFunc("/graph", graphHandler)
	mux.HandleFunc("/validate", validateHandler)
	mux.HandleFunc("/test", testHandler)
	mux.HandleFunc("/assistant/chat", assistantChatHandler)
	mux.HandleFunc("/runs/inline", runsInlineHandler)
	mux.HandleFunc("/tools", toolsIndexHandler)
	mux.HandleFunc("/tools/", toolsManifestHandler)
	mux.HandleFunc("/flows", flowsHandler)
	mux.HandleFunc("/flows/", flowSpecHandler)
	mux.HandleFunc("/events", eventsHandler)
	mux.HandleFunc("/spec", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte(docs.BeemflowSpec))
	})
	mux.Handle("/metrics", promhttp.Handler())

	addr := ":8080"
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
		Addr:    addr,
		Handler: wrappedMux,
	}

	// Channel to listen for errors from ListenAndServe
	errChan := make(chan error, 1)
	go func() {
		logger.Info("HTTP server starting on %s", addr)
		errChan <- server.ListenAndServe()
	}()

	// Listen for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Received signal %v, shutting down HTTP server...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("HTTP server shutdown error: %v", err)
			return err
		}
		logger.Info("HTTP server shutdown complete.")
		return nil
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: %v", err)
			return err
		}
		return nil
	}
}

// GET /runs (list all runs)
func runsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	runs, err := svc.ListRuns(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			logger.ErrorCtx(r.Context(), "w.Write failed", "error", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(runs); err != nil {
		logger.ErrorCtx(r.Context(), "json.Encode failed", "error", err)
	}
}

// POST /runs { flow: <filename>, event: <object> }
func runsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Flow  string         `json:"flow"`
		Event map[string]any `json:"event"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	id, err := svc.StartRun(r.Context(), req.Flow, req.Event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	resp := map[string]any{
		"run_id": id.String(),
		"status": "STARTED",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("json.Encode failed: %v", err)
	}
}

// GET /runs/{id}
func runStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		idStr := r.URL.Path[len("/runs/"):]
		id, err := uuid.Parse(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("invalid run ID")); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		run, err := svc.GetRun(r.Context(), id)
		if err != nil || run == nil {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte("run not found")); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(run); err != nil {
			logger.Error("json.Encode failed: %v", err)
		}
		return
	} else if r.Method == http.MethodDelete {
		idStr := r.URL.Path[len("/runs/"):]
		id, err := uuid.Parse(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("invalid run ID")); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		if eng == nil || eng.Storage == nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte("engine/storage not initialized")); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		err = svc.DeleteRun(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("deleted")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// POST /resume/{token}
func resumeHandler(w http.ResponseWriter, r *http.Request) {
	tokenOrID := r.URL.Path[len("/resume/"):]
	// Direct runID update for tests
	if id, err := uuid.Parse(tokenOrID); err == nil {
		// Decode JSON body
		var resumeEvent map[string]any
		if err := json.NewDecoder(r.Body).Decode(&resumeEvent); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Update event map
		runsMu.Lock()
		if run, ok := runs[id]; ok {
			for k, v := range resumeEvent {
				run.Event[k] = v
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": run.Status, "outputs": run.Event["outputs"]})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		runsMu.Unlock()
		return
	}
	// Token-based resume if engine initialized
	if eng != nil {
		runsMu.Lock()
		runID, ok := runTokens[tokenOrID]
		run, ok2 := runs[runID]
		runsMu.Unlock()
		if !ok || !ok2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		eng.Resume(r.Context(), tokenOrID, nil)
		outputs := eng.GetCompletedOutputs(tokenOrID)
		runsMu.Lock()
		if outputs != nil {
			run.Event["outputs"] = outputs
			run.Status = model.RunSucceeded
			ended := time.Now()
			run.EndedAt = &ended
		}
		runsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": run.Status, "outputs": outputs})
		return
	}
	// Invalid run ID
	w.WriteHeader(http.StatusBadRequest)
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flowName := r.URL.Query().Get("flow")
	if flowName == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing flow name")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	graph, err := svc.GraphFlow(r.Context(), flowName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "text/vnd.mermaid")
	if _, err := w.Write([]byte(graph)); err != nil {
		logger.Error("w.Write failed: %v", err)
	}
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Flow string `json:"flow"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	err := svc.ValidateFlow(r.Context(), req.Flow)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		logger.Error("w.Write failed: %v", err)
	}
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateRunEvent updates the event for a run in the in-memory map. Used for tests.
func UpdateRunEvent(id uuid.UUID, newEvent map[string]any) error {
	runsMu.Lock()
	defer runsMu.Unlock()
	run, ok := runs[id]
	if !ok {
		return logger.Errorf("run not found")
	}
	run.Event = newEvent
	return nil
}

func assistantChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Messages []string `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	draft, errors, err := svc.AssistantChat(r.Context(), "", req.Messages)
	resp := map[string]any{
		"draft":  draft,
		"errors": errors,
	}
	if err != nil {
		resp["error"] = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("json.Encode failed: %v", err)
	}
}

func runsInlineHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Spec  string         `json:"spec"`
		Event map[string]any `json:"event"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	// Parse and validate the flow spec
	flow, err := api.ParseFlowFromString(req.Spec)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid flow spec: " + err.Error())); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	// Start the run inline
	id, outputs, err := svc.RunSpec(r.Context(), flow, req.Event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("run error: " + err.Error())); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	resp := map[string]any{
		"run_id":  id.String(),
		"outputs": outputs,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("json.Encode failed: %v", err)
	}
}

// toolsIndexHandler returns a JSON list of all registered tool manifests from the registry index.json.
func toolsIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tools, err := svc.ListTools(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("failed to list tools")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		logger.Error("json.Encode failed: %v", err)
	}
}

// toolsManifestHandler returns the manifest for a single tool by name from the registry index.json.
func toolsManifestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	nameWithExt := strings.TrimPrefix(r.URL.Path, "/tools/")
	name := strings.TrimSuffix(nameWithExt, ".json")
	manifest, err := svc.GetToolManifest(r.Context(), name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("failed to get tool manifest")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		logger.Error("json.Encode failed: %v", err)
	}
}

// Handler: GET /flows (list all flow specs), POST /flows (upload/update flow)
func flowsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// List all flow specs
		flows, err := svc.ListFlows(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		var specs []any
		for _, name := range flows {
			flow, err := svc.GetFlow(r.Context(), name)
			if err != nil {
				continue
			}
			specs = append(specs, flow)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(specs); err != nil {
			logger.Error("json.Encode failed: %v", err)
		}
		return
	} else if r.Method == http.MethodPost {
		// Upload or update a flow (stub)
		w.WriteHeader(http.StatusNotImplemented)
		if _, err := w.Write([]byte("upload/update flow not implemented yet")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// Handler: GET /flows/{name} (get flow spec), DELETE /flows/{name} (delete flow)
func flowSpecHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/flows/")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing flow name")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	if r.Method == http.MethodGet {
		flow, err := svc.GetFlow(r.Context(), name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(flow); err != nil {
			logger.Error("json.Encode failed: %v", err)
		}
		return
	} else if r.Method == http.MethodDelete {
		// Delete flow (remove YAML file)
		path := config.DefaultFlowsDir + "/" + name + ".flow.yaml"
		err := os.Remove(path)
		if err != nil {
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
				if _, err := w.Write([]byte("flow not found")); err != nil {
					logger.Error("w.Write failed: %v", err)
				}
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("deleted")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// Handler: POST /events (publish event)
func eventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Topic   string         `json:"topic"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	err := svc.PublishEvent(r.Context(), req.Topic, req.Payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		logger.Error("w.Write failed: %v", err)
	}
}
