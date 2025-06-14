package http

import "net/http"

// Handler is the entry point for Vercel serverless functions
// It delegates to the integrated ServerlessHandler in this same package
func Handler(w http.ResponseWriter, r *http.Request) {
	ServerlessHandler(w, r)
}
