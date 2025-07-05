package http

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
	serverlessMux  *http.ServeMux
	muxMutex       sync.RWMutex
)

// ServerlessHandler is the minimal Vercel function for BeemFlow
func ServerlessHandler(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Initialize once
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
		
		if initErr == nil {
			serverlessMux = createServerlessMux()
		}
	})

	if initErr != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Use the cached mux
	muxMutex.RLock()
	mux := serverlessMux
	muxMutex.RUnlock()

	if mux == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	mux.ServeHTTP(w, r)
}

// createServerlessMux creates the HTTP multiplexer with all routes
func createServerlessMux() *http.ServeMux {
	mux := http.NewServeMux()
	
	// Generate handlers based on environment filtering
	endpoints := strings.TrimSpace(os.Getenv("BEEMFLOW_ENDPOINTS"))
	if endpoints != "" {
		// Split by comma and trim spaces
		groups := make([]string, 0)
		for _, group := range strings.Split(endpoints, ",") {
			if trimmed := strings.TrimSpace(group); trimmed != "" {
				groups = append(groups, trimmed)
			}
		}
		if len(groups) > 0 {
			filteredOps := api.GetOperationsMapByGroups(groups)
			api.GenerateHTTPHandlersForOperations(mux, filteredOps)
		} else {
			api.GenerateHTTPHandlers(mux)
		}
	} else {
		api.GenerateHTTPHandlers(mux)
	}

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy"}`))
	})

	return mux
}

// ResetServerlessMux resets the serverless mux (for testing)
func ResetServerlessMux() {
	muxMutex.Lock()
	defer muxMutex.Unlock()
	
	// Reset the Once so initialization can happen again
	initServerless = sync.Once{}
	initErr = nil
	serverlessMux = nil
}
