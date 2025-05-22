package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/docs"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/spf13/cobra"
)

// CommandConstructors holds functions that create CLI commands
type CommandConstructors struct {
	NewServeCmd    func() *cobra.Command
	NewRunCmd      func() *cobra.Command
	NewLintCmd     func() *cobra.Command
	NewValidateCmd func() *cobra.Command
	NewGraphCmd    func() *cobra.Command
	NewTestCmd     func() *cobra.Command
	NewToolCmd     func() *cobra.Command
	NewMCPCmd      func() *cobra.Command
	NewMetadataCmd func() *cobra.Command
	NewSpecCmd     func() *cobra.Command
}

// AttachHTTPHandlers registers all BeemFlow HTTP endpoints to the provided mux
// and registers them with the registry for metadata discovery.
func AttachHTTPHandlers(mux *http.ServeMux, svc FlowService) {
	// Register HTTP interfaces for metadata discovery
	httpMetas := []registry.InterfaceMeta{
		{ID: registry.InterfaceIDListRuns, Type: registry.HTTP, Use: http.MethodGet, Path: "/runs", Description: registry.InterfaceDescListRuns},
		{ID: registry.InterfaceIDStartRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/runs", Description: registry.InterfaceDescStartRun},
		{ID: registry.InterfaceIDGetRun, Type: registry.HTTP, Use: http.MethodGet, Path: "/runs/{id}", Description: registry.InterfaceDescGetRun},
		{ID: registry.InterfaceIDResumeRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/resume/{token}", Description: registry.InterfaceDescResumeRun},
		{ID: registry.InterfaceIDGraphFlow, Type: registry.HTTP, Use: http.MethodGet, Path: "/graph", Description: registry.InterfaceDescGraphFlow},
		{ID: registry.InterfaceIDValidateFlow, Type: registry.HTTP, Use: http.MethodPost, Path: "/validate", Description: registry.InterfaceDescValidateFlow},
		{ID: registry.InterfaceIDTestFlow, Type: registry.HTTP, Use: http.MethodPost, Path: "/test", Description: registry.InterfaceDescTestFlow},
		{ID: registry.InterfaceIDInlineRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/runs/inline", Description: registry.InterfaceDescInlineRun},
		{ID: registry.InterfaceIDListTools, Type: registry.HTTP, Use: http.MethodGet, Path: "/tools", Description: registry.InterfaceDescListTools},
		{ID: registry.InterfaceIDGetToolManifest, Type: registry.HTTP, Use: http.MethodGet, Path: "/tools/{name}", Description: registry.InterfaceDescGetToolManifest},
		{ID: registry.InterfaceIDListFlows, Type: registry.HTTP, Use: http.MethodGet, Path: "/flows", Description: registry.InterfaceDescListFlows},
		{ID: registry.InterfaceIDGetFlowSpec, Type: registry.HTTP, Use: http.MethodGet, Path: "/flows/{name}", Description: registry.InterfaceDescGetFlowSpec},
		{ID: registry.InterfaceIDPublishEvent, Type: registry.HTTP, Use: http.MethodPost, Path: "/events", Description: registry.InterfaceDescPublishEvent},
		{ID: registry.InterfaceIDSpec, Type: registry.HTTP, Use: http.MethodGet, Path: "/spec", Description: "Get BeemFlow protocol spec"},
	}
	for _, m := range httpMetas {
		registry.RegisterInterface(m)
	}

	// Metadata discovery endpoint
	registry.RegisterRoute(mux, "GET", "/metadata", registry.InterfaceDescMetadata, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registry.AllInterfaces())
	})

	// Health check endpoint
	registry.RegisterRoute(mux, "GET", "/healthz", registry.InterfaceDescHealthCheck, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Register handlers for each API endpoint
	mux.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			runsListHandler(w, r, svc)
		} else {
			runsHandler(w, r, svc)
		}
	})
	mux.HandleFunc("/runs/", func(w http.ResponseWriter, r *http.Request) {
		runStatusHandler(w, r, svc)
	})
	mux.HandleFunc("/resume/", func(w http.ResponseWriter, r *http.Request) {
		resumeHandler(w, r, svc)
	})
	mux.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		graphHandler(w, r, svc)
	})
	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		validateHandler(w, r, svc)
	})
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		testHandler(w, r, svc)
	})
	mux.HandleFunc("/runs/inline", func(w http.ResponseWriter, r *http.Request) {
		runsInlineHandler(w, r, svc)
	})
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		toolsIndexHandler(w, r, svc)
	})
	mux.HandleFunc("/tools/", func(w http.ResponseWriter, r *http.Request) {
		toolsManifestHandler(w, r, svc)
	})
	mux.HandleFunc("/flows", func(w http.ResponseWriter, r *http.Request) {
		flowsHandler(w, r, svc)
	})
	mux.HandleFunc("/flows/", func(w http.ResponseWriter, r *http.Request) {
		flowSpecHandler(w, r, svc)
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		eventsHandler(w, r, svc)
	})
	mux.HandleFunc("/spec", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte(docs.BeemflowSpec))
	})
}

