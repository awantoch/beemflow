package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/docs"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// OperationDefinition defines a single operation that can be exposed as HTTP, CLI, or MCP.
type OperationDefinition struct {
	ID          string                                           // Unique identifier
	Name        string                                           // Human-readable name
	Description string                                           // Description for help/docs
	Group       string                                           // Logical group (flows, runs, events, tools, system)
	HTTPMethod  string                                           // HTTP method (GET, POST, etc.)
	HTTPPath    string                                           // HTTP path pattern (/flows/{id})
	CLIUse      string                                           // CLI usage pattern (get <name>)
	CLIShort    string                                           // CLI short description
	MCPName     string                                           // MCP tool name
	ArgsType    reflect.Type                                     // Type for operation arguments
	Handler     func(ctx context.Context, args any) (any, error) // Core implementation
	CLIHandler  func(cmd *cobra.Command, args []string) error    // Optional custom CLI handler
	HTTPHandler func(w http.ResponseWriter, r *http.Request)     // Optional custom HTTP handler
	MCPHandler  any                                              // Optional custom MCP handler
	SkipHTTP    bool                                             // Skip HTTP generation
	SkipCLI     bool                                             // Skip CLI generation
	SkipMCP     bool                                             // Skip MCP generation
}

// ArgumentTypes for common operations
type EmptyArgs struct{}

type GetFlowArgs struct {
	Name string `json:"name" flag:"name" description:"Flow name"`
}

type ValidateFlowArgs struct {
	Name string `json:"name" flag:"name" description:"Flow name or file path to validate"`
}

type GraphFlowArgs struct {
	Name string `json:"name" flag:"name" description:"Flow name or file path to graph"`
}

type StartRunArgs struct {
	FlowName string         `json:"flowName" flag:"flow-name" description:"Name of the flow to run"`
	Event    map[string]any `json:"event" flag:"event-json" description:"Event data as JSON"`
}

type GetRunArgs struct {
	RunID string `json:"runID" flag:"run-id" description:"Run ID"`
}

type PublishEventArgs struct {
	Topic   string         `json:"topic" flag:"topic" description:"Event topic"`
	Payload map[string]any `json:"payload" flag:"payload-json" description:"Event payload as JSON"`
}

type ResumeRunArgs struct {
	Token string         `json:"token" flag:"token" description:"Resume token"`
	Event map[string]any `json:"event" flag:"event-json" description:"Event data as JSON"`
}

type ConvertOpenAPIArgs struct {
	OpenAPI string `json:"openapi" flag:"openapi" description:"OpenAPI spec as JSON string or file path"`
	APIName string `json:"api_name" flag:"api-name" description:"Name prefix for generated tools"`
	BaseURL string `json:"base_url" flag:"base-url" description:"Base URL override"`
}

// FlowFileArgs represents arguments for flow file operations
type FlowFileArgs struct {
	File string `json:"file" flag:"file,f" description:"Path to flow file"`
}

// SearchArgs represents arguments for search operations
type SearchArgs struct {
	Query string `json:"query" flag:"query" description:"Search query"`
}

// Global operation registry
var operationRegistry = make(map[string]*OperationDefinition)

// RegisterOperation registers an operation definition
func RegisterOperation(op *OperationDefinition) {
	if op.MCPName == "" {
		op.MCPName = op.ID
	}
	operationRegistry[op.ID] = op
}

// GetOperation retrieves an operation by ID
func GetOperation(id string) (*OperationDefinition, bool) {
	op, exists := operationRegistry[id]
	return op, exists
}

// GetAllOperations returns all registered operations
func GetAllOperations() map[string]*OperationDefinition {
	return operationRegistry
}

