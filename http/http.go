package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/parser"
	"github.com/google/uuid"
)

var (
	runsMu    sync.Mutex
	runs      = make(map[uuid.UUID]*model.Run)
	runTokens = make(map[string]uuid.UUID) // token -> runID
	eng       *engine.Engine
)

func StartServer(addr string) error {
	eng = engine.NewEngine()
	http.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			runsListHandler(w, r)
		} else {
			runsHandler(w, r)
		}
	})
	http.HandleFunc("/runs/", runStatusHandler)
	http.HandleFunc("/resume/", resumeHandler)
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
	runsMu.Lock()
	defer runsMu.Unlock()
	var allRuns []map[string]interface{}
	for _, run := range runs {
		allRuns = append(allRuns, map[string]interface{}{
			"id":         run.ID.String(),
			"status":     run.Status,
			"flow":       run.FlowName,
			"started_at": run.StartedAt,
			"ended_at":   run.EndedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allRuns)
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
		w.Write([]byte("invalid JSON body"))
		return
	}
	if req.Flow == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing flow filename"))
		return
	}
	// Load and parse the flow file
	flowPath := req.Flow
	if !filepath.IsAbs(flowPath) {
		flowPath = filepath.Join("flows", flowPath)
		if _, err := os.Stat(flowPath); os.IsNotExist(err) {
			flowPath = req.Flow // try as given
		}
	}
	flow, err := parser.ParseFlow(flowPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("failed to parse flow: %v", err)))
		return
	}
	runID := uuid.New()
	go func() {
		outputs, err := eng.Execute(r.Context(), flow, req.Event)
		runsMu.Lock()
		run := &model.Run{
			ID:        runID,
			FlowName:  flow.Name,
			Event:     req.Event,
			Vars:      flow.Vars,
			Status:    model.RunSucceeded,
			StartedAt: time.Now(),
			EndedAt:   nil,
			Steps:     nil, // Not tracking step runs in this minimal version
		}
		if err != nil {
			if err.Error() == "step wait_for_resume is waiting for event (await_event pause)" {
				run.Status = model.RunWaiting
				// Find the token from event or vars
				token := ""
				if v, ok := req.Event["token"].(string); ok {
					token = v
				} else if v, ok := flow.Vars["token"].(string); ok {
					token = v
				}
				if token != "" {
					runTokens[token] = runID
				}
			}
			run.Status = model.RunWaiting
		}
		if outputs != nil {
			// Store outputs in Event for now
			run.Event["outputs"] = outputs
		}
		runs[runID] = run
		runsMu.Unlock()
	}()
	resp := map[string]any{
		"run_id": runID.String(),
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
	runsMu.Lock()
	run, ok := runs[id]
	runsMu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("run not found"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

// POST /resume/{token}
func resumeHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/resume/"):]
	if token == "" {
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
	runsMu.Lock()
	runID, ok := runTokens[token]
	run, ok2 := runs[runID]
	runsMu.Unlock()
	if !ok || !ok2 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("run not found for token"))
		return
	}
	// Resume the engine
	eng.Resume(token, resumeEvent)
	outputs := eng.GetCompletedOutputs(token)
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
	w.WriteHeader(http.StatusNotImplemented)
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
