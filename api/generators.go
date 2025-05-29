package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/awantoch/beemflow/constants"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/utils"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/spf13/cobra"
)

// Global variable to control exit behavior (disabled during tests)
var enableCLIExitCodes = true

// DisableCLIExitCodes disables os.Exit calls for testing
func DisableCLIExitCodes() {
	enableCLIExitCodes = false
}

// EnableCLIExitCodes enables os.Exit calls for production
func EnableCLIExitCodes() {
	enableCLIExitCodes = true
}

// GenerateHTTPHandlers creates HTTP handlers for all operations and registers them
func GenerateHTTPHandlers(mux *http.ServeMux, svc FlowService) {
	// Group operations by path to handle multiple methods on same path
	pathOperations := make(map[string][]*OperationDefinition)

	for _, op := range GetAllOperations() {
		if op.SkipHTTP {
			continue
		}
		pathOperations[op.HTTPPath] = append(pathOperations[op.HTTPPath], op)
	}

	// Register handlers for each unique path
	for path, ops := range pathOperations {
		// Create combined handler for all methods on this path
		handler := generateCombinedHTTPHandler(ops, svc)
		mux.HandleFunc(path, handler)
	}
}

// parseHTTPArgs parses HTTP request into operation arguments
func parseHTTPArgs(r *http.Request, op *OperationDefinition) (any, error) {
	// Create new instance of args type
	args := reflect.New(op.ArgsType).Interface()

	// Handle different HTTP methods
	switch op.HTTPMethod {
	case http.MethodGet:
		return parseGetArgs(r, args, op)
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return parsePostArgs(r, args)
	default:
		return args, nil
	}
}

// parseGetArgs parses GET request arguments from query params and path
func parseGetArgs(r *http.Request, args any, op *OperationDefinition) (any, error) {
	v := reflect.ValueOf(args).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Try to get from path parameters
		if value := extractPathParam(r.URL.Path, op.HTTPPath, fieldType.Name); value != "" {
			if err := setFieldValue(field, value); err != nil {
				return nil, err
			}
			continue
		}

		// Try to get from query parameters
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if value := r.URL.Query().Get(jsonTag); value != "" {
				if err := setFieldValue(field, value); err != nil {
					return nil, err
				}
			}
		}
	}

	return args, nil
}

// parsePostArgs parses POST request arguments from JSON body
func parsePostArgs(r *http.Request, args any) (any, error) {
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(args); err != nil {
			return nil, err
		}
	}
	return args, nil
}

// extractPathParam extracts a parameter from URL path
func extractPathParam(path, pattern, fieldName string) string {
	// Simple path parameter extraction
	// In a real implementation, you'd want more sophisticated path matching
	pathParts := strings.Split(path, "/")
	patternParts := strings.Split(pattern, "/")

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := part[1 : len(part)-1]
			if paramName == strings.ToLower(fieldName) && i < len(pathParts) {
				return pathParts[i]
			}
		}
	}
	return ""
}

// setFieldValue sets a reflect.Value from a string
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.SetInt(i)
		} else {
			return err
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(value); err == nil {
			field.SetBool(b)
		} else {
			return err
		}
	case reflect.Interface:
		// For map[string]any fields, try to parse as JSON
		var data any
		if err := json.Unmarshal([]byte(value), &data); err == nil {
			field.Set(reflect.ValueOf(data))
		} else {
			return err
		}
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}
	return nil
}

// GenerateMCPTools creates MCP tool registrations for all operations
func GenerateMCPTools(svc FlowService) []mcpserver.ToolRegistration {
	var tools []mcpserver.ToolRegistration

	for _, op := range GetAllOperations() {
		if op.SkipMCP {
			continue
		}

		// Create tool registration
		handler := generateMCPHandler(op, svc)
		tools = append(tools, mcpserver.ToolRegistration{
			Name:        op.MCPName,
			Description: op.Description,
			Handler:     handler,
		})
	}

	return tools
}

// generateMCPHandler creates an MCP handler for the given operation
func generateMCPHandler(op *OperationDefinition, svc FlowService) any {
	// Use custom handler if provided
	if op.MCPHandler != nil {
		return op.MCPHandler
	}

	// Create generic handler based on args type
	return func(ctx context.Context, args any) (*mcp.ToolResponse, error) {
		// Convert args to expected type
		convertedArgs, err := convertMCPArgs(args, op.ArgsType)
		if err != nil {
			return nil, err
		}

		// Execute operation
		result, err := op.Handler(ctx, svc, convertedArgs)
		if err != nil {
			return nil, err
		}

		// Convert result to MCP response
		return convertToMCPResponse(result)
	}
}

