package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/docs"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/spf13/cobra"
)

// CommandConstructors holds functions that create CLI commands.
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
		{ID: "convertOpenAPI", Type: registry.HTTP, Use: http.MethodPost, Path: "/tools/convert", Description: "Convert OpenAPI specs to BeemFlow tool manifests"},
	}
	for _, m := range httpMetas {
		registry.RegisterInterface(m)
	}

	// Metadata discovery endpoint
	registry.RegisterRoute(mux, "GET", "/metadata", registry.InterfaceDescMetadata, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		if err := json.NewEncoder(w).Encode(registry.AllInterfaces()); err != nil {
			utils.Error("Failed to encode metadata response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	// Health check endpoint
	registry.RegisterRoute(mux, "GET", "/healthz", registry.InterfaceDescHealthCheck, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			utils.Error("Failed to write health check response: %v", err)
		}
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
	mux.HandleFunc("/tools/convert", func(w http.ResponseWriter, r *http.Request) {
		convertOpenAPIHandler(w, r, svc)
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
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeTextMarkdown)
		if _, err := w.Write([]byte(docs.BeemflowSpec)); err != nil {
			utils.Error("Failed to write spec response: %v", err)
		}
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
				var m map[string]any
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
		{
			ID:   "beemflow_convert_openapi",
			Desc: "Convert OpenAPI spec to BeemFlow tool manifests",
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				// Extract required parameters
				openapi, ok := args["openapi"].(string)
				if !ok {
					return nil, fmt.Errorf("missing required parameter: openapi")
				}

				// Extract optional parameters with defaults
				apiName, _ := args["api_name"].(string)
				if apiName == "" {
					apiName = "api"
				}

				baseURL, _ := args["base_url"].(string)

				// Use the DRY helper for conversion
				return convertOpenAPISpec(ctx, openapi, apiName, baseURL)
			},
		},
	}
	regs := make([]mcpserver.ToolRegistration, 0, len(defs))
	for _, d := range defs {
		regs = append(regs, mcpserver.ToolRegistration{Name: d.ID, Description: d.Desc, Handler: d.Handler})
		registry.RegisterInterface(registry.InterfaceMeta{ID: d.ID, Type: registry.MCP, Use: d.ID, Description: d.Desc})
	}
	return regs
}

// Helper functions for HTTP handlers

// httpResponse represents a standardized HTTP response
type httpResponse struct {
	StatusCode int
	Data       any
	Error      string
}

// writeResponse standardizes JSON response writing with proper headers and error handling
func writeResponse(w http.ResponseWriter, r *http.Request, resp httpResponse) {
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(resp.StatusCode)

	var payload any
	if resp.Error != "" {
		payload = map[string]string{"error": resp.Error}
	} else {
		payload = resp.Data
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		utils.ErrorCtx(r.Context(), "Failed to encode JSON response", "error", err)
	}
}

// writeTextResponse writes a plain text response with proper error handling
func writeTextResponse(w http.ResponseWriter, r *http.Request, statusCode int, text string) {
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(text)); err != nil {
		utils.ErrorCtx(r.Context(), "Failed to write text response", "error", err)
	}
}

// decodeJSONRequest safely decodes JSON request body
func decodeJSONRequest(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}

// methodGuard ensures only specified HTTP methods are allowed
func methodGuard(w http.ResponseWriter, r *http.Request, allowedMethods ...string) bool {
	for _, method := range allowedMethods {
		if r.Method == method {
			return true
		}
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
	return false
}

// parseUUIDFromPath extracts and parses UUID from URL path
func parseUUIDFromPath(path, prefix string) (uuid.UUID, error) {
	idStr := path[len(prefix):]
	return uuid.Parse(idStr)
}

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

// GET /runs (list all runs).
func runsListHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, http.MethodGet) {
		return
	}

	runs, err := svc.ListRuns(r.Context())
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data:       runs,
	})
}

// POST /runs { flow: <filename>, event: <object> }.
func runsHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Flow  string         `json:"flow"`
		Event map[string]any `json:"event"`
	}

	if err := decodeJSONRequest(r, &req); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid request body",
		})
		return
	}

	id, err := svc.StartRun(r.Context(), req.Flow, req.Event)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data: map[string]any{
			"run_id": id.String(),
			"status": "STARTED",
		},
	})
}

// GET /runs/{id}.
func runStatusHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	switch r.Method {
	case http.MethodGet:
		handleGetRun(w, r, svc)
	case http.MethodDelete:
		handleDeleteRun(w, r, svc)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleGetRun handles GET /runs/{id}
func handleGetRun(w http.ResponseWriter, r *http.Request, svc FlowService) {
	id, err := parseUUIDFromPath(r.URL.Path, "/runs/")
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid run ID",
		})
		return
	}

	run, err := svc.GetRun(r.Context(), id)
	if err != nil || run == nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusNotFound,
			Error:      "run not found",
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data:       run,
	})
}

// handleDeleteRun handles DELETE /runs/{id}
func handleDeleteRun(w http.ResponseWriter, r *http.Request, svc FlowService) {
	id, err := parseUUIDFromPath(r.URL.Path, "/runs/")
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid run ID",
		})
		return
	}

	if err := svc.DeleteRun(r.Context(), id); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
		})
		return
	}

	writeTextResponse(w, r, http.StatusOK, "deleted")
}

