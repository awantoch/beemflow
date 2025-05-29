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
	registerHTTPMetadata()

	// Register system endpoints (health, metadata, spec)
	registerSystemEndpoints(mux)

	// Register API endpoints
	registerAPIEndpoints(mux, svc)
}

// registerHTTPMetadata registers all HTTP interface metadata for discovery
func registerHTTPMetadata() {
	httpMetas := []registry.InterfaceMeta{
		{ID: constants.InterfaceIDListRuns, Type: registry.HTTP, Use: http.MethodGet, Path: "/runs", Description: constants.InterfaceDescListRuns},
		{ID: constants.InterfaceIDStartRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/runs", Description: constants.InterfaceDescStartRun},
		{ID: constants.InterfaceIDGetRun, Type: registry.HTTP, Use: http.MethodGet, Path: "/runs/{id}", Description: constants.InterfaceDescGetRun},
		{ID: constants.InterfaceIDResumeRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/resume/{token}", Description: constants.InterfaceDescResumeRun},
		{ID: constants.InterfaceIDGraphFlow, Type: registry.HTTP, Use: http.MethodGet, Path: "/graph", Description: constants.InterfaceDescGraphFlow},
		{ID: constants.InterfaceIDValidateFlow, Type: registry.HTTP, Use: http.MethodPost, Path: "/validate", Description: constants.InterfaceDescValidateFlow},
		{ID: constants.InterfaceIDTestFlow, Type: registry.HTTP, Use: http.MethodPost, Path: "/test", Description: constants.InterfaceDescTestFlow},
		{ID: constants.InterfaceIDInlineRun, Type: registry.HTTP, Use: http.MethodPost, Path: "/runs/inline", Description: constants.InterfaceDescInlineRun},
		{ID: constants.InterfaceIDListTools, Type: registry.HTTP, Use: http.MethodGet, Path: "/tools", Description: constants.InterfaceDescListTools},
		{ID: constants.InterfaceIDGetToolManifest, Type: registry.HTTP, Use: http.MethodGet, Path: "/tools/{name}", Description: constants.InterfaceDescGetToolManifest},
		{ID: constants.InterfaceIDListFlows, Type: registry.HTTP, Use: http.MethodGet, Path: "/flows", Description: constants.InterfaceDescListFlows},
		{ID: constants.InterfaceIDGetFlowSpec, Type: registry.HTTP, Use: http.MethodGet, Path: "/flows/{name}", Description: constants.InterfaceDescGetFlowSpec},
		{ID: constants.InterfaceIDPublishEvent, Type: registry.HTTP, Use: http.MethodPost, Path: "/events", Description: constants.InterfaceDescPublishEvent},
		{ID: constants.InterfaceIDSpec, Type: registry.HTTP, Use: http.MethodGet, Path: "/spec", Description: constants.InterfaceDescSpec},
		{ID: constants.InterfaceIDConvertOpenAPI, Type: registry.HTTP, Use: http.MethodPost, Path: "/tools/convert", Description: constants.InterfaceDescConvertOpenAPI},
	}
	for _, m := range httpMetas {
		registry.RegisterInterface(m)
	}
}

// registerSystemEndpoints registers health check, metadata, and spec endpoints
func registerSystemEndpoints(mux *http.ServeMux) {
	// Health check endpoint
	registry.RegisterRoute(mux, constants.HTTPMethodGET, "/metadata", constants.InterfaceDescMetadata, func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(registry.AllInterfaces())
		if err != nil {
			utils.Error(constants.LogFailedEncodeMetadata, err)
			http.Error(w, constants.ResponseInvalidRequestBody, http.StatusInternalServerError)
			return
		}
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.Write(b)
	})

	registry.RegisterRoute(mux, constants.HTTPMethodGET, "/healthz", constants.InterfaceDescHealthCheck, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		if _, err := w.Write([]byte(constants.HealthCheckResponse)); err != nil {
			utils.Error(constants.LogFailedWriteHealthCheck, err)
		}
	})

	// Spec endpoint
	registry.RegisterRoute(mux, constants.HTTPMethodGET, "/spec", constants.InterfaceDescSpec, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeTextMarkdown)
		if _, err := w.Write([]byte(docs.BeemflowSpec)); err != nil {
			utils.Error(constants.LogFailedWriteSpec, err)
		}
	})
}

