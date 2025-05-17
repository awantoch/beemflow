package http

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/awantoch/beemflow/internal/model"
	"github.com/google/uuid"
)

var (
	runsMu sync.Mutex
	runs   = make(map[uuid.UUID]*model.Run)
)

func StartServer(addr string) error {
	http.HandleFunc("/runs", runsHandler)
	http.HandleFunc("/resume/", resumeHandler)
	http.HandleFunc("/graph", graphHandler)
	http.HandleFunc("/validate", validateHandler)
	http.HandleFunc("/test", testHandler)
	return http.ListenAndServe(addr, nil)
}

func runsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Create a new run
		var req struct {
			FlowID string         `json:"flow_id"`
			Event  map[string]any `json:"event"`
		}
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// For now, just create a dummy run
		runID := uuid.New()
		run := &model.Run{
			ID:       runID,
			FlowName: req.FlowID,
			Event:    req.Event,
			Status:   model.RunRunning,
		}
		runsMu.Lock()
		runs[runID] = run
		runsMu.Unlock()
		resp := map[string]any{"run_id": runID, "status": run.Status}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	case "GET":
		// Query run status
		idStr := r.URL.Query().Get("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		runsMu.Lock()
		run, ok := runs[id]
		runsMu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(run)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func resumeHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement resume logic
	w.WriteHeader(http.StatusNotImplemented)
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement graph rendering
	w.WriteHeader(http.StatusNotImplemented)
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement flow validation
	w.WriteHeader(http.StatusNotImplemented)
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement flow test endpoint
	w.WriteHeader(http.StatusNotImplemented)
}