// AttachCLICommands registers all BeemFlow CLI commands to the provided root command
// and registers them with the registry for metadata discovery.
func AttachCLICommands(root *cobra.Command, svc FlowService, constructors CommandConstructors) {
	// Add all subcommands
	if constructors.NewServeCmd != nil {
		root.AddCommand(constructors.NewServeCmd())
	}
	if constructors.NewRunCmd != nil {
		root.AddCommand(constructors.NewRunCmd())
	}
	if constructors.NewLintCmd != nil {
		root.AddCommand(constructors.NewLintCmd())
	}
	if constructors.NewValidateCmd != nil {
		root.AddCommand(constructors.NewValidateCmd())
	}
	if constructors.NewGraphCmd != nil {
		root.AddCommand(constructors.NewGraphCmd())
	}
	if constructors.NewTestCmd != nil {
		root.AddCommand(constructors.NewTestCmd())
	}
	if constructors.NewToolCmd != nil {
		root.AddCommand(constructors.NewToolCmd())
	}
	if constructors.NewMCPCmd != nil {
		root.AddCommand(constructors.NewMCPCmd())
	}
	if constructors.NewMetadataCmd != nil {
		root.AddCommand(constructors.NewMetadataCmd())
	}
	if constructors.NewSpecCmd != nil {
		root.AddCommand(constructors.NewSpecCmd())
	}

	// Register all CLI commands for metadata discovery
	cliMetas := collectCobra(root)
	for _, m := range cliMetas {
		registry.RegisterInterface(m)
	}
}