// registerAPIEndpoints registers all API endpoints with their handlers
func registerAPIEndpoints(mux *http.ServeMux, svc FlowService) {
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
}

// AttachCLICommands registers all BeemFlow CLI commands to the provided root command
// and registers them with the registry for metadata discovery.
func AttachCLICommands(root *cobra.Command, svc FlowService, constructors CommandConstructors) {
	// Add all subcommands
	addSubcommands(root, constructors)

	// Register CLI metadata for discovery
	registerCLIMetadata(root)
}

// addSubcommands adds all available subcommands to the root command
func addSubcommands(root *cobra.Command, constructors CommandConstructors) {
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
}

// registerCLIMetadata registers all CLI commands for metadata discovery
func registerCLIMetadata(root *cobra.Command) {
	cliMetas := collectCobra(root)
	for _, m := range cliMetas {
		registry.RegisterInterface(m)
	}
}

// toolDef represents a tool definition for MCP registration
type toolDef struct {
	ID, Desc string
	Handler  any
}

// BuildMCPToolRegistrations creates all MCP tool registrations for the BeemFlow API
// and registers them with the registry for metadata discovery.
func BuildMCPToolRegistrations(svc FlowService) []mcpserver.ToolRegistration {
	toolDefs := createMCPToolDefinitions(svc)
	return registerMCPTools(toolDefs)
}

// createMCPToolDefinitions creates all MCP tool definitions with their handlers
func createMCPToolDefinitions(svc FlowService) []toolDef {
	return []toolDef{
		{ID: constants.MCPToolSpec, Desc: constants.InterfaceDescSpecMCP, Handler: createSpecHandler()},
		{ID: constants.InterfaceIDListFlows, Desc: constants.InterfaceDescListFlows, Handler: createListFlowsHandler(svc)},
		{ID: constants.InterfaceIDValidateFlow, Desc: constants.InterfaceDescValidateFlow, Handler: createValidateFlowHandler(svc)},
		{ID: constants.InterfaceIDStartRun, Desc: constants.InterfaceDescStartRun, Handler: createStartRunHandler(svc)},
		{ID: constants.InterfaceIDGetRun, Desc: constants.InterfaceDescGetRun, Handler: createGetRunHandler(svc)},
		{ID: constants.InterfaceIDPublishEvent, Desc: constants.InterfaceDescPublishEvent, Handler: createPublishEventHandler(svc)},
		{ID: constants.MCPToolConvertOpenAPI, Desc: constants.InterfaceDescConvertOpenAPI, Handler: createConvertOpenAPIHandler()},
	}
}

// registerMCPTools registers tool definitions and creates tool registrations
func registerMCPTools(defs []toolDef) []mcpserver.ToolRegistration {
	regs := make([]mcpserver.ToolRegistration, 0, len(defs))
	for _, d := range defs {
		regs = append(regs, mcpserver.ToolRegistration{Name: d.ID, Description: d.Desc, Handler: d.Handler})
		registry.RegisterInterface(registry.InterfaceMeta{ID: d.ID, Type: registry.MCP, Use: d.ID, Description: d.Desc})
	}
	return regs
}

// MCP Tool Handler Creators - Beautiful, focused functions for each tool

// createSpecHandler creates the spec tool handler
func createSpecHandler() func(context.Context, mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
		return mcp.NewToolResponse(mcp.NewTextContent(docs.BeemflowSpec)), nil
	}
}

// createListFlowsHandler creates the MCP handler for listing flows
func createListFlowsHandler(svc FlowService) func(context.Context, map[string]any) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args map[string]any) (*mcp.ToolResponse, error) {
		flows, err := svc.ListFlows(ctx)
		if err != nil {
			return nil, err
		}

		b, err := json.Marshal(map[string]any{constants.FieldFlows: flows})
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
	}
}

// createValidateFlowHandler creates the MCP handler for validating flows
func createValidateFlowHandler(svc FlowService) func(context.Context, map[string]any) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args map[string]any) (*mcp.ToolResponse, error) {
		m, ok := args[constants.FieldFlow].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid flow argument")
		}

		// Ensure 'on' field exists
		if _, ok := m[constants.FieldOn]; !ok {
			m[constants.FieldOn] = nil
		}

		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}

		if err := svc.ValidateFlow(ctx, string(b)); err != nil {
			return nil, err
		}

		return mcp.NewToolResponse(mcp.NewTextContent(constants.StatusValid)), nil
	}
}