// convertMCPArgs converts MCP arguments to the expected type
func convertMCPArgs(args any, targetType reflect.Type) (any, error) {
	// If args is already the right type, return as-is
	if reflect.TypeOf(args) == targetType {
		return args, nil
	}

	// Create new instance of target type
	target := reflect.New(targetType).Interface()

	// Convert via JSON marshaling/unmarshaling
	data, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, target); err != nil {
		return nil, err
	}

	return target, nil
}

// convertToMCPResponse converts operation result to MCP response
func convertToMCPResponse(result any) (*mcp.ToolResponse, error) {
	if result == nil {
		return mcp.NewToolResponse(mcp.NewTextContent("success")), nil
	}

	// If result is already a string, return as text
	if str, ok := result.(string); ok {
		return mcp.NewToolResponse(mcp.NewTextContent(str)), nil
	}

	// Otherwise, convert to JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(data))), nil
}

// GenerateCLICommands creates CLI commands for all operations
func GenerateCLICommands(svc FlowService) []*cobra.Command {
	var commands []*cobra.Command

	for _, op := range GetAllOperations() {
		if op.SkipCLI {
			continue
		}

		// Create command
		cmd := generateCLICommand(op, svc)
		commands = append(commands, cmd)
	}

	return commands
}

// generateCLICommand creates a CLI command for the given operation
func generateCLICommand(op *OperationDefinition, svc FlowService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   op.CLIUse,
		Short: op.CLIShort,
		Long:  op.Description,
	}

	// Add flags based on args type
	addCLIFlags(cmd, op.ArgsType)

	// Set run function
	if op.CLIHandler != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			err := op.CLIHandler(cmd, args, svc)
			if err != nil && enableCLIExitCodes {
				// Handle specific error types for exit codes (only when not testing)
				errStr := err.Error()
				switch {
				case strings.Contains(errStr, "YAML parse error"):
					os.Exit(1)
				case strings.Contains(errStr, "schema validation error"):
					os.Exit(2)
				case strings.Contains(errStr, "graph export error"):
					os.Exit(2)
				case strings.Contains(errStr, "failed to write graph"):
					os.Exit(3)
				}
			}
			return err
		}
	} else {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return runGeneratedCLICommand(cmd, args, op, svc)
		}
	}

	// Add special flags for certain operations
	if op.ID == "graphFlow" {
		cmd.Flags().StringP("output", "o", "", "Path to write graph output (defaults to stdout)")
	}

	return cmd
}

// addCLIFlags adds flags to a CLI command based on the args type
func addCLIFlags(cmd *cobra.Command, argsType reflect.Type) {
	// Skip if empty args
	if argsType.Name() == "EmptyArgs" {
		return
	}

	for i := 0; i < argsType.NumField(); i++ {
		field := argsType.Field(i)
		flagTag := field.Tag.Get("flag")
		descTag := field.Tag.Get("description")

		if flagTag == "" || flagTag == "-" {
			continue
		}

		// Add flag based on field type
		switch field.Type.Kind() {
		case reflect.String:
			cmd.Flags().String(flagTag, "", descTag)
		case reflect.Bool:
			cmd.Flags().Bool(flagTag, false, descTag)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			cmd.Flags().Int(flagTag, 0, descTag)
		case reflect.Interface:
			// For map[string]any fields
			cmd.Flags().String(flagTag, "", descTag+" (JSON)")
		}
	}
}

// runGeneratedCLICommand executes a generated CLI command
func runGeneratedCLICommand(cmd *cobra.Command, args []string, op *OperationDefinition, svc FlowService) error {
	// Parse arguments from flags and positional args
	opArgs, err := parseCLIArgs(cmd, args, op.ArgsType)
	if err != nil {
		return err
	}

	// Execute operation
	result, err := op.Handler(cmd.Context(), svc, opArgs)
	if err != nil {
		return err
	}

	// Output result
	return outputCLIResult(result)
}

