package handler

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/awantoch/beemflow/config"
	api "github.com/awantoch/beemflow/core"
)

var (
	initServerless sync.Once
	initErr        error
	// Cache the HTTP multiplexer to avoid regenerating on every request
	cachedMux      *http.ServeMux
	muxMutex       sync.RWMutex
)

// Handler is the entry point for Vercel serverless functions
func Handler(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Initialize once (includes handler generation)
	initServerless.Do(func() {
		cfg := &config.Config{
			Storage: config.StorageConfig{
				Driver: "sqlite",
				DSN:    os.Getenv("DATABASE_URL"),
			},
			FlowsDir: os.Getenv("FLOWS_DIR"),
		}
		if cfg.Storage.DSN == "" {
			cfg.Storage.DSN = ":memory:"
		}
		if cfg.FlowsDir != "" {
			api.SetFlowsDir(cfg.FlowsDir)
		}
		_, initErr = api.InitializeDependencies(cfg)
		
		// Generate handlers ONCE during initialization, not on every request
		if initErr == nil {
			mux := http.NewServeMux()
			if endpoints := os.Getenv("BEEMFLOW_ENDPOINTS"); endpoints != "" {
				filteredOps := api.GetOperationsMapByGroups(strings.Split(endpoints, ","))
				api.GenerateHTTPHandlersForOperations(mux, filteredOps)
			} else {
				api.GenerateHTTPHandlers(mux)
			}
			
			// Add health check
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status":"healthy"}`))
			})
			
			// Cache the mux
			muxMutex.Lock()
			cachedMux = mux
			muxMutex.Unlock()
		}
	})

	if initErr != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Use cached mux for all requests
	muxMutex.RLock()
	mux := cachedMux
	muxMutex.RUnlock()
	
	if mux == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	mux.ServeHTTP(w, r)
}