// createStartRunHandler creates the start run tool handler
func createStartRunHandler(svc FlowService) func(context.Context, mcpserver.StartRunArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args mcpserver.StartRunArgs) (*mcp.ToolResponse, error) {
		id, err := svc.StartRun(ctx, args.FlowName, args.Event)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(map[string]any{constants.FieldRunID: id.String()})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
	}
}

// createGetRunHandler creates the get run tool handler
func createGetRunHandler(svc FlowService) func(context.Context, mcpserver.GetRunArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args mcpserver.GetRunArgs) (*mcp.ToolResponse, error) {
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
	}
}

// createPublishEventHandler creates the publish event tool handler
func createPublishEventHandler(svc FlowService) func(context.Context, mcpserver.PublishEventArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args mcpserver.PublishEventArgs) (*mcp.ToolResponse, error) {
		err := svc.PublishEvent(ctx, args.Topic, args.Payload)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(constants.StatusPublished)), nil
	}
}

// createConvertOpenAPIHandler creates the convert OpenAPI tool handler
func createConvertOpenAPIHandler() func(context.Context, map[string]any) (any, error) {
	return func(ctx context.Context, args map[string]any) (any, error) {
		// Extract required parameters
		openapi, ok := args[constants.MCPParamOpenAPI].(string)
		if !ok {
			return nil, fmt.Errorf(constants.MCPMissingParam, constants.MCPParamOpenAPI)
		}

		// Extract optional parameters with defaults
		apiName, _ := args[constants.MCPParamAPIName].(string)
		if apiName == "" {
			apiName = constants.DefaultAPIName
		}

		baseURL, _ := args[constants.MCPParamBaseURL].(string)

		// Use the DRY helper for conversion
		return convertOpenAPISpec(ctx, openapi, apiName, baseURL)
	}
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
		payload = map[string]string{constants.FieldError: resp.Error}
	} else {
		payload = resp.Data
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		utils.ErrorCtx(r.Context(), constants.LogFailedEncodeJSON, constants.FieldError, err)
	}
}

// writeTextResponse writes a plain text response with proper error handling
func writeTextResponse(w http.ResponseWriter, r *http.Request, statusCode int, text string) {
	w.WriteHeader(statusCode)
	if _, err := w.Write([]byte(text)); err != nil {
		utils.ErrorCtx(r.Context(), constants.LogFailedWriteText, constants.FieldError, err)
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
	if !methodGuard(w, r, constants.HTTPMethodPOST) {
		return
	}

	var req struct {
		Flow  string         `json:"flow"`
		Event map[string]any `json:"event"`
	}

	if err := decodeJSONRequest(r, &req); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      constants.ResponseInvalidRequestBody,
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
			constants.FieldRunID:  id.String(),
			constants.FieldStatus: constants.StatusStarted,
		},
	})
}

// GET /runs/{id}.
func runStatusHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	id, err := parseUUIDFromPath(r.URL.Path, constants.PathRuns)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      constants.ResponseInvalidRunID,
		})
		return
	}

	if !methodGuard(w, r, constants.HTTPMethodGET, constants.HTTPMethodDELETE) {
		return
	}

	if r.Method == constants.HTTPMethodGET {
		handleGetRun(w, r, svc, id)
	} else {
		handleDeleteRun(w, r, svc, id)
	}
}

// handleGetRun handles GET /runs/{id}
func handleGetRun(w http.ResponseWriter, r *http.Request, svc FlowService, id uuid.UUID) {
	run, err := svc.GetRun(r.Context(), id)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusNotFound,
			Error:      constants.ResponseRunNotFound,
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data:       run,
	})
}

// handleDeleteRun handles DELETE /runs/{id}
func handleDeleteRun(w http.ResponseWriter, r *http.Request, svc FlowService, id uuid.UUID) {
	if err := svc.DeleteRun(r.Context(), id); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      err.Error(),
		})
		return
	}

	writeTextResponse(w, r, http.StatusOK, constants.StatusDeleted)
}

// POST /resume/{token}.
func resumeHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, constants.HTTPMethodPOST) {
		return
	}

	tokenOrID := r.URL.Path[len(constants.PathResume):]

	// Parse the JSON body for event data
	var resumeEvent map[string]any
	if err := decodeJSONRequest(r, &resumeEvent); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      constants.ResponseInvalidRequestBody,
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
		Data:       map[string]any{constants.FieldOutputs: outputs},
	})
}

