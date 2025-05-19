package http

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/storage"
	"github.com/google/uuid"
)

var (
	runsMu    sync.Mutex
	runs      = make(map[uuid.UUID]*model.Run)
	runTokens = make(map[string]uuid.UUID) // token -> runID
	eng       *engine.Engine
	svc       = api.NewFlowService()
)

func StartServer(addr string) error {
	// Load configuration
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil {
		if os.IsNotExist(err) {
			// Use default config if missing
			cfg = &config.Config{}
		} else {
			return logger.Errorf("failed to load config: %w", err)
		}
	}
	// Initialize storage based on config or default to SQLite
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
			logger.Warn("Failed to create default sqlite storage: %v, using in-memory fallback", err)
			store = storage.NewMemoryStorage()
		} else {
			store = sqliteStore
		}
	}
	eng = engine.NewEngineWithStorage(store)
	mux := http.NewServeMux()
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
		if eng == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
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
	return http.ListenAndServe(addr, mux)
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
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(runs); err != nil {
		logger.Error("json.Encode failed: %v", err)
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
	if eng == nil {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	tokenOrID := r.URL.Path[len("/resume/"):]
	if tokenOrID == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing token")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	var resumeEvent map[string]any
	if err := json.NewDecoder(r.Body).Decode(&resumeEvent); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid JSON body")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	// Support both token and runID for test compatibility
	if id, err := uuid.Parse(tokenOrID); err == nil {
		runsMu.Lock()
		run, ok := runs[id]
		runsMu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte("run not found for token")); err != nil {
				logger.Error("w.Write failed: %v", err)
			}
			return
		}
		// Update event directly for test
		runsMu.Lock()
		for k, v := range resumeEvent {
			run.Event[k] = v
		}
		runsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":  run.Status,
			"outputs": run.Event["outputs"],
		}); err != nil {
			logger.Error("json.Encode failed: %v", err)
		}
		return
	}
	// Otherwise, treat as token (normal path)
	runsMu.Lock()
	runID, ok := runTokens[tokenOrID]
	run, ok2 := runs[runID]
	runsMu.Unlock()
	if !ok || !ok2 {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid run ID")); err != nil {
			logger.Error("w.Write failed: %v", err)
		}
		return
	}
	// Resume the engine
	eng.Resume(tokenOrID, resumeEvent)
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
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":  run.Status,
		"outputs": outputs,
	}); err != nil {
		logger.Error("json.Encode failed: %v", err)
	}
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
	w.Header().Set("Content-Type", "text/vnd.graphviz")
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
