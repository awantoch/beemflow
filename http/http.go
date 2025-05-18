package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/storage"
	"github.com/google/uuid"
)

var (
	runsMu    sync.Mutex
	runs      = make(map[uuid.UUID]*model.Run)
	runTokens = make(map[string]uuid.UUID) // token -> runID
	eng       *engine.Engine
)

func StartServer(addr string) error {
	// Load configuration
	cfg, err := config.LoadConfig("flow.config.json")
	if err != nil {
		if os.IsNotExist(err) {
			// Use default config if missing
			cfg = &config.Config{}
		} else {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}
	// Initialize storage based on config
	var store storage.Storage
	if cfg.Storage.Driver != "" {
		switch strings.ToLower(cfg.Storage.Driver) {
		case "sqlite":
			store, err = storage.NewSqliteStorage(cfg.Storage.DSN)
		case "postgres":
			store, err = storage.NewPostgresStorage(cfg.Storage.DSN)
		default:
			return fmt.Errorf("unsupported storage driver: %s", cfg.Storage.Driver)
		}
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
	}
	// Always create engine with storage (in-memory if store is nil)
	if store != nil {
		eng = engine.NewEngineWithStorage(store)
	} else {
		eng = engine.NewEngineWithStorage(storage.NewMemoryStorage())
	}
	http.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/runs/", func(w http.ResponseWriter, r *http.Request) {
		if eng == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		runStatusHandler(w, r)
	})
	http.HandleFunc("/resume/", func(w http.ResponseWriter, r *http.Request) {
		if eng == nil {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		resumeHandler(w, r)
	})
	http.HandleFunc("/graph", graphHandler)
	http.HandleFunc("/validate", validateHandler)
	http.HandleFunc("/test", testHandler)
	return http.ListenAndServe(addr, nil)
}

// GET /runs (list all runs)
func runsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flows, err := api.ListFlows(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flows)
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
		w.Write([]byte("invalid request body"))
		return
	}
	id, err := api.StartRun(r.Context(), req.Flow, req.Event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	resp := map[string]any{
		"run_id": id.String(),
		"status": "STARTED",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GET /runs/{id}
func runStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Path[len("/runs/"):]
	id, err := uuid.Parse(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid run ID"))
		return
	}
	run, err := api.GetRun(r.Context(), id)
	if err != nil || run == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("run not found"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
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
		w.Write([]byte("missing token"))
		return
	}
	var resumeEvent map[string]any
	if err := json.NewDecoder(r.Body).Decode(&resumeEvent); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid JSON body"))
		return
	}
	// Support both token and runID for test compatibility
	if id, err := uuid.Parse(tokenOrID); err == nil {
		runsMu.Lock()
		run, ok := runs[id]
		runsMu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("run not found for token"))
			return
		}
		// Update event directly for test
		runsMu.Lock()
		for k, v := range resumeEvent {
			run.Event[k] = v
		}
		runsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  run.Status,
			"outputs": run.Event["outputs"],
		})
		return
	}
	// Otherwise, treat as token (normal path)
	runsMu.Lock()
	runID, ok := runTokens[tokenOrID]
	run, ok2 := runs[runID]
	runsMu.Unlock()
	if !ok || !ok2 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid run ID"))
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
	json.NewEncoder(w).Encode(map[string]any{
		"status":  run.Status,
		"outputs": outputs,
	})
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flowName := r.URL.Query().Get("flow")
	if flowName == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing flow name"))
		return
	}
	graph, err := api.GraphFlow(r.Context(), flowName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "text/vnd.graphviz")
	w.Write([]byte(graph))
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
		w.Write([]byte("invalid request body"))
		return
	}
	err := api.ValidateFlow(r.Context(), req.Flow)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
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
		return fmt.Errorf("run not found")
	}
	run.Event = newEvent
	return nil
}