// handleDirectRunUpdate handles direct run updates for test scenarios
func handleDirectRunUpdate(w http.ResponseWriter, r *http.Request, id uuid.UUID, resumeEvent map[string]any) {
	// Get storage from config
	cfg, err := config.LoadConfig(config.DefaultConfigPath)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      constants.ResponseFailedToLoadConfig,
		})
		return
	}

	store, err := GetStoreFromConfig(cfg)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      constants.ResponseFailedToGetStorage,
		})
		return
	}

	// Get the run directly from storage
	run, err := store.GetRun(r.Context(), id)
	if err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusNotFound,
			Error:      constants.ResponseRunNotFound,
		})
		return
	}

	// Update the event in the run
	run.Event = resumeEvent

	// Save the updated run
	if err := store.SaveRun(r.Context(), run); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusInternalServerError,
			Error:      constants.ResponseFailedToSaveRun,
		})
		return
	}

	// Return response
	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data: map[string]any{
			constants.FieldStatus:  run.Status,
			constants.FieldOutputs: run.Event[constants.FieldOutputs],
		},
	})
}

// GET /graph?flow=<name>.
func graphHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if !methodGuard(w, r, constants.HTTPMethodGET) {
		return
	}

	flowName := r.URL.Query().Get(constants.QueryParamFlow)
	if flowName == "" {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      constants.ResponseMissingFlowParameter,
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
			Error:      constants.ResponseInvalidRequestBody,
		})
		return
	}

	if err := svc.ValidateFlow(r.Context(), req.Flow); err != nil {
		writeResponse(w, r, httpResponse{
			StatusCode: http.StatusBadRequest,
			Error:      fmt.Sprintf(constants.ValidationFailed, err),
		})
		return
	}

	writeResponse(w, r, httpResponse{
		StatusCode: http.StatusOK,
		Data:       map[string]string{constants.FieldStatus: constants.StatusValid},
	})
}

// GET /test (not implemented).
func testHandler(w http.ResponseWriter, _ *http.Request, _ FlowService) {
	w.WriteHeader(http.StatusNotImplemented)
}

func runsInlineHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != constants.HTTPMethodPOST {
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
		if _, err := w.Write([]byte(constants.ResponseInvalidRequestBody)); err != nil {
			utils.Error(constants.LogWriteFailed, err)
		}
		return
	}
	// Parse and validate the flow spec
	flow, err := ParseFlowFromString(req.Spec)
	if err != nil {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(constants.ResponseInvalidFlowSpec + ": " + err.Error())); err != nil {
			utils.Error(constants.LogWriteFailed, err)
		}
		return
	}
	// Start the run inline
	id, outputs, err := svc.RunSpec(r.Context(), flow, req.Event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(constants.ResponseRunError + ": " + err.Error())); err != nil {
			utils.Error(constants.LogWriteFailed, err)
		}
		return
	}
	resp := map[string]any{
		constants.FieldRunID:   id.String(),
		constants.FieldOutputs: outputs,
	}
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		utils.Error(constants.LogJSONEncodeFailed, err)
	}
}

// toolsIndexHandler returns a JSON list of all registered tool manifests from the registry index.json.
func toolsIndexHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != constants.HTTPMethodGET {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tools, err := svc.ListTools(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(constants.ResponseFailedToListTools)); err != nil {
			utils.Error(constants.LogWriteFailed, err)
		}
		return
	}
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		utils.Error(constants.LogJSONEncodeFailed, err)
	}
}

// toolsManifestHandler returns the manifest for a single tool by name from the registry index.json.
func toolsManifestHandler(w http.ResponseWriter, r *http.Request, svc FlowService) {
	if r.Method != constants.HTTPMethodGET {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	nameWithExt := strings.TrimPrefix(r.URL.Path, constants.PathTools)
	name := strings.TrimSuffix(nameWithExt, ".json")
	manifest, err := svc.GetToolManifest(r.Context(), name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(constants.ResponseFailedToGetToolManifest)); err != nil {
			utils.Error(constants.LogWriteFailed, err)
		}
		return
	}
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		utils.Error(constants.LogJSONEncodeFailed, err)
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
		req.APIName = constants.DefaultAPIName // Default name
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