// POST /resume/{token}.
func resumeHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, http.MethodPost) {
		return
	}

	tokenOrID := r.URL.Path[len("/resume/"):]

	// Parse the JSON body for event data
	var resumeEvent map[string]any
	if err := decodeJSONRequest(r, &resumeEvent); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid request body",
		})
		return
	}

	// Try to parse as UUID for direct run update (used in tests)
	if id, err := uuid.Parse(tokenOrID); err == nil {
		handleDirectRunUpdate(w, r, id, resumeEvent)
		return
	}

	// Resume the run using the service
	outputs, err := svc.ResumeRun(r.Context(), tokenOrID, resumeEvent)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data:       map[string]any{"outputs": outputs},
	})
}

// handleDirectRunUpdate handles direct run updates for test scenarios
func handleDirectRunUpdate(w http.ResponseWriter, r *http.Request, id uuid.UUID, resumeEvent map[string]any) {
	// Get storage from config
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      "failed to load config",
		})
		return
	}

	store, err := GetStoreFromConfig(cfg)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      "failed to get storage",
		})
		return
	}

	// Get the run directly from storage
	run, err := store.GetRun(r.Context(), id)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusNotFound,
			Error:      "run not found",
		})
		return
	}

	// Update the event in the run
	run.Event = resumeEvent

	// Save the updated run
	if err := store.SaveRun(r.Context(), run); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      "failed to save run",
		})
		return
	}

	// Return response
	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data: map[string]any{
			"status":  run.Status,
			"outputs": run.Event["outputs"],
		},
	})
}

// GET /graph?flow=<name>.
func graphHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, http.MethodGet) {
		return
	}

	flowName := r.URL.Query().Get("flow")
	if flowName == "" {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "missing flow parameter",
		})
		return
	}

	graph, err := svc.GraphFlow(r.Context(), flowName)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
		})
		return
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeTextVndMermaid)
	writeTextResponse(w, r, http.StatusOK, graph)
}

// POST /validate.
func validateHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, http.MethodPost) {
		return
	}

	var req struct {
		Flow string `json:"flow"`
	}

	if err := decodeJSONRequest(r, &req); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid request body",
		})
		return
	}

	if err := svc.ValidateFlow(r.Context(), req.Flow); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      fmt.Sprintf("validation failed: %v", err),
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data:       map[string]string{"status": "valid"},
	})
}

// GET /test (not implemented).
func testHandler(w http.ResponseWriter, _ *http.Request, _ FlowService) {
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
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	// Parse and validate the flow spec
	flow, err := ParseFlowFromString(req.Spec)
	if err != nil {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
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
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
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
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
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
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}

// Handler: GET /flows (list all flow specs), POST /flows (upload/update flow).
func flowsHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	switch r.Method {
	case http.MethodGet:
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
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		if err := json.NewEncoder(w).Encode(specs); err != nil {
			utils.Error("json.Encode failed: %v", err)
		}
	case http.MethodPost:
		// Upload or update a flow (stub)
		w.WriteHeader(http.StatusNotImplemented)
		if _, err := w.Write([]byte("upload/update flow not implemented yet")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Handler: GET /flows/{name} (get flow spec), DELETE /flows/{name} (delete flow).
func flowSpecHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	name := strings.TrimPrefix(r.URL.Path, "/flows/")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing flow name")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}
	switch r.Method {
	case http.MethodGet:
		flow, err := svc.GetFlow(r.Context(), name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(err.Error())); err != nil {
				utils.Error("w.Write failed: %v", err)
			}
			return
		}
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		if err := json.NewEncoder(w).Encode(flow); err != nil {
			utils.Error("json.Encode failed: %v", err)
		}
	case http.MethodDelete:
		// Delete flow (stub)
		w.WriteHeader(http.StatusNotImplemented)
		if _, err := w.Write([]byte("delete flow not implemented yet")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Handler: POST /events (publish event).
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
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
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

// convertOpenAPISpec is a DRY helper that calls the core adapter for OpenAPI conversion
func convertOpenAPISpec(ctx context.Context, openapi, apiName, baseURL string) (map[string]any, error) {
	coreAdapter := &adapter.CoreAdapter{}
	inputs := map[string]any{
		"__use":    "core.convert_openapi",
		"openapi":  openapi,
		"api_name": apiName,
		"base_url": baseURL,
	}
	return coreAdapter.Execute(ctx, inputs)
}

// convertOpenAPIHandler converts OpenAPI specs to BeemFlow tool manifests
// POST /tools/convert with JSON body: { "openapi": "...", "api_name": "my_api", "base_url": "https://..." }
func convertOpenAPIHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	_ = svc // Service not needed for this operation, uses core adapter directly
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OpenAPI string `json:"openapi"`  // OpenAPI spec as JSON string
		APIName string `json:"api_name"` // Name prefix for generated tools
		BaseURL string `json:"base_url"` // Optional base URL override
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("invalid request body: " + err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}

	if req.OpenAPI == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("missing openapi field")); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}

	if req.APIName == "" {
		req.APIName = "api" // Default name
	}

	// Use the DRY helper for conversion
	result, err := convertOpenAPISpec(r.Context(), req.OpenAPI, req.APIName, req.BaseURL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("conversion failed: " + err.Error())); err != nil {
			utils.Error("w.Write failed: %v", err)
		}
		return
	}

	// Return the result
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		utils.Error("json.Encode failed: %v", err)
	}
}