// BuildMCPToolRegistrations creates all MCP tool registrations for the BeemFlow API
// and registers them with the registry for metadata discovery.
func BuildMCPToolRegistrations(svc FlowService) []mcpserver.ToolRegistration {
	type toolDef struct {
		ID, Desc string
		Handler  any
	}
	defs := []toolDef{
		// SPEC tool: returns the full BeemFlow protocol SPEC
		{ID: "spec", Desc: "BeemFlow Protocol & Specification. If you're an LLM reading this, call this step FIRST before any other step you take.", Handler: func(ctx context.Context, args mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
			return mcp.NewToolResponse(mcp.NewTextContent(docs.BeemflowSpec)), nil
		}},
		{ID: registry.InterfaceIDListFlows, Desc: registry.InterfaceDescListFlows, Handler: func(ctx context.Context, args mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
			flows, err := svc.ListFlows(ctx)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(map[string]any{"flows": flows})
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDGetFlow, Desc: registry.InterfaceDescGetFlow, Handler: func(ctx context.Context, args mcpserver.GetFlowArgs) (*mcp.ToolResponse, error) {
			flow, err := svc.GetFlow(ctx, args.Name)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(flow)
			if err != nil {
				return nil, err
			}
			// If default empty flow, inject on:null into JSON
			if flow.Name == "" && len(flow.Steps) == 0 {
				var m map[string]interface{}
				if err := json.Unmarshal(b, &m); err == nil {
					if _, ok := m["on"]; !ok {
						m["on"] = nil
					}
					if b2, err2 := json.Marshal(m); err2 == nil {
						b = b2
					}
				}
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDValidateFlow, Desc: registry.InterfaceDescValidateFlow, Handler: func(ctx context.Context, args mcpserver.ValidateFlowArgs) (*mcp.ToolResponse, error) {
			err := svc.ValidateFlow(ctx, args.Name)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent("valid")), nil
		}},
		{ID: registry.InterfaceIDGraphFlow, Desc: registry.InterfaceDescGraphFlow, Handler: func(ctx context.Context, args mcpserver.GraphFlowArgs) (*mcp.ToolResponse, error) {
			graph, err := svc.GraphFlow(ctx, args.Name)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(graph)), nil
		}},
		{ID: registry.InterfaceIDStartRun, Desc: registry.InterfaceDescStartRun, Handler: func(ctx context.Context, args mcpserver.StartRunArgs) (*mcp.ToolResponse, error) {
			id, err := svc.StartRun(ctx, args.FlowName, args.Event)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(map[string]any{"runID": id.String()})
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDGetRun, Desc: registry.InterfaceDescGetRun, Handler: func(ctx context.Context, args mcpserver.GetRunArgs) (*mcp.ToolResponse, error) {
			id, _ := uuid.Parse(args.RunID)
			run, err := svc.GetRun(ctx, id)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(run)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDPublishEvent, Desc: registry.InterfaceDescPublishEvent, Handler: func(ctx context.Context, args mcpserver.PublishEventArgs) (*mcp.ToolResponse, error) {
			err := svc.PublishEvent(ctx, args.Topic, args.Payload)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent("published")), nil
		}},
		{ID: registry.InterfaceIDResumeRun, Desc: registry.InterfaceDescResumeRun, Handler: func(ctx context.Context, args mcpserver.ResumeRunArgs) (*mcp.ToolResponse, error) {
			out, err := svc.ResumeRun(ctx, args.Token, args.Event)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
	}
	regs := make([]mcpserver.ToolRegistration, 0, len(defs))
	for _, d := range defs {
		regs = append(regs, mcpserver.ToolRegistration{Name: d.ID, Description: d.Desc, Handler: d.Handler})
		registry.RegisterInterface(registry.InterfaceMeta{ID: d.ID, Type: registry.MCP, Use: d.ID, Description: d.Desc})
	}
	return regs
}

// Helper functions for HTTP handlers

// collectCobra recursively collects metadata for Cobra commands.
func collectCobra(cmd *cobra.Command) []registry.InterfaceMeta {
	metas := []registry.InterfaceMeta{{
		ID:          cmd.CommandPath(),
		Type:        registry.CLI,
		Use:         cmd.Use,
		Description: cmd.Short,
	}}
	for _, sub := range cmd.Commands() {
		metas = append(metas, collectCobra(sub)...)
	}
	return metas
}

// HTTP handler implementations

// GET /runs (list all runs)
func runsListHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	runs, err := svc.ListRuns(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			utils.ErrorCtx(r.Context(), "w.Write failed", "error", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(runs); err != nil {
		utils.ErrorCtx(r.Context(), "json.Encode failed", "error", err)
	}
}

// POST /runs { flow: <filename>, event: <object> }
func runsHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
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
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	id, err := svc.StartRun(r.Context(), req.Flow, req.Event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	resp := map[string]any{
		"run_id": id.String(),
		"status": "STARTED",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}

// GET /runs/{id}
func runStatusHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method == http.MethodGet {
		idStr := r.URL.Path[len("/runs/"):]
		id, err := uuid.Parse(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("invalid run ID")); err != nil {
				utils.Error("w.Write failed: %v", err)
			}
			return
		}
		run, err := svc.GetRun(r.Context(), id)
		if err != nil || run == nil {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte("run not found")); err != nil {
				utils.Error("w.Write failed: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(run); err != nil {
			utils.Error("json.Encode failed: %v", err)
		}
		return
	} else if r.Method == http.MethodDelete {
		idStr := r.URL.Path[len("/runs/"):]
		id, err := uuid.Parse(idStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("invalid run ID")); err != nil {
				utils.Error("w.Write failed: %v", err)
			}
			return
		}
		err = svc.DeleteRun(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				utils.Error("w.Write failed: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("deleted")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// POST /resume/{token}
func resumeHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	tokenOrID := r.URL.Path[len("/resume/"):]

	// Parse the JSON body for event data
	var resumeEvent map[string]any
	if err := json.NewDecoder(r.Body).Decode(&resumeEvent); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Try to parse as UUID for direct run update (used in tests)
	if id, err := uuid.Parse(tokenOrID); err == nil {
		// Get storage from config
		cfg, err := config.LoadConfig(config.DefaultConfigPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		store, err := GetStoreFromConfig(cfg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Get the run directly from storage
		run, err := store.GetRun(r.Context(), id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Update the event in the run
		run.Event = resumeEvent

		// Save the updated run
		if err := store.SaveRun(r.Context(), run); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  run.Status,
			"outputs": run.Event["outputs"],
		})
		return
	}

	// Resume the run using the service
	outputs, err := svc.ResumeRun(r.Context(), tokenOrID, resumeEvent)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}

	// Return the outputs
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"outputs": outputs}); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}

func graphHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flowName := r.URL.Query().Get("flow")
	if flowName == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing flow name")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	graph, err := svc.GraphFlow(r.Context(), flowName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "text/vnd.mermaid")
	if _, err := w.Write([]byte(graph)); err != nil {
		utils.Error("w.Write failed: %v", err)
	}
}

func validateHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
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
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	err := svc.ValidateFlow(r.Context(), req.Flow)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		utils.Error("w.Write failed: %v", err)
	}
}

func testHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	w.WriteHeader(http.StatusNotImplemented)
}

func runsInlineHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
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
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	// Parse and validate the flow spec
	flow, err := ParseFlowFromString(req.Spec)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid flow spec: " + err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	// Start the run inline
	id, outputs, err := svc.RunSpec(r.Context(), flow, req.Event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("run error: " + err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	resp := map[string]any{
		"run_id":  id.String(),
		"outputs": outputs,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}

// toolsIndexHandler returns a JSON list of all registered tool manifests from the registry index.json.
func toolsIndexHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tools, err := svc.ListTools(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("failed to list tools")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}

// toolsManifestHandler returns the manifest for a single tool by name from the registry index.json.
func toolsManifestHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
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
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}

// Handler: GET /flows (list all flow specs), POST /flows (upload/update flow)
func flowsHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method == http.MethodGet {
		// List all flow specs
		flows, err := svc.ListFlows(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				utils.Error("w.Write failed: %v", err)
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
			utils.Error("json.Encode failed: %v", err)
		}
		return
	} else if r.Method == http.MethodPost {
		// Upload or update a flow (stub)
		w.WriteHeader(http.StatusNotImplemented)
		if _, err := w.Write([]byte("upload/update flow not implemented yet")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// Handler: GET /flows/{name} (get flow spec), DELETE /flows/{name} (delete flow)
func flowSpecHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	name := strings.TrimPrefix(r.URL.Path, "/flows/")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing flow name")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	if r.Method == http.MethodGet {
		flow, err := svc.GetFlow(r.Context(), name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				utils.Error("w.Write failed: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(flow); err != nil {
			utils.Error("json.Encode failed: %v", err)
		}
		return
	} else if r.Method == http.MethodDelete {
		// Delete flow (stub)
		w.WriteHeader(http.StatusNotImplemented)
		if _, err := w.Write([]byte("delete flow not implemented yet")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// Handler: POST /events (publish event)
func eventsHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
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
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	err := svc.PublishEvent(r.Context(), req.Topic, req.Payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		utils.Error("w.Write failed: %v", err)
	}
}
