package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/docs"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/spf13/cobra"
)

// OperationDefinition defines a single operation with all its metadata and implementation
type OperationDefinition struct {
	ID          string                                                            // Unique identifier
	Name        string                                                            // Display name
	Description string                                                            // Human readable description
	HTTPMethod  string                                                            // HTTP method (GET, POST, etc.)
	HTTPPath    string                                                            // HTTP path pattern
	CLIUse      string                                                            // CLI command usage pattern
	CLIShort    string                                                            // CLI short description
	MCPName     string                                                            // MCP tool name (defaults to ID)
	ArgsType    reflect.Type                                                      // Type for request arguments
	Handler     func(ctx context.Context, svc FlowService, args any) (any, error) // Core implementation
	CLIHandler  func(cmd *cobra.Command, args []string, svc FlowService) error    // Optional custom CLI handler
	HTTPHandler func(w http.ResponseWriter, r *http.Request, svc FlowService)     // Optional custom HTTP handler
	MCPHandler  func(ctx context.Context, args any) (*mcp.ToolResponse, error)    // Optional custom MCP handler
	SkipHTTP    bool                                                              // Skip HTTP interface generation
	SkipMCP     bool                                                              // Skip MCP interface generation
	SkipCLI     bool                                                              // Skip CLI interface generation
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

// init registers all core operations
func init() {
	// List Flows
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDListFlows,
		Name:        "List Flows",
		Description: constants.InterfaceDescListFlows,
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/flows",
		CLIUse:      "list",
		CLIShort:    "List all available flows",
		MCPName:     "beemflow_list_flows",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return svc.ListFlows(ctx)
		},
	})

	// Get Flow
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGetFlow,
		Name:        "Get Flow",
		Description: constants.InterfaceDescGetFlow,
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/flows/{name}",
		CLIUse:      "get <name>",
		CLIShort:    "Get a flow by name",
		MCPName:     "beemflow_get_flow",
		ArgsType:    reflect.TypeOf(GetFlowArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*GetFlowArgs)
			return svc.GetFlow(ctx, a.Name)
		},
	})

	// Validate Flow (Unified: handles both flow names and files)
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDValidateFlow,
		Name:        "Validate Flow",
		Description: "Validate a flow (from name or file)",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/validate",
		CLIUse:      "validate <name_or_file>",
		CLIShort:    "Validate a flow (from name or file)",
		MCPName:     "beemflow_validate_flow",
		ArgsType:    reflect.TypeOf(ValidateFlowArgs{}),
		CLIHandler: func(cmd *cobra.Command, args []string, svc FlowService) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one argument required (flow name or file path)")
			}
			nameOrFile := args[0]

			var err error

			// Try as flow name first, then as file
			if strings.Contains(nameOrFile, ".") || strings.Contains(nameOrFile, "/") {
				// Looks like a file path - parse and validate it directly
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
				// Looks like a flow name - use service
				err = svc.ValidateFlow(cmd.Context(), nameOrFile)
				if err != nil {
					utils.Error("Validation error: %v\n", err)
					return fmt.Errorf("validation error: %w", err)
				}
			}

			utils.User("Validation OK: flow is valid!")
			return nil
		},
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*ValidateFlowArgs)

			// Try as flow name first, then as file
			if strings.Contains(a.Name, ".") || strings.Contains(a.Name, "/") {
				// Looks like a file path - parse and validate it directly
				flow, parseErr := dsl.Parse(a.Name)
				if parseErr != nil {
					return nil, fmt.Errorf("YAML parse error: %w", parseErr)
				}
				err := dsl.Validate(flow)
				if err != nil {
					return nil, fmt.Errorf("schema validation error: %w", err)
				}
			} else {
				// Looks like a flow name - use service
				err := svc.ValidateFlow(ctx, a.Name)
				if err != nil {
					return nil, err
				}
			}

			return map[string]any{"status": "valid", "message": "Validation OK: flow is valid!"}, nil
		},
	})

	// Graph Flow (Unified: handles both flow names and files)
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGraphFlow,
		Name:        "Graph Flow",
		Description: "Generate a Mermaid diagram for a flow (from name or file)",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/flows/graph",
		CLIUse:      "graph <name_or_file>",
		CLIShort:    "Generate a Mermaid diagram for a flow",
		MCPName:     "beemflow_graph_flow",
		ArgsType:    reflect.TypeOf(GraphFlowArgs{}),
		CLIHandler: func(cmd *cobra.Command, args []string, svc FlowService) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one argument required (flow name or file path)")
			}
			nameOrFile := args[0]

			// Get output flag
			outPath, _ := cmd.Flags().GetString("output")

			var diagram string
			var err error

			// Try as flow name first, then as file
			if strings.Contains(nameOrFile, ".") || strings.Contains(nameOrFile, "/") {
				// Looks like a file path - parse it directly
				flow, parseErr := dsl.Parse(nameOrFile)
				if parseErr != nil {
					utils.Error("YAML parse error: %v\n", parseErr)
					return fmt.Errorf("YAML parse error: %w", parseErr)
				}
				diagram, err = graph.ExportMermaid(flow)
			} else {
				// Looks like a flow name - use service
				diagram, err = svc.GraphFlow(cmd.Context(), nameOrFile)
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
		},
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*GraphFlowArgs)

			var diagram string
			var err error

			// Try as flow name first, then as file
			if strings.Contains(a.Name, ".") || strings.Contains(a.Name, "/") {
				// Looks like a file path - parse it directly
				flow, parseErr := dsl.Parse(a.Name)
				if parseErr != nil {
					return nil, fmt.Errorf("YAML parse error: %w", parseErr)
				}
				diagram, err = graph.ExportMermaid(flow)
			} else {
				// Looks like a flow name - use service
				diagram, err = svc.GraphFlow(ctx, a.Name)
			}

			if err != nil {
				return nil, fmt.Errorf("graph export error: %w", err)
			}

			return map[string]any{"diagram": diagram}, nil
		},
	})

	// Start Run
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDStartRun,
		Name:        "Start Run",
		Description: constants.InterfaceDescStartRun,
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/runs",
		CLIUse:      "start <flow-name>",
		CLIShort:    "Start a new flow run",
		MCPName:     "beemflow_start_run",
		ArgsType:    reflect.TypeOf(StartRunArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*StartRunArgs)
			runID, err := svc.StartRun(ctx, a.FlowName, a.Event)
			if err != nil {
				return nil, err
			}
			return map[string]any{"runID": runID.String()}, nil
		},
	})

	// Get Run
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGetRun,
		Name:        "Get Run",
		Description: constants.InterfaceDescGetRun,
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/runs/{id}",
		CLIUse:      "get-run <run-id>",
		CLIShort:    "Get run status and details",
		MCPName:     "beemflow_get_run",
		ArgsType:    reflect.TypeOf(GetRunArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*GetRunArgs)
			runID, err := uuid.Parse(a.RunID)
			if err != nil {
				return nil, fmt.Errorf("invalid run ID: %w", err)
			}
			return svc.GetRun(ctx, runID)
		},
	})

	// List Runs
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDListRuns,
		Name:        "List Runs",
		Description: constants.InterfaceDescListRuns,
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/runs",
		CLIUse:      "list-runs",
		CLIShort:    "List all runs",
		MCPName:     "beemflow_list_runs",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return svc.ListRuns(ctx)
		},
	})

	// Publish Event
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDPublishEvent,
		Name:        "Publish Event",
		Description: constants.InterfaceDescPublishEvent,
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/events",
		CLIUse:      "publish <topic>",
		CLIShort:    "Publish an event to a topic",
		MCPName:     "beemflow_publish_event",
		ArgsType:    reflect.TypeOf(PublishEventArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*PublishEventArgs)
			err := svc.PublishEvent(ctx, a.Topic, a.Payload)
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
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/resume/{token}",
		CLIUse:      "resume <token>",
		CLIShort:    "Resume a paused run",
		MCPName:     "beemflow_resume_run",
		ArgsType:    reflect.TypeOf(ResumeRunArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*ResumeRunArgs)
			return svc.ResumeRun(ctx, a.Token, a.Event)
		},
	})

	// List Tools
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDListTools,
		Name:        "List Tools",
		Description: constants.InterfaceDescListTools,
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/tools",
		CLIUse:      "list-tools",
		CLIShort:    "List all available tools",
		MCPName:     "beemflow_list_tools",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return svc.ListTools(ctx)
		},
	})

	// Get Tool Manifest
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDGetToolManifest,
		Name:        "Get Tool Manifest",
		Description: constants.InterfaceDescGetToolManifest,
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/tools/{name}",
		CLIUse:      "get-tool <name>",
		CLIShort:    "Get tool manifest",
		MCPName:     "beemflow_get_tool_manifest",
		ArgsType:    reflect.TypeOf(GetFlowArgs{}), // Reuse same structure
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			a := args.(*GetFlowArgs)
			return svc.GetToolManifest(ctx, a.Name)
		},
	})

	// Spec
	RegisterOperation(&OperationDefinition{
		ID:          "spec",
		Name:        "Show Specification",
		Description: "Show the BeemFlow protocol & specification",
		HTTPMethod:  http.MethodGet,
		HTTPPath:    "/spec",
		CLIUse:      "spec",
		CLIShort:    "Show the BeemFlow protocol & specification",
		MCPName:     "beemflow_spec",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return docs.BeemflowSpec, nil
		},
	})

	// Lint Flow
	RegisterOperation(&OperationDefinition{
		ID:          "lintFlow",
		Name:        "Lint Flow",
		Description: "Lint a flow file (YAML parse + schema validate)",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/flows/lint",
		CLIUse:      "lint [file]",
		CLIShort:    "Lint a flow file (YAML parse + schema validate)",
		MCPName:     "beemflow_lint_flow",
		ArgsType:    reflect.TypeOf(FlowFileArgs{}),
		CLIHandler: func(cmd *cobra.Command, args []string, svc FlowService) error {
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
		},
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
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
		},
	})

	// Test Flow (stub for now)
	RegisterOperation(&OperationDefinition{
		ID:          "testFlow",
		Name:        "Test Flow",
		Description: "Test a flow file",
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/flows/test",
		CLIUse:      "test",
		CLIShort:    "Test a flow file",
		MCPName:     "beemflow_test_flow",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "flow test (not yet implemented)", nil
		},
	})

	// Convert OpenAPI - The operation that was causing all the duplication!
	RegisterOperation(&OperationDefinition{
		ID:          constants.InterfaceIDConvertOpenAPI,
		Name:        "Convert OpenAPI",
		Description: constants.InterfaceDescConvertOpenAPI,
		HTTPMethod:  http.MethodPost,
		HTTPPath:    "/tools/convert",
		CLIUse:      "convert [openapi_file]",
		CLIShort:    "Convert OpenAPI spec to BeemFlow tool manifests",
		MCPName:     "beemflow_convert_openapi",
		ArgsType:    reflect.TypeOf(ConvertOpenAPIExtendedArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
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
}

// ConvertOpenAPIExtendedArgs includes the output flag for CLI
type ConvertOpenAPIExtendedArgs struct {
	OpenAPI string `json:"openapi" flag:"openapi" description:"OpenAPI spec as JSON string or file path"`
	APIName string `json:"api_name" flag:"api-name" description:"Name prefix for generated tools"`
	BaseURL string `json:"base_url" flag:"base-url" description:"Base URL override"`
	Output  string `json:"-" flag:"output,o" description:"Output file path (default: stdout)"`
}

// ============================================================================
// CUSTOM OPERATION HANDLERS (consolidated from operations_custom.go)
// ============================================================================

// RegisterCustomOperationHandlers registers operations that need custom handling
func RegisterCustomOperationHandlers() {
	// Override the convertOpenAPI operation with custom CLI handler
	if op, exists := GetOperation(constants.InterfaceIDConvertOpenAPI); exists {
		op.CLIHandler = convertOpenAPICLIHandler
	}
}

// convertOpenAPICLIHandler provides custom CLI handling for convertOpenAPI
// This handles file input, stdin, and proper output formatting
func convertOpenAPICLIHandler(cmd *cobra.Command, args []string, svc FlowService) error {
	// Parse flags
	apiName, _ := cmd.Flags().GetString("api-name")
	baseURL, _ := cmd.Flags().GetString("base-url")
	output, _ := cmd.Flags().GetString("output")

	// Set defaults
	if apiName == "" {
		apiName = constants.DefaultAPIName
	}

	// Read OpenAPI data from file, flag, or stdin
	var openapiData []byte
	var err error

	if len(args) > 0 {
		// Read from file argument
		openapiData, err = os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read OpenAPI file: %w", err)
		}
	} else if openapiFlag, _ := cmd.Flags().GetString("openapi"); openapiFlag != "" {
		// Use flag value directly
		openapiData = []byte(openapiFlag)
	} else {
		// Read from stdin
		openapiData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	}

	// Use the core adapter for conversion
	coreAdapter := &adapter.CoreAdapter{}
	inputs := map[string]any{
		"__use":    constants.CoreConvertOpenAPI,
		"openapi":  string(openapiData),
		"api_name": apiName,
		"base_url": baseURL,
	}

	// Execute conversion
	result, err := coreAdapter.Execute(cmd.Context(), inputs)
	if err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	// Output result
	return outputConvertResult(result, output)
}

// outputConvertResult outputs the conversion result to file or stdout
func outputConvertResult(result any, outputPath string) error {
	// Format as JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Output to file or stdout
	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Conversion result written to %s\n", outputPath)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

// init calls the custom handler registration
func init() {
	RegisterCustomOperationHandlers()
}