// GetOperationsByGroups returns operations filtered by the specified groups
func GetOperationsByGroups(groups []string) []*OperationDefinition {
	if len(groups) == 0 {
		// Return all operations as slice
		result := make([]*OperationDefinition, 0, len(operationRegistry))
		for _, op := range operationRegistry {
			result = append(result, op)
		}
		return result
	}

	groupSet := make(map[string]bool)
	for _, group := range groups {
		groupSet[strings.TrimSpace(group)] = true
	}

	var filtered []*OperationDefinition
	for _, op := range operationRegistry {
		if groupSet[op.Group] {
			filtered = append(filtered, op)
		}
	}
	return filtered
}

// GetOperationsMapByGroups returns operations filtered by the specified groups as a map
func GetOperationsMapByGroups(groups []string) map[string]*OperationDefinition {
	if len(groups) == 0 {
		return operationRegistry
	}

	groupSet := make(map[string]bool)
	for _, group := range groups {
		groupSet[strings.TrimSpace(group)] = true
	}

	filtered := make(map[string]*OperationDefinition)
	for id, op := range operationRegistry {
		if groupSet[op.Group] {
			filtered[id] = op
		}
	}
	return filtered
}

// looksLikeFilePath determines if a string looks like a file path vs a flow name
func looksLikeFilePath(nameOrFile string) bool {
	// Check if it has a common file extension
	ext := filepath.Ext(nameOrFile)
	if ext == ".yaml" || ext == ".yml" || ext == ".json" {
		return true
	}

	// Check if it exists as a file
	if _, err := os.Stat(nameOrFile); err == nil {
		return true
	}

	// Check if it contains path separators
	return strings.Contains(nameOrFile, "/") || strings.Contains(nameOrFile, "\\")
}

// Handler functions to reduce cyclomatic complexity of init()
func validateFlowCLIHandler(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument required (flow name or file path)")
	}
	nameOrFile := args[0]

	var err error
	if looksLikeFilePath(nameOrFile) {
		// Parse and validate file directly
		flow, parseErr := dsl.Parse(nameOrFile)
		if parseErr != nil {
			utils.Error("YAML parse error: %v\n", parseErr)
			return fmt.Errorf("YAML parse error: %w", parseErr)
		}
		err = dsl.Validate(flow)
		if err != nil {
			utils.Error("Schema validation error: %v\n", err)
			return fmt.Errorf("schema validation error: %w", err)
		}
	} else {
		// Use flow name service
		err = ValidateFlow(cmd.Context(), nameOrFile)
		if err != nil {
			utils.Error("Validation error: %v\n", err)
			return fmt.Errorf("validation error: %w", err)
		}
	}

	utils.User("Validation OK: flow is valid!")
	return nil
}

func validateFlowHandler(ctx context.Context, args any) (any, error) {
	a := args.(*ValidateFlowArgs)
	err := ValidateFlow(ctx, a.Name)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "valid", "message": "Validation OK: flow is valid!"}, nil
}

func graphFlowCLIHandler(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument required (flow name or file path)")
	}
	nameOrFile := args[0]

	// Get output flag
	outPath, _ := cmd.Flags().GetString("output")

	var diagram string
	var err error

	if looksLikeFilePath(nameOrFile) {
		// Parse file directly and generate diagram
		flow, parseErr := dsl.Parse(nameOrFile)
		if parseErr != nil {
			utils.Error("YAML parse error: %v\n", parseErr)
			return fmt.Errorf("YAML parse error: %w", parseErr)
		}
		diagram, err = graph.ExportMermaid(flow)
	} else {
		// Use flow name service
		diagram, err = GraphFlow(cmd.Context(), nameOrFile)
	}

	if err != nil {
		utils.Error("Graph export error: %v\n", err)
		return fmt.Errorf("graph export error: %w", err)
	}

	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(diagram), 0644); err != nil {
			utils.Error("Failed to write graph to %s: %v\n", outPath, err)
			return fmt.Errorf("failed to write graph to %s: %w", outPath, err)
		}
		utils.User("Graph written to %s", outPath)
	} else {
		utils.Info("%s", diagram)
	}
	return nil
}

