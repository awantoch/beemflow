package http

import (
	"encoding/json"
	"fmt"
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
	w.WriteHeader(http.StatusNotImplemented)
}

func resumeHandler(w http.ResponseWriter, r *http.Request) {
	runIDStr := r.URL.Path[len("/resume/"):]
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid run ID"))
		return
	}

	var newEvent map[string]any
	if err := json.NewDecoder(r.Body).Decode(&newEvent); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid JSON body"))
		return
	}

	runsMu.Lock()
	run, ok := runs[runID]
	if !ok {
		runsMu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("run not found"))
		return
	}
	run.Event = newEvent
	runsMu.Unlock()

	// TODO: trigger actual resumption of the flow execution
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("event context updated for run"))
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented yet. Planned for a future release.
	w.WriteHeader(http.StatusNotImplemented)
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented yet. Planned for a future release.
	w.WriteHeader(http.StatusNotImplemented)
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented yet. Planned for a future release.
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateRunEvent updates the event context for a run by ID.
func UpdateRunEvent(runID uuid.UUID, newEvent map[string]any) error {
	runsMu.Lock()
	defer runsMu.Unlock()
	run, ok := runs[runID]
	if !ok {
		return fmt.Errorf("run not found")
	}
	run.Event = newEvent
	return nil
}
