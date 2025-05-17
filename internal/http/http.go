package http

import (
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