func graphFlowHandler(ctx context.Context, args any) (any, error) {
	a := args.(*GraphFlowArgs)
	diagram, err := GraphFlow(ctx, a.Name)
	if err != nil {
		return nil, err
	}
	return map[string]any{"diagram": diagram}, nil
}

func lintFlowCLIHandler(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one file argument required")
	}
	file := args[0]
	flow, err := dsl.Parse(file)
	if err != nil {
		utils.Error("YAML parse error: %v\n", err)
		return fmt.Errorf("YAML parse error: %w", err)
	}
	err = dsl.Validate(flow)
	if err != nil {
		utils.Error("Schema validation error: %v\n", err)
		return fmt.Errorf("schema validation error: %w", err)
	}
	utils.User("Lint OK: flow is valid!")
	return nil
}

func lintFlowHandler(ctx context.Context, args any) (any, error) {
	a := args.(*FlowFileArgs)
	flow, err := dsl.Parse(a.File)
	if err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}
	err = dsl.Validate(flow)
	if err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}
	return map[string]any{"status": "valid", "message": "Lint OK: flow is valid!"}, nil
}

// init registers all core operations
func init() {
	// List Flows
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDListFlows,
		Name:        "List Flows",
		Description: constants.InterfaceDescListFlows,
		Group:       "flows",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/flows",
		CLIUse:      "flows list",
		CLIShort:    "List all available flows",
		MCPName:     "beemflow_list_flows",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			return ListFlows(ctx)
		},
	})

	// Get Flow
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGetFlow,
		Name:        "Get Flow",
		Description: constants.InterfaceDescGetFlow,
		Group:       "flows",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/flows/{name}",
		CLIUse:      "flows get <name>",
		CLIShort:    "Get a flow by name",
		MCPName:     "beemflow_get_flow",
		ArgsType:    reflect.TypeOf(GetFlowArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*GetFlowArgs)
			return GetFlow(ctx, a.Name)
		},
	})

	// Validate Flow (Unified: handles both flow names and files)
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDValidateFlow,
		Name:        "Validate Flow",
		Description: "Validate a flow (from name or file)",
		Group:       "flows",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/validate",
		CLIUse:      "flows validate <name_or_file>",
		CLIShort:    "Validate a flow (from name or file)",
		MCPName:     "beemflow_validate_flow",
		ArgsType:    reflect.TypeOf(ValidateFlowArgs{}),
		CLIHandler:  validateFlowCLIHandler,
		Handler:     validateFlowHandler,
	})

	// Graph Flow (Unified: handles both flow names and files)
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGraphFlow,
		Name:        "Graph Flow",
		Description: "Generate a Mermaid diagram for a flow (from name or file)",
		Group:       "flows",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/flows/graph",
		CLIUse:      "flows graph <name_or_file>",
		CLIShort:    "Generate a Mermaid diagram for a flow",
		MCPName:     "beemflow_graph_flow",
		ArgsType:    reflect.TypeOf(GraphFlowArgs{}),
		CLIHandler:  graphFlowCLIHandler,
		Handler:     graphFlowHandler,
	})

	// Start Run
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDStartRun,
		Name:        "Start Run",
		Description: constants.InterfaceDescStartRun,
		Group:       "runs",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/runs",
		CLIUse:      "runs start <flow-name>",
		CLIShort:    "Start a new flow run by name",
		MCPName:     "beemflow_start_run",
		ArgsType:    reflect.TypeOf(StartRunArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*StartRunArgs)
			return StartRun(ctx, a.FlowName, a.Event)
		},
	})

	// Get Run
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGetRun,
		Name:        "Get Run",
		Description: constants.InterfaceDescGetRun,
		Group:       "runs",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/runs/{id}",
		CLIUse:      "runs get <run-id>",
		CLIShort:    "Get run status and details",
		MCPName:     "beemflow_get_run",
		ArgsType:    reflect.TypeOf(GetRunArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*GetRunArgs)
			runID, err := uuid.Parse(a.RunID)
			if err != nil {
				return nil, fmt.Errorf("invalid run ID: %w", err)
			}
			return GetRun(ctx, runID)
		},
	})

	// List Runs
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDListRuns,
		Name:        "List Runs",
		Description: constants.InterfaceDescListRuns,
		Group:       "runs",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/runs",
		CLIUse:      "runs list",
		CLIShort:    "List all runs",
		MCPName:     "beemflow_list_runs",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			return ListRuns(ctx)
		},
	})

	// Publish Event
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDPublishEvent,
		Name:        "Publish Event",
		Description: constants.InterfaceDescPublishEvent,
		Group:       "events",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/events",
		CLIUse:      "publish <topic>",
		CLIShort:    "Publish an event to a topic",
		MCPName:     "beemflow_publish_event",
		ArgsType:    reflect.TypeOf(PublishEventArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*PublishEventArgs)
			err := PublishEvent(ctx, a.Topic, a.Payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"status": "published"}, nil
		},
	})

	// Resume Run
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDResumeRun,
		Name:        "Resume Run",
		Description: constants.InterfaceDescResumeRun,
		Group:       "runs",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/resume/{token}",
		CLIUse:      "resume <token>",
		CLIShort:    "Resume a paused run",
		MCPName:     "beemflow_resume_run",
		ArgsType:    reflect.TypeOf(ResumeRunArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*ResumeRunArgs)
			return ResumeRun(ctx, a.Token, a.Event)
		},
	})

	// Spec
	RegisterOperation(&OperationDefinition{
		ID:          "spec",
		Name:        "Show Specification",
		Description: "Show the BeemFlow protocol & specification",
		Group:       "system",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/spec",
		CLIUse:      "spec",
		CLIShort:    "Show the BeemFlow protocol & specification",
		MCPName:     "beemflow_spec",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			return docs.BeemflowSpec, nil
		},
	})

	// Lint Flow
	RegisterOperation(&OperationDefinition{
		ID:          "lintFlow",
		Name:        "Lint Flow",
		Description: "Lint a flow file (YAML parse + schema validate)",
		Group:       "flows",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/flows/lint",
		CLIUse:      "lint [file]",
		CLIShort:    "Lint a flow file (YAML parse + schema validate)",
		MCPName:     "beemflow_lint_flow",
		ArgsType:    reflect.TypeOf(FlowFileArgs{}),
		CLIHandler:  lintFlowCLIHandler,
		Handler:     lintFlowHandler,
	})

	// Test Flow
	RegisterOperation(&OperationDefinition{
		ID:          "testFlow",
		Name:        "Test Flow",
		Description: "Test a flow file",
		Group:       "flows",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/flows/test",
		CLIUse:      "test",
		CLIShort:    "Test a flow file",
		MCPName:     "beemflow_test_flow",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			// For now, test just validates the flow - could be expanded to run tests
			return map[string]any{
				"status":  "success",
				"message": "Test functionality not implemented yet",
			}, nil
		},
	})

	// Convert OpenAPI
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDConvertOpenAPI,
		Name:        "Convert OpenAPI",
		Description: constants.InterfaceDescConvertOpenAPI,
		Group:       "tools",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/tools/convert",
		CLIUse:      "convert [openapi_file]",
		CLIShort:    "Convert OpenAPI spec to BeemFlow tool manifests",
		MCPName:     "beemflow_convert_openapi",
		ArgsType:    reflect.TypeOf(ConvertOpenAPIExtendedArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*ConvertOpenAPIExtendedArgs)
			// Use the core adapter for conversion
			coreAdapter := &adapter.CoreAdapter{}
			inputs := map[string]any{
				"__use":    constants.CoreConvertOpenAPI,
				"openapi":  a.OpenAPI,
				"api_name": a.APIName,
				"base_url": a.BaseURL,
			}

			// Set defaults
			if inputs["api_name"] == "" {
				inputs["api_name"] = constants.DefaultAPIName
			}

			return coreAdapter.Execute(ctx, inputs)
		},
	})

	// === FEDERATION APIS (Now unified - can be used via HTTP, CLI, and MCP) ===

	// List Tools
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDListTools,
		Name:        "List Tools",
		Description: constants.InterfaceDescListTools,
		Group:       "tools",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/tools",
		CLIUse:      "tools list",
		CLIShort:    "List all tools from registries",
		MCPName:     "beemflow_list_tools",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			return ListToolManifests(ctx)
		},
	})

	// Get Tool Manifest
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGetToolManifest,
		Name:        "Get Tool Manifest",
		Description: constants.InterfaceDescGetToolManifest,
		Group:       "tools",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/tools/{name}",
		CLIUse:      "tools get <name>",
		CLIShort:    "Get a tool manifest by name",
		MCPName:     "beemflow_get_tool_manifest",
		ArgsType:    reflect.TypeOf(GetFlowArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			a := args.(*GetFlowArgs)
			return GetToolManifest(ctx, a.Name)
		},
	})

	// Registry Index
	RegisterOperation(&OperationDefinition{
		ID:          "registry_index",
		Name:        "Registry Index",
		Description: "Get the complete registry index for federation",
		Group:       "system",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/registry",
		CLIUse:      "registry",
		CLIShort:    "Show complete registry index",
		MCPName:     "beemflow_registry_index",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			return GetRegistryIndex(ctx)
		},
	})

	// System Cron - Simple endpoint for triggering scheduled workflows
	RegisterOperation(&OperationDefinition{
		ID:          "system_cron",
		Name:        "System Cron Trigger",
		Description: "Triggers all workflows with schedule.cron (called by Vercel or system cron)",
		Group:       "system",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/cron",
		SkipCLI:     true,
		SkipMCP:     true,
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		HTTPHandler: func(w http.ResponseWriter, r *http.Request) {
			// Verify CRON_SECRET if set (Vercel security)
			if secret := os.Getenv("CRON_SECRET"); secret != "" {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer "+secret {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
			
			ctx := r.Context()
			triggeredWorkflows := []string{}
			
			// List all workflows
			flows, err := ListFlows(ctx)
			if err != nil {
				utils.Error("Failed to list flows: %v", err)
				http.Error(w, "Failed to list workflows", http.StatusInternalServerError)
				return
			}
			
			// Early exit if no workflows
			if len(flows) == 0 {
				response := map[string]interface{}{
					"status":    "completed",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"triggered": 0,
					"workflows": []string{},
					"results":   map[string]string{},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
			
			// Trigger each workflow that has schedule.cron
			for _, flowName := range flows {
				flow, err := GetFlow(ctx, flowName)
				if err != nil {
					continue
				}
				
				// Check if workflow has schedule.cron trigger
				hasCron := false
				switch on := flow.On.(type) {
				case string:
					hasCron = (on == "schedule.cron")
				case []interface{}:
					for _, trigger := range on {
						if str, ok := trigger.(string); ok && str == "schedule.cron" {
							hasCron = true
							break
						}
					}
				}
				
				if !hasCron {
					continue
				}
				
				// Trigger the workflow
				event := map[string]interface{}{
					"trigger":   "schedule.cron",
					"workflow":  flowName,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}
				
				if _, err := StartRun(ctx, flowName, event); err != nil {
					utils.Error("Failed to trigger %s: %v", flowName, err)
				} else {
					triggeredWorkflows = append(triggeredWorkflows, flowName)
				}
			}
			
			// Response for compatibility
			response := map[string]interface{}{
				"status":    "completed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"triggered": len(triggeredWorkflows),
				"workflows": triggeredWorkflows,
				"results":   map[string]string{}, // For backward compatibility
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		},
	})

	// Per-workflow cron endpoint for more precise control
	RegisterOperation(&OperationDefinition{
		ID:          "workflow_cron",
		Name:        "Workflow Cron Trigger",
		Description: "Triggers a specific workflow (called by system cron)",
		Group:       "system",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/cron/{workflow}",
		SkipCLI:     true,
		SkipMCP:     true,
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		HTTPHandler: func(w http.ResponseWriter, r *http.Request) {
			// Verify CRON_SECRET if set (Vercel security)
			if secret := os.Getenv("CRON_SECRET"); secret != "" {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer "+secret {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			}
			
			ctx := r.Context()
			
			// Extract workflow name from path safely
			// First check for any path traversal attempts in the original path
			if strings.Contains(r.URL.Path, "..") {
				http.Error(w, "Invalid workflow name", http.StatusBadRequest)
				return
			}
			
			cleanPath := path.Clean(r.URL.Path)
			
			// Ensure the path starts with /cron/
			if !strings.HasPrefix(cleanPath, "/cron/") {
				http.Error(w, "Invalid path", http.StatusBadRequest)
				return
			}
			
			// Extract workflow name - everything after /cron/
			workflowName := strings.TrimPrefix(cleanPath, "/cron/")
			
			// Additional validation
			if workflowName == "" || workflowName == "." || workflowName == "/" || 
			   strings.ContainsAny(workflowName, "/\\") {
				http.Error(w, "Invalid workflow name", http.StatusBadRequest)
				return
			}
			
			// Verify workflow exists
			flow, err := GetFlow(ctx, workflowName)
			if err != nil {
				http.Error(w, "Workflow not found", http.StatusNotFound)
				return
			}
			
			// Check if it has schedule.cron trigger
			hasCron := false
			switch on := flow.On.(type) {
			case string:
				hasCron = (on == "schedule.cron")
			case []interface{}:
				for _, trigger := range on {
					if str, ok := trigger.(string); ok && str == "schedule.cron" {
						hasCron = true
						break
					}
				}
			}
			
			if !hasCron {
				http.Error(w, "Workflow does not have schedule.cron trigger", http.StatusBadRequest)
				return
			}
			
			// Trigger the workflow
			event := map[string]interface{}{
				"trigger":   "schedule.cron",
				"workflow":  workflowName,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			
			runID, err := StartRun(ctx, workflowName, event)
			if err != nil {
				utils.Error("Failed to trigger %s: %v", workflowName, err)
				http.Error(w, "Failed to trigger workflow", http.StatusInternalServerError)
				return
			}
			
			response := map[string]interface{}{
				"status":   "triggered",
				"workflow": workflowName,
				"run_id":   runID.String(),
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		},
	})

	// === MANAGEMENT APIS (Simplified - no custom CLI handlers needed) ===

	// NOTE: Some management APIs have been simplified to avoid CLI duplication.
	// Tools search/install and registry stats can be added later if needed.

	// Root Greeting
	RegisterOperation(&OperationDefinition{
		ID:          "root",
		Name:        "Root Greeting",
		Description: "Simple greeting at the API root path",
		Group:       "system",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/",
		CLIUse:      "",
		CLIShort:    "",
		SkipCLI:     true,
		SkipMCP:     true,
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, args any) (any, error) {
			return "Hi, I'm BeemBeem! :D", nil
		},
		HTTPHandler: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hi, I'm BeemBeem! :D"))
		},
	})
}

// ConvertOpenAPIExtendedArgs includes the output flag for CLI
type ConvertOpenAPIExtendedArgs struct {
	OpenAPI string `json:"openapi" flag:"openapi" description:"OpenAPI spec as JSON string or file path"`
	APIName string `json:"api_name" flag:"api-name" description:"Name prefix for generated tools"`
	BaseURL string `json:"base_url" flag:"base-url" description:"Base URL override"`
	Output  string `json:"-" flag:"output,o" description:"Output file path (default: stdout)"`
}

// ============================================================================
// END OF FILE
// ============================================================================
