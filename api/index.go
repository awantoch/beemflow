package handler

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/awantoch/beemflow/config"
	api "github.com/awantoch/beemflow/core"
)

// Handler is the entry point for Vercel serverless functions
func Handler(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Add serverless flag to context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, "serverless", true)
	r = r.WithContext(ctx)

	// Initialize dependencies fresh for each request
	// This ensures clean resource management - everything is created
	// and destroyed within the request lifecycle
	
	// Determine storage driver and DSN from DATABASE_URL
	var driver, dsn string
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		if strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://") {
			driver = "postgres"
			dsn = databaseURL
		} else {
			driver = "sqlite"
			dsn = databaseURL
		}
	} else {
		driver = "sqlite"
		dsn = ":memory:"
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{
			Driver: driver,
			DSN:    dsn,
		},
		FlowsDir: os.Getenv("FLOWS_DIR"),
		Event: &config.EventConfig{
			Driver: "serverless", // Serverless-optimized event bus (no goroutines)
		},
	}
	if cfg.FlowsDir != "" {
		api.SetFlowsDir(cfg.FlowsDir)
	}

	// Initialize dependencies with automatic cleanup
	cleanup, err := api.InitializeDependencies(cfg)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer cleanup() // Ensure all resources are released when request ends
		
	// Generate handlers
	mux := http.NewServeMux()
	if endpoints := os.Getenv("BEEMFLOW_ENDPOINTS"); endpoints != "" {
		filteredOps := api.GetOperationsMapByGroups(strings.Split(endpoints, ","))
		api.GenerateHTTPHandlersForOperations(mux, filteredOps)
	} else {
		api.GenerateHTTPHandlers(mux)
	}

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy"}`))
	})

	mux.ServeHTTP(w, r)
}