// parseCLIArgs parses CLI arguments into the expected type
func parseCLIArgs(cmd *cobra.Command, args []string, argsType reflect.Type) (any, error) {
	// Create new instance
	target := reflect.New(argsType).Interface()
	targetVal := reflect.ValueOf(target).Elem()
	targetType := targetVal.Type()

	// Handle positional arguments for simple cases
	if len(args) > 0 && targetType.NumField() > 0 {
		// Set first field from first positional argument
		firstField := targetVal.Field(0)
		if firstField.CanSet() && firstField.Kind() == reflect.String {
			firstField.SetString(args[0])
		}
	}

	// Parse flags
	for i := 0; i < targetType.NumField(); i++ {
		field := targetVal.Field(i)
		fieldType := targetType.Field(i)

		if !field.CanSet() {
			continue
		}

		flagTag := fieldType.Tag.Get("flag")
		if flagTag == "" || flagTag == "-" {
			continue
		}

		// Get flag value and set field
		switch field.Kind() {
		case reflect.String:
			if value, err := cmd.Flags().GetString(flagTag); err == nil && value != "" {
				field.SetString(value)
			} else if err != nil {
				return nil, fmt.Errorf("failed to get string flag %s: %w", flagTag, err)
			}
		case reflect.Bool:
			if value, err := cmd.Flags().GetBool(flagTag); err == nil {
				field.SetBool(value)
			} else {
				return nil, fmt.Errorf("failed to get bool flag %s: %w", flagTag, err)
			}
		case reflect.Int, reflect.Int64:
			if value, err := cmd.Flags().GetInt(flagTag); err == nil && value != 0 {
				field.SetInt(int64(value))
			} else if err != nil {
				return nil, fmt.Errorf("failed to get int flag %s: %w", flagTag, err)
			}
		case reflect.Interface:
			// For map[string]any fields, parse JSON
			if value, err := cmd.Flags().GetString(flagTag); err == nil && value != "" {
				var data any
				if err := json.Unmarshal([]byte(value), &data); err != nil {
					return nil, fmt.Errorf("failed to parse JSON for flag %s: %w", flagTag, err)
				}
				field.Set(reflect.ValueOf(data))
			} else if err != nil {
				return nil, fmt.Errorf("failed to get string flag %s: %w", flagTag, err)
			}
		}
	}

	return target, nil
}

// outputCLIResult outputs the result of a CLI operation
func outputCLIResult(result any) error {
	if result == nil {
		utils.Info("Success")
		return nil
	}

	// If result is a string, output directly
	if str, ok := result.(string); ok {
		utils.User("%s", str)
		return nil
	}

	// Otherwise, output as formatted JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	utils.User("%s", string(data))
	return nil
}

// HandleCLIFileArgs handles CLI commands that can take file input or stdin
func HandleCLIFileArgs(cmd *cobra.Command, args []string, flagName string) ([]byte, error) {
	// Check if there's a positional argument (file path)
	if len(args) > 0 {
		return os.ReadFile(args[0])
	}

	// Check if there's a flag value
	if flagValue, err := cmd.Flags().GetString(flagName); err == nil && flagValue != "" {
		// If it looks like a file path, read it
		if _, err := os.Stat(flagValue); err == nil {
			return os.ReadFile(flagValue)
		}
		// Otherwise, treat as direct content
		return []byte(flagValue), nil
	}

	// Fall back to stdin
	return io.ReadAll(os.Stdin)
}

// generateCombinedHTTPHandler creates a combined HTTP handler for multiple operations on the same path
func generateCombinedHTTPHandler(ops []*OperationDefinition, svc FlowService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Find the operation that matches the HTTP method
		var matchingOp *OperationDefinition
		for _, op := range ops {
			if op.HTTPMethod == r.Method {
				matchingOp = op
				break
			}
		}

		if matchingOp == nil {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Use custom handler if provided
		if matchingOp.HTTPHandler != nil {
			matchingOp.HTTPHandler(w, r, svc)
			return
		}

		// Parse arguments
		args, err := parseHTTPArgs(r, matchingOp)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid arguments: %v", err), http.StatusBadRequest)
			return
		}

		// Execute operation
		result, err := matchingOp.Handler(r.Context(), svc, args)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			utils.Error("Failed to encode response: %v", err)
		}
	}
}

// =====================================================
// UNIFIED INTERFACE FUNCTIONS (consolidated from transport_unified.go)
// =====================================================

// UnifiedAttachHTTPHandlers is the new unified way to attach HTTP handlers
// This replaces the old AttachHTTPHandlers function
func UnifiedAttachHTTPHandlers(mux *http.ServeMux, svc FlowService) {
	// Register system endpoints (health, spec) that don't follow the operation pattern
	registerUnifiedSystemEndpoints(mux)

	// Generate and register all operation handlers
	GenerateHTTPHandlers(mux, svc)
}

// registerUnifiedSystemEndpoints registers system endpoints that are not operations
func registerUnifiedSystemEndpoints(mux *http.ServeMux) {
	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		if _, err := w.Write([]byte(constants.HealthCheckResponse)); err != nil {
			utils.Error(constants.LogFailedWriteHealthCheck, err)
		}
	})

	// Note: Static file serving removed to avoid route conflicts
	// Each application can add their own static file serving as needed
}

// UnifiedBuildMCPToolRegistrations is the new unified way to build MCP tools
// This replaces the old BuildMCPToolRegistrations function
func UnifiedBuildMCPToolRegistrations(svc FlowService) []mcpserver.ToolRegistration {
	return GenerateMCPTools(svc)
}

// UnifiedAttachCLICommands is the new unified way to attach CLI commands
// This provides a simple way to add all generated commands to a root command
func UnifiedAttachCLICommands(root *cobra.Command, svc FlowService) {
	commands := GenerateCLICommands(svc)
	for _, cmd := range commands {
		root.AddCommand(cmd)
	}
}
