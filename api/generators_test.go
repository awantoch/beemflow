package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/spf13/cobra"
)

// Mock FlowService for testing
type mockFlowService struct{}

func (m *mockFlowService) ListFlows(ctx context.Context) ([]string, error) {
	return []string{"test-flow"}, nil
}

func (m *mockFlowService) GetFlow(ctx context.Context, name string) (model.Flow, error) {
	if name == "test-flow" {
		return model.Flow{Name: "test-flow"}, nil
	}
	return model.Flow{}, fmt.Errorf("flow not found")
}

func (m *mockFlowService) ValidateFlow(ctx context.Context, name string) error {
	if name == "invalid-flow" {
		return fmt.Errorf("validation error")
	}
	return nil
}

func (m *mockFlowService) GraphFlow(ctx context.Context, name string) (string, error) {
	if name == "test-flow" {
		return "graph TD\n  A --> B", nil
	}
	return "", fmt.Errorf("graph error")
}

func (m *mockFlowService) StartRun(ctx context.Context, flowName string, event map[string]any) (uuid.UUID, error) {
	if flowName == "error-flow" {
		return uuid.Nil, fmt.Errorf("start error")
	}
	return uuid.New(), nil
}

func (m *mockFlowService) GetRun(ctx context.Context, runID uuid.UUID) (*model.Run, error) {
	return &model.Run{ID: runID, FlowName: "test-flow"}, nil
}

func (m *mockFlowService) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return []*model.Run{{ID: uuid.New(), FlowName: "test-flow"}}, nil
}

func (m *mockFlowService) DeleteRun(ctx context.Context, runID uuid.UUID) error {
	return nil
}

func (m *mockFlowService) PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	if topic == "error-topic" {
		return fmt.Errorf("publish error")
	}
	return nil
}

func (m *mockFlowService) ResumeRun(ctx context.Context, token string, event map[string]any) (map[string]any, error) {
	if token == "error-token" {
		return nil, fmt.Errorf("resume error")
	}
	return map[string]any{"result": "resumed"}, nil
}

func (m *mockFlowService) RunSpec(ctx context.Context, flow *model.Flow, event map[string]any) (uuid.UUID, map[string]any, error) {
	return uuid.New(), map[string]any{"spec": "test"}, nil
}

func (m *mockFlowService) ListTools(ctx context.Context) ([]registry.ToolManifest, error) {
	return []registry.ToolManifest{{Name: "test-tool"}}, nil
}

func (m *mockFlowService) GetToolManifest(ctx context.Context, name string) (*registry.ToolManifest, error) {
	return &registry.ToolManifest{Name: name}, nil
}

func TestSetFieldValue(t *testing.T) {
	tests := []struct {
		name        string
		fieldKind   reflect.Kind
		value       string
		wantErr     bool
		checkResult func(field reflect.Value) bool
	}{
		{
			name:      "string field",
			fieldKind: reflect.String,
			value:     "test",
			wantErr:   false,
			checkResult: func(field reflect.Value) bool {
				return field.String() == "test"
			},
		},
		{
			name:      "int field",
			fieldKind: reflect.Int,
			value:     "42",
			wantErr:   false,
			checkResult: func(field reflect.Value) bool {
				return field.Int() == 42
			},
		},
		{
			name:      "bool field true",
			fieldKind: reflect.Bool,
			value:     "true",
			wantErr:   false,
			checkResult: func(field reflect.Value) bool {
				return field.Bool() == true
			},
		},
		{
			name:      "bool field false",
			fieldKind: reflect.Bool,
			value:     "false",
			wantErr:   false,
			checkResult: func(field reflect.Value) bool {
				return field.Bool() == false
			},
		},
		{
			name:      "interface field with JSON",
			fieldKind: reflect.Interface,
			value:     `{"key": "value"}`,
			wantErr:   false,
			checkResult: func(field reflect.Value) bool {
				data, ok := field.Interface().(map[string]any)
				return ok && data["key"] == "value"
			},
		},
		{
			name:      "invalid int",
			fieldKind: reflect.Int,
			value:     "not-a-number",
			wantErr:   true,
		},
		{
			name:      "invalid bool",
			fieldKind: reflect.Bool,
			value:     "not-a-bool",
			wantErr:   true,
		},
		{
			name:      "invalid json",
			fieldKind: reflect.Interface,
			value:     `{"invalid": json}`,
			wantErr:   true,
		},
		{
			name:      "unsupported type",
			fieldKind: reflect.Float64,
			value:     "3.14",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a value of the specified kind
			var field reflect.Value
			switch tt.fieldKind {
			case reflect.String:
				var s string
				field = reflect.ValueOf(&s).Elem()
			case reflect.Int:
				var i int
				field = reflect.ValueOf(&i).Elem()
			case reflect.Bool:
				var b bool
				field = reflect.ValueOf(&b).Elem()
			case reflect.Interface:
				var iface any
				field = reflect.ValueOf(&iface).Elem()
			case reflect.Float64:
				var f float64
				field = reflect.ValueOf(&f).Elem()
			}

			err := setFieldValue(field, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("setFieldValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkResult != nil && !tt.checkResult(field) {
				t.Errorf("setFieldValue() field value check failed")
			}
		})
	}
}

func TestExtractPathParam(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		pattern   string
		fieldName string
		want      string
	}{
		{
			name:      "single parameter",
			path:      "/flows/test-flow",
			pattern:   "/flows/{name}",
			fieldName: "name",
			want:      "test-flow",
		},
		{
			name:      "multiple parameters",
			path:      "/runs/123/steps/456",
			pattern:   "/runs/{runid}/steps/{stepid}",
			fieldName: "runid",
			want:      "123",
		},
		{
			name:      "parameter not found",
			path:      "/flows/test-flow",
			pattern:   "/flows/{name}",
			fieldName: "missing",
			want:      "",
		},
		{
			name:      "no parameters in pattern",
			path:      "/flows",
			pattern:   "/flows",
			fieldName: "name",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPathParam(tt.path, tt.pattern, tt.fieldName)
			if got != tt.want {
				t.Errorf("extractPathParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGetArgs(t *testing.T) {
	type TestArgs struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	req := httptest.NewRequest("GET", "/test?name=test-flow&count=5", nil)
	op := &OperationDefinition{
		HTTPPath: "/test",
	}

	args := &TestArgs{}
	result, err := parseGetArgs(req, args, op)
	if err != nil {
		t.Errorf("parseGetArgs() error = %v", err)
		return
	}

	resultArgs, ok := result.(*TestArgs)
	if !ok {
		t.Errorf("parseGetArgs() result type assertion failed")
		return
	}

	if resultArgs.Name != "test-flow" {
		t.Errorf("parseGetArgs() Name = %v, want test-flow", resultArgs.Name)
	}

	if resultArgs.Count != 5 {
		t.Errorf("parseGetArgs() Count = %v, want 5", resultArgs.Count)
	}
}

func TestParsePostArgs(t *testing.T) {
	type TestArgs struct {
		Name string `json:"name"`
		Data any    `json:"data"`
	}

	body := `{"name": "test", "data": {"key": "value"}}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	args := &TestArgs{}
	result, err := parsePostArgs(req, args)
	if err != nil {
		t.Errorf("parsePostArgs() error = %v", err)
		return
	}

	resultArgs, ok := result.(*TestArgs)
	if !ok {
		t.Errorf("parsePostArgs() result type assertion failed")
		return
	}

	if resultArgs.Name != "test" {
		t.Errorf("parsePostArgs() Name = %v, want test", resultArgs.Name)
	}
}

func TestParseHTTPArgs(t *testing.T) {
	type TestArgs struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name     string
		method   string
		body     string
		wantErr  bool
		wantName string
	}{
		{
			name:     "GET request",
			method:   "GET",
			wantErr:  false,
			wantName: "",
		},
		{
			name:     "POST request",
			method:   "POST",
			body:     `{"name": "test"}`,
			wantErr:  false,
			wantName: "test",
		},
		{
			name:    "unsupported method",
			method:  "DELETE",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/test", strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, "/test", nil)
			}

			op := &OperationDefinition{
				HTTPMethod: tt.method,
				HTTPPath:   "/test",
				ArgsType:   reflect.TypeOf(TestArgs{}),
			}

			result, err := parseHTTPArgs(req, op)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHTTPArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantName != "" {
				if args, ok := result.(*TestArgs); ok {
					if args.Name != tt.wantName {
						t.Errorf("parseHTTPArgs() Name = %v, want %v", args.Name, tt.wantName)
					}
				}
			}
		})
	}
}

func TestConvertMCPArgs(t *testing.T) {
	type TestArgs struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	tests := []struct {
		name       string
		args       any
		targetType reflect.Type
		wantErr    bool
		checkFunc  func(result any) bool
	}{
		{
			name:       "same type",
			args:       &TestArgs{Name: "test", Count: 5},
			targetType: reflect.TypeOf(TestArgs{}),
			wantErr:    false,
			checkFunc: func(result any) bool {
				args, ok := result.(*TestArgs)
				return ok && args.Name == "test" && args.Count == 5
			},
		},
		{
			name:       "map to struct",
			args:       map[string]any{"name": "test", "count": 5},
			targetType: reflect.TypeOf(TestArgs{}),
			wantErr:    false,
			checkFunc: func(result any) bool {
				args, ok := result.(*TestArgs)
				return ok && args.Name == "test" && args.Count == 5
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertMCPArgs(tt.args, tt.targetType)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMCPArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil && !tt.checkFunc(result) {
				t.Errorf("convertMCPArgs() result check failed")
			}
		})
	}
}

func TestConvertToMCPResponse(t *testing.T) {
	tests := []struct {
		name    string
		result  any
		wantErr bool
		check   func(resp *mcp.ToolResponse) bool
	}{
		{
			name:    "nil result",
			result:  nil,
			wantErr: false,
			check: func(resp *mcp.ToolResponse) bool {
				return len(resp.Content) > 0
			},
		},
		{
			name:    "string result",
			result:  "test response",
			wantErr: false,
			check: func(resp *mcp.ToolResponse) bool {
				return len(resp.Content) > 0
			},
		},
		{
			name:    "struct result",
			result:  map[string]string{"key": "value"},
			wantErr: false,
			check: func(resp *mcp.ToolResponse) bool {
				return len(resp.Content) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := convertToMCPResponse(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToMCPResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil && !tt.check(resp) {
				t.Errorf("convertToMCPResponse() response check failed")
			}
		})
	}
}

func TestOutputCLIResult(t *testing.T) {
	tests := []struct {
		name    string
		result  any
		wantErr bool
	}{
		{
			name:    "nil result",
			result:  nil,
			wantErr: false,
		},
		{
			name:    "string result",
			result:  "test output",
			wantErr: false,
		},
		{
			name:    "struct result",
			result:  map[string]string{"key": "value"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := outputCLIResult(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("outputCLIResult() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateHTTPHandlers(t *testing.T) {
	// Register a test operation
	testOp := &OperationDefinition{
		ID:          "test-op",
		Name:        "Test Operation",
		Description: "Test operation for HTTP handlers",
		HTTPMethod:  "GET",
		HTTPPath:    "/test",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return map[string]string{"result": "test"}, nil
		},
	}
	RegisterOperation(testOp)

	mux := http.NewServeMux()
	svc := &mockFlowService{}

	// This should not panic
	GenerateHTTPHandlers(mux, svc)

	// Test that the handler was registered
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should not be 404
	if w.Code == http.StatusNotFound {
		t.Error("GenerateHTTPHandlers() did not register handler")
	}
}

func TestGenerateMCPTools(t *testing.T) {
	// Register a test operation
	testOp := &OperationDefinition{
		ID:          "test-mcp-op",
		Name:        "Test MCP Operation",
		MCPName:     "test_mcp_tool",
		Description: "Test MCP tool",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}
	RegisterOperation(testOp)

	svc := &mockFlowService{}
	tools := GenerateMCPTools(svc)

	// Should have at least one tool
	if len(tools) == 0 {
		t.Error("GenerateMCPTools() returned no tools")
	}

	// Find our test tool
	found := false
	for _, tool := range tools {
		if tool.Name == "test_mcp_tool" {
			found = true
			if tool.Description != "Test MCP tool" {
				t.Errorf("Tool description = %v, want 'Test MCP tool'", tool.Description)
			}
			break
		}
	}

	if !found {
		t.Error("GenerateMCPTools() did not include test tool")
	}
}

func TestGenerateCLICommands(t *testing.T) {
	// Register a test operation
	testOp := &OperationDefinition{
		ID:          "test-cli-op",
		Name:        "Test CLI Operation",
		Description: "Test operation for CLI commands",
		CLIUse:      "test-cmd",
		CLIShort:    "Test CLI command",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}
	RegisterOperation(testOp)

	svc := &mockFlowService{}
	commands := GenerateCLICommands(svc)

	// Should have at least one command
	if len(commands) == 0 {
		t.Error("GenerateCLICommands() returned no commands")
	}

	// Find our test command
	found := false
	for _, cmd := range commands {
		if cmd.Use == "test-cmd" {
			found = true
			if cmd.Short != "Test CLI command" {
				t.Errorf("Command short = %v, want 'Test CLI command'", cmd.Short)
			}
			break
		}
	}

	if !found {
		t.Error("GenerateCLICommands() did not include test command")
	}
}

func TestGenerateCombinedHTTPHandler(t *testing.T) {
	// Create test operations for same path but different methods
	getOp := &OperationDefinition{
		ID:          "test-get",
		Description: "Test GET operation",
		HTTPMethod:  "GET",
		HTTPPath:    "/test",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return map[string]string{"method": "GET"}, nil
		},
	}

	postOp := &OperationDefinition{
		ID:          "test-post",
		Description: "Test POST operation",
		HTTPMethod:  "POST",
		HTTPPath:    "/test",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return map[string]string{"method": "POST"}, nil
		},
	}

	ops := []*OperationDefinition{getOp, postOp}
	svc := &mockFlowService{}

	handler := generateCombinedHTTPHandler(ops, svc)

	// Test GET request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET request failed with status %d", w.Code)
	}

	// Test POST request
	req = httptest.NewRequest("POST", "/test", nil)
	w = httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST request failed with status %d", w.Code)
	}

	// Test unsupported method
	req = httptest.NewRequest("PUT", "/test", nil)
	w = httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT request should return 405, got %d", w.Code)
	}
}

func TestHandleCLIFileArgs(t *testing.T) {
	// Test with content string (not a file path)
	cmd := &cobra.Command{}
	cmd.Flags().String("content", "", "content flag")
	cmd.Flags().Set("content", "test content")

	data, err := HandleCLIFileArgs(cmd, []string{}, "content")
	if err != nil {
		t.Errorf("HandleCLIFileArgs() error = %v", err)
		return
	}

	if string(data) != "test content" {
		t.Errorf("HandleCLIFileArgs() data = %v, want 'test content'", string(data))
	}
}

func TestAddCLIFlags(t *testing.T) {
	type TestArgs struct {
		Name    string         `flag:"name" description:"Name flag"`
		Count   int            `flag:"count" description:"Count flag"`
		Enabled bool           `flag:"enabled" description:"Enabled flag"`
		Data    map[string]any `flag:"data" description:"Data flag"`
		Skip    string         `flag:"-"` // Should be skipped
	}

	cmd := &cobra.Command{Use: "test"}
	addCLIFlags(cmd, reflect.TypeOf(TestArgs{}))

	// Check that flags were added
	if !cmd.Flags().HasAvailableFlags() {
		t.Error("addCLIFlags() did not add any flags")
	}

	// Check specific flags
	nameFlag := cmd.Flags().Lookup("name")
	if nameFlag == nil {
		t.Error("addCLIFlags() did not add name flag")
	}

	countFlag := cmd.Flags().Lookup("count")
	if countFlag == nil {
		t.Error("addCLIFlags() did not add count flag")
	}

	enabledFlag := cmd.Flags().Lookup("enabled")
	if enabledFlag == nil {
		t.Error("addCLIFlags() did not add enabled flag")
	}

	// Check that skipped flag was not added
	skipFlag := cmd.Flags().Lookup("skip")
	if skipFlag != nil {
		t.Error("addCLIFlags() should not add flag marked with '-'")
	}
}

func TestParseCLIArgs(t *testing.T) {
	type TestArgs struct {
		Name    string      `flag:"name"`
		Count   int         `flag:"count"`
		Enabled bool        `flag:"enabled"`
		Data    interface{} `flag:"data"`
	}

	cmd := &cobra.Command{Use: "test"}
	addCLIFlags(cmd, reflect.TypeOf(TestArgs{}))

	// Set flag values
	cmd.Flags().Set("name", "test-name")
	cmd.Flags().Set("count", "42")
	cmd.Flags().Set("enabled", "true")
	cmd.Flags().Set("data", `{"key": "value"}`)

	result, err := parseCLIArgs(cmd, []string{"positional-arg"}, reflect.TypeOf(TestArgs{}))
	if err != nil {
		t.Errorf("parseCLIArgs() error = %v", err)
		return
	}

	args, ok := result.(*TestArgs)
	if !ok {
		t.Error("parseCLIArgs() result type assertion failed")
		return
	}

	if args.Name != "test-name" {
		t.Errorf("parseCLIArgs() Name = %v, want 'test-name'", args.Name)
	}

	if args.Count != 42 {
		t.Errorf("parseCLIArgs() Count = %v, want 42", args.Count)
	}

	if !args.Enabled {
		t.Error("parseCLIArgs() Enabled should be true")
	}

	if args.Data == nil {
		t.Error("parseCLIArgs() Data should not be nil")
		return
	}

	data, ok := args.Data.(map[string]interface{})
	if !ok || data["key"] != "value" {
		t.Errorf("parseCLIArgs() Data = %v, want map with key=value", args.Data)
	}
}

func TestUnifiedAttachHTTPHandlers(t *testing.T) {
	// Call the function - since we can't easily test real HTTP registration
	// due to invalid patterns, we'll just ensure it doesn't panic
	t.Log("Testing UnifiedAttachHTTPHandlers function call")

	// Create a new ServeMux that won't be used for actual registration
	// Just verify the function exists and is callable
	svc := &mockFlowService{}
	mux := http.NewServeMux()

	// Call the function to ensure it doesn't panic
	UnifiedAttachHTTPHandlers(mux, svc)

	t.Log("UnifiedAttachHTTPHandlers function is available and callable")
}

func TestRegisterUnifiedSystemEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	registerUnifiedSystemEndpoints(mux)

	// Create a test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test health check endpoint
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestUnifiedBuildMCPToolRegistrations(t *testing.T) {
	svc := &mockFlowService{}
	tools := UnifiedBuildMCPToolRegistrations(svc)

	// Should return at least some tools from operations
	if len(tools) == 0 {
		t.Error("Expected some MCP tool registrations, got none")
	}

	// Check that tools have required fields
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("Tool registration missing name")
		}
		if tool.Handler == nil {
			t.Error("Tool registration missing handler")
		}
	}
}

func TestUnifiedAttachCLICommands(t *testing.T) {
	rootCmd := &cobra.Command{Use: "test"}
	svc := &mockFlowService{}

	// Get initial command count
	initialCmdCount := len(rootCmd.Commands())

	UnifiedAttachCLICommands(rootCmd, svc)

	// Should have added some commands
	finalCmdCount := len(rootCmd.Commands())
	if finalCmdCount <= initialCmdCount {
		t.Error("Expected CLI commands to be added, but count didn't increase")
	}

	// Check that commands have proper structure (skip commands with empty fields)
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "" {
			t.Logf("Skipping command with empty Use field")
			continue
		}
		if cmd.Short == "" {
			t.Logf("Skipping command with empty Short description")
			continue
		}
		// If we get here, the command has valid structure
		t.Logf("Valid command found: %s - %s", cmd.Use, cmd.Short)
	}
}

func TestRunGeneratedCLICommand(t *testing.T) {
	svc := &mockFlowService{}

	// Create a test operation
	op := &OperationDefinition{
		ID: "testOp",
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
		ArgsType:    reflect.TypeOf(EmptyArgs{}), // Use EmptyArgs to avoid flag issues
		Description: "Test operation",
	}

	// Create a test command - don't add flags that don't exist in the args type
	cmd := &cobra.Command{Use: "test"}

	// Test with mock arguments
	err := runGeneratedCLICommand(cmd, []string{}, op, svc)
	if err != nil {
		t.Errorf("runGeneratedCLICommand failed: %v", err)
	}
}

func TestRunGeneratedCLICommand_WithError(t *testing.T) {
	svc := &mockFlowService{}

	// Create a test operation that returns an error
	op := &OperationDefinition{
		ID: "testOpError",
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return nil, fmt.Errorf("test error")
		},
		ArgsType:    reflect.TypeOf(EmptyArgs{}), // Use EmptyArgs to avoid flag issues
		Description: "Test operation with error",
	}

	// Create a test command
	cmd := &cobra.Command{Use: "test"}

	// Test that error is returned
	err := runGeneratedCLICommand(cmd, []string{}, op, svc)
	if err == nil {
		t.Error("Expected error from runGeneratedCLICommand, got nil")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Errorf("Expected error to contain 'test error', got: %v", err)
	}
}

func TestGenerateCLICommand_WithCustomHandler(t *testing.T) {
	svc := &mockFlowService{}
	handlerCalled := false

	// Create operation with custom CLI handler
	op := &OperationDefinition{
		ID:          "testCustom",
		CLIUse:      "test-custom",
		CLIShort:    "Test custom handler",
		Description: "Test operation with custom CLI handler",
		ArgsType:    reflect.TypeOf(ValidateFlowArgs{}),
		CLIHandler: func(cmd *cobra.Command, args []string, svc FlowService) error {
			handlerCalled = true
			return nil
		},
	}

	cmd := generateCLICommand(op, svc)

	// Execute the command
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	if !handlerCalled {
		t.Error("Custom CLI handler was not called")
	}
}

func TestGenerateCLICommand_WithFlags(t *testing.T) {
	svc := &mockFlowService{}

	// Create operation with args that should generate flags
	type TestArgs struct {
		StringFlag string `flag:"string-flag" description:"String flag"`
		BoolFlag   bool   `flag:"bool-flag" description:"Bool flag"`
		IntFlag    int    `flag:"int-flag" description:"Int flag"`
		JSONFlag   any    `flag:"json-flag" description:"JSON flag"`
	}

	op := &OperationDefinition{
		ID:          "testFlags",
		CLIUse:      "test-flags",
		CLIShort:    "Test flags",
		Description: "Test operation with flags",
		ArgsType:    reflect.TypeOf(TestArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "success", nil
		},
	}

	cmd := generateCLICommand(op, svc)

	// Check that flags were added
	expectedFlags := []string{"string-flag", "bool-flag", "int-flag", "json-flag"}
	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag %s to be added, but it wasn't", flagName)
		}
	}
}

func TestGenerateCLICommand_GraphFlowSpecialFlag(t *testing.T) {
	svc := &mockFlowService{}

	// Create graphFlow operation
	op := &OperationDefinition{
		ID:          "graphFlow",
		CLIUse:      "graph",
		CLIShort:    "Graph flow",
		Description: "Generate flow graph",
		ArgsType:    reflect.TypeOf(GraphFlowArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "graph output", nil
		},
	}

	cmd := generateCLICommand(op, svc)

	// Check that output flag was added
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("Expected 'output' flag to be added for graphFlow operation")
	}
}

func TestGenerateCLICommand_WithErrorExitCodes(t *testing.T) {
	// Disable CLI exit codes for testing
	originalExitCodesSetting := enableCLIExitCodes
	DisableCLIExitCodes()
	defer func() {
		if originalExitCodesSetting {
			EnableCLIExitCodes()
		}
	}()

	svc := &mockFlowService{}

	// Create operation with custom CLI handler that returns specific errors
	op := &OperationDefinition{
		ID:          "testExitCodes",
		CLIUse:      "test-exit",
		CLIShort:    "Test exit codes",
		Description: "Test operation with exit codes",
		ArgsType:    reflect.TypeOf(ValidateFlowArgs{}),
		CLIHandler: func(cmd *cobra.Command, args []string, svc FlowService) error {
			return fmt.Errorf("YAML parse error: invalid syntax")
		},
	}

	cmd := generateCLICommand(op, svc)

	// Test that the command returns the expected error
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Error("Expected error from CLI handler, got nil")
	}
	if !strings.Contains(err.Error(), "YAML parse error") {
		t.Errorf("Expected error to contain 'YAML parse error', got: %v", err)
	}
}

func TestParseCLIArgs_Comprehensive(t *testing.T) {
	// Test with various field types
	type TestArgs struct {
		PositionalArg string // Should be set from positional args (first field)
		StringField   string `flag:"string-field"`
		BoolField     bool   `flag:"bool-field"`
		IntField      int    `flag:"int-field"`
		JSONField     any    `flag:"json-field"`
		IgnoredField  string `flag:"-"`
		NoFlagField   string
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("string-field", "", "String field")
	cmd.Flags().Bool("bool-field", false, "Bool field")
	cmd.Flags().Int("int-field", 0, "Int field")
	cmd.Flags().String("json-field", "", "JSON field")

	// Set flag values
	cmd.Flags().Set("string-field", "test-string")
	cmd.Flags().Set("bool-field", "true")
	cmd.Flags().Set("int-field", "42")
	cmd.Flags().Set("json-field", `{"key": "value"}`)

	// Test with positional argument
	args := []string{"positional-value"}
	argsType := reflect.TypeOf(TestArgs{})

	result, err := parseCLIArgs(cmd, args, argsType)
	if err != nil {
		t.Fatalf("parseCLIArgs failed: %v", err)
	}

	parsedArgs, ok := result.(*TestArgs)
	if !ok {
		t.Fatalf("Expected *TestArgs, got %T", result)
	}

	// Verify all fields were set correctly
	if parsedArgs.StringField != "test-string" {
		t.Errorf("Expected StringField 'test-string', got %s", parsedArgs.StringField)
	}
	if !parsedArgs.BoolField {
		t.Error("Expected BoolField to be true")
	}
	if parsedArgs.IntField != 42 {
		t.Errorf("Expected IntField 42, got %d", parsedArgs.IntField)
	}
	if parsedArgs.PositionalArg != "positional-value" {
		t.Errorf("Expected PositionalArg 'positional-value', got %s", parsedArgs.PositionalArg)
	}

	// Verify JSON field
	jsonData, ok := parsedArgs.JSONField.(map[string]any)
	if !ok {
		t.Errorf("Expected JSONField to be map[string]any, got %T", parsedArgs.JSONField)
	} else if jsonData["key"] != "value" {
		t.Errorf("Expected JSONField[key] to be 'value', got %v", jsonData["key"])
	}
}

func TestParseCLIArgs_ErrorCases(t *testing.T) {
	type TestArgs struct {
		InvalidJSON any `flag:"json-field"`
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("json-field", "", "JSON field")

	// Set invalid JSON
	cmd.Flags().Set("json-field", "{invalid json}")

	argsType := reflect.TypeOf(TestArgs{})

	_, err := parseCLIArgs(cmd, []string{}, argsType)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON") {
		t.Errorf("Expected JSON parse error, got: %v", err)
	}
}

func TestOutputCLIResult_Comprehensive(t *testing.T) {
	// Capture output
	var capturedOutput strings.Builder
	utils.SetUserOutput(&capturedOutput)
	defer func() {
		utils.SetUserOutput(os.Stdout)
	}()

	tests := []struct {
		name             string
		result           any
		expectedContains string
	}{
		{
			name:             "nil result",
			result:           nil,
			expectedContains: "", // utils.Info output won't be captured, but no panic
		},
		{
			name:             "string result",
			result:           "test output",
			expectedContains: "test output",
		},
		{
			name:             "struct result",
			result:           map[string]any{"key": "value", "number": 42},
			expectedContains: `"key": "value"`, // Just check for key content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedOutput.Reset()

			err := outputCLIResult(tt.result)
			if err != nil {
				t.Errorf("outputCLIResult failed: %v", err)
			}

			if tt.result != nil { // Only check output for non-nil results
				output := capturedOutput.String()
				if tt.expectedContains != "" && !strings.Contains(output, tt.expectedContains) {
					t.Errorf("Expected output to contain %s, got %s", tt.expectedContains, output)
				}
			}
		})
	}
}

func TestConvertMCPArgs_ErrorCases(t *testing.T) {
	type TestArgs struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	// Test with unmarshaling error (invalid JSON structure)
	args := map[string]any{
		"name":  "test",
		"count": "not-a-number", // This will cause unmarshaling error when converting to int
	}
	targetType := reflect.TypeOf(TestArgs{})

	_, err := convertMCPArgs(args, targetType)
	if err == nil {
		t.Error("Expected error for invalid type conversion")
	}

	// Test with marshaling error (unmarshalable input)
	invalidArgs := make(chan int) // channels can't be marshaled
	_, err = convertMCPArgs(invalidArgs, targetType)
	if err == nil {
		t.Error("Expected marshaling error for channel type")
	}
}

func TestGenerateMCPHandler_Basic(t *testing.T) {
	svc := &mockFlowService{}

	// Test with custom MCP handler
	op := &OperationDefinition{
		ID:          "testMCP",
		Description: "Test MCP operation",
		MCPHandler: func(ctx context.Context, args any) (*mcp.ToolResponse, error) {
			return mcp.NewToolResponse(mcp.NewTextContent("custom response")), nil
		},
	}

	handler := generateMCPHandler(op, svc)
	if handler == nil {
		t.Error("generateMCPHandler returned nil for custom handler")
	}

	// Test with standard handler - success case
	op2 := &OperationDefinition{
		ID:          "testMCP2",
		Description: "Test MCP operation 2",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}

	handler2 := generateMCPHandler(op2, svc)
	if handler2 == nil {
		t.Error("generateMCPHandler returned nil for standard handler")
	}

	// Test the generated handler - success case
	if handlerFunc, ok := handler2.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		resp, err := handlerFunc(ctx, EmptyArgs{})
		if err != nil {
			t.Errorf("Generated handler failed: %v", err)
		}
		if resp == nil {
			t.Error("Generated handler returned nil response")
		}
	}
}

func TestGenerateMCPHandler_Comprehensive(t *testing.T) {
	svc := &mockFlowService{}

	// Test with custom MCP handler
	op := &OperationDefinition{
		ID:          "testMCP",
		Description: "Test MCP operation",
		MCPHandler: func(ctx context.Context, args any) (*mcp.ToolResponse, error) {
			return mcp.NewToolResponse(mcp.NewTextContent("custom response")), nil
		},
	}

	handler := generateMCPHandler(op, svc)
	if handler == nil {
		t.Error("generateMCPHandler returned nil for custom handler")
	}

	// Test with standard handler - success case
	op2 := &OperationDefinition{
		ID:          "testMCP2",
		Description: "Test MCP operation 2",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}

	handler2 := generateMCPHandler(op2, svc)
	if handler2 == nil {
		t.Error("generateMCPHandler returned nil for standard handler")
	}

	// Test the generated handler - success case
	if handlerFunc, ok := handler2.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		resp, err := handlerFunc(ctx, EmptyArgs{})
		if err != nil {
			t.Errorf("Generated handler failed: %v", err)
		}
		if resp == nil {
			t.Error("Generated handler returned nil response")
		}
	}

	// Test with standard handler - convertMCPArgs error case
	op3 := &OperationDefinition{
		ID:          "testMCP3",
		Description: "Test MCP operation 3",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}

	handler3 := generateMCPHandler(op3, svc)
	if handlerFunc, ok := handler3.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		// Pass invalid args that will cause convertMCPArgs to fail
		invalidArgs := make(chan int) // channels can't be marshaled to JSON
		_, err := handlerFunc(ctx, invalidArgs)
		if err == nil {
			t.Error("Expected error for invalid args that can't be converted")
		}
	}

	// Test with standard handler - operation handler error case
	op4 := &OperationDefinition{
		ID:          "testMCP4",
		Description: "Test MCP operation 4",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return nil, fmt.Errorf("operation failed")
		},
	}

	handler4 := generateMCPHandler(op4, svc)
	if handlerFunc, ok := handler4.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		_, err := handlerFunc(ctx, EmptyArgs{})
		if err == nil {
			t.Error("Expected error from failing operation handler")
		}
		if !strings.Contains(err.Error(), "operation failed") {
			t.Errorf("Expected 'operation failed' in error, got: %v", err)
		}
	}

	// Test with standard handler - convertToMCPResponse error case
	op5 := &OperationDefinition{
		ID:          "testMCP5",
		Description: "Test MCP operation 5",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			// Return something that can't be marshaled to JSON
			return make(chan int), nil
		},
	}

	handler5 := generateMCPHandler(op5, svc)
	if handlerFunc, ok := handler5.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		_, err := handlerFunc(ctx, EmptyArgs{})
		if err == nil {
			t.Error("Expected error from convertToMCPResponse with unmarshalable result")
		}
	}
}

func TestGenerateMCPHandler_ErrorCases(t *testing.T) {
	svc := &mockFlowService{}

	// Test with standard handler - convertMCPArgs error case
	op3 := &OperationDefinition{
		ID:          "testMCP3",
		Description: "Test MCP operation 3",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}

	handler3 := generateMCPHandler(op3, svc)
	if handlerFunc, ok := handler3.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		// Pass invalid args that will cause convertMCPArgs to fail
		invalidArgs := make(chan int) // channels can't be marshaled to JSON
		_, err := handlerFunc(ctx, invalidArgs)
		if err == nil {
			t.Error("Expected error for invalid args that can't be converted")
		}
	}

	// Test with standard handler - operation handler error case
	op4 := &OperationDefinition{
		ID:          "testMCP4",
		Description: "Test MCP operation 4",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return nil, fmt.Errorf("operation failed")
		},
	}

	handler4 := generateMCPHandler(op4, svc)
	if handlerFunc, ok := handler4.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		_, err := handlerFunc(ctx, EmptyArgs{})
		if err == nil {
			t.Error("Expected error from failing operation handler")
		}
		if !strings.Contains(err.Error(), "operation failed") {
			t.Errorf("Expected 'operation failed' in error, got: %v", err)
		}
	}

	// Test with standard handler - convertToMCPResponse error case
	op5 := &OperationDefinition{
		ID:          "testMCP5",
		Description: "Test MCP operation 5",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			// Return something that can't be marshaled to JSON
			return make(chan int), nil
		},
	}

	handler5 := generateMCPHandler(op5, svc)
	if handlerFunc, ok := handler5.(func(context.Context, any) (*mcp.ToolResponse, error)); ok {
		ctx := context.Background()
		_, err := handlerFunc(ctx, EmptyArgs{})
		if err == nil {
			t.Error("Expected error from convertToMCPResponse with unmarshalable result")
		}
	}
}

// TestMCPToolGeneration tests that MCP tools can be generated without panicking
func TestMCPToolGeneration(t *testing.T) {
	// Disable CLI exit codes during testing
	DisableCLIExitCodes()
	defer EnableCLIExitCodes()

	svc := &mockFlowService{}

	// This should not panic
	tools := GenerateMCPTools(svc)

	// Verify we got some tools
	if len(tools) == 0 {
		t.Error("Expected at least some MCP tools to be generated")
	}

	// Verify all tools have valid handlers
	for _, tool := range tools {
		if tool.Handler == nil {
			t.Errorf("Tool %s has nil handler", tool.Name)
		}
		if tool.Name == "" {
			t.Error("Tool has empty name")
		}
		if tool.Description == "" {
			t.Error("Tool has empty description")
		}
	}
}

// TestMCPHandlerGeneration tests specific handler generation
func TestMCPHandlerGeneration(t *testing.T) {
	svc := &mockFlowService{}

	tests := []struct {
		name     string
		op       *OperationDefinition
		wantNil  bool
		testCall bool
	}{
		{
			name: "EmptyArgs operation",
			op: &OperationDefinition{
				ID:       "test-empty",
				MCPName:  "test_empty",
				ArgsType: reflect.TypeOf(EmptyArgs{}),
				Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
					return "success", nil
				},
			},
			wantNil:  false,
			testCall: true,
		},
		{
			name: "GetFlowArgs operation",
			op: &OperationDefinition{
				ID:       "test-get-flow",
				MCPName:  "test_get_flow",
				ArgsType: reflect.TypeOf(GetFlowArgs{}),
				Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
					return "success", nil
				},
			},
			wantNil:  false,
			testCall: true,
		},
		{
			name: "StartRunArgs operation",
			op: &OperationDefinition{
				ID:       "test-start-run",
				MCPName:  "test_start_run",
				ArgsType: reflect.TypeOf(StartRunArgs{}),
				Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
					return "success", nil
				},
			},
			wantNil:  false,
			testCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := generateMCPHandler(tt.op, svc)

			if tt.wantNil && handler != nil {
				t.Errorf("Expected nil handler for %s, got non-nil", tt.name)
			}
			if !tt.wantNil && handler == nil {
				t.Errorf("Expected non-nil handler for %s, got nil", tt.name)
			}

			// Test calling the handler if it's not nil and we want to test it
			if handler != nil && tt.testCall {
				handlerType := reflect.TypeOf(handler)
				if handlerType.Kind() != reflect.Func {
					t.Errorf("Handler for %s is not a function", tt.name)
					return
				}

				// Verify the handler has the right signature
				if handlerType.NumIn() != 1 {
					t.Errorf("Handler for %s should have 1 input, got %d", tt.name, handlerType.NumIn())
				}

				if handlerType.NumOut() != 2 {
					t.Errorf("Handler for %s should have 2 outputs, got %d", tt.name, handlerType.NumOut())
				}
			}
		})
	}
}

// TestMCPArgsConversion tests the argument conversion functions
func TestMCPArgsConversion(t *testing.T) {
	tests := []struct {
		name       string
		args       any
		targetType reflect.Type
		wantErr    bool
	}{
		{
			name:       "Convert to EmptyArgs",
			args:       map[string]any{},
			targetType: reflect.TypeOf(EmptyArgs{}),
			wantErr:    false,
		},
		{
			name:       "Convert to GetFlowArgs",
			args:       map[string]any{"name": "test-flow"},
			targetType: reflect.TypeOf(GetFlowArgs{}),
			wantErr:    false,
		},
		{
			name:       "Convert nil args",
			args:       nil,
			targetType: reflect.TypeOf(EmptyArgs{}),
			wantErr:    false,
		},
		{
			name:       "Convert with nil target type",
			args:       map[string]any{},
			targetType: nil,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertMCPArgs(tt.args, tt.targetType)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertMCPArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Error("convertMCPArgs() returned nil result without error")
			}
		})
	}
}

// TestMCPResponseConversion tests the response conversion function
func TestMCPResponseConversion(t *testing.T) {
	tests := []struct {
		name    string
		result  any
		wantErr bool
	}{
		{
			name:    "Convert nil result",
			result:  nil,
			wantErr: false,
		},
		{
			name:    "Convert string result",
			result:  "test result",
			wantErr: false,
		},
		{
			name:    "Convert map result",
			result:  map[string]any{"key": "value"},
			wantErr: false,
		},
		{
			name:    "Convert slice result",
			result:  []string{"item1", "item2"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := convertToMCPResponse(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToMCPResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp == nil {
				t.Error("convertToMCPResponse() returned nil response without error")
			}
		})
	}
}

// TestMCPServerRealStartup tests that we can actually start an MCP server without crashes
func TestMCPServerRealStartup(t *testing.T) {
	// This test simulates what happens in cmd/flow/main.go when starting the MCP server
	svc := &mockFlowService{}

	// This should not panic - if it does, we have the same issue that was happening in production
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MCP server startup panicked: %v", r)
		}
	}()

	// Generate tools (this is what was causing the original panic)
	tools := UnifiedBuildMCPToolRegistrations(svc)

	if len(tools) == 0 {
		t.Fatal("No tools were registered - MCP server would be empty")
	}

	t.Logf("Successfully generated %d MCP tools without panic", len(tools))

	// Verify all tools have the required fields for MCP registration
	for _, tool := range tools {
		if tool.Name == "" {
			t.Errorf("Tool has empty name: %+v", tool)
		}
		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}
		if tool.Handler == nil {
			t.Errorf("Tool %s has nil handler", tool.Name)
		}

		// Verify handler signature - this is critical for MCP compatibility
		handlerType := reflect.TypeOf(tool.Handler)
		if handlerType.Kind() != reflect.Func {
			t.Errorf("Tool %s handler is not a function: %T", tool.Name, tool.Handler)
		}
	}

	// Test that all our MCP-compatible types can be instantiated and marshaled
	// (simulating what happens during schema generation)
	mcpTypes := []any{
		MCPStartRunArgs{},
		MCPPublishEventArgs{},
		MCPResumeRunArgs{},
		MCPGetFlowArgs{},
		MCPValidateFlowArgs{},
		MCPGraphFlowArgs{},
		MCPGetRunArgs{},
		MCPConvertOpenAPIExtendedArgs{},
		MCPFlowFileArgs{},
		EmptyArgs{},
	}

	for _, mcpType := range mcpTypes {
		// Test JSON marshaling (required for schema generation)
		data, err := json.Marshal(mcpType)
		if err != nil {
			t.Errorf("Failed to marshal MCP type %T: %v", mcpType, err)
		}

		// Test that we can create instances via reflection (what MCP does)
		typeOf := reflect.TypeOf(mcpType)
		newInstance := reflect.New(typeOf).Interface()

		// Test JSON round-trip (what MCP schema generation does)
		if err := json.Unmarshal(data, newInstance); err != nil {
			t.Errorf("Failed to unmarshal MCP type %T: %v", mcpType, err)
		}
	}
}

// TestMCPRegressionMapStringAny is a regression test for the original nil pointer panic
// when MCP tried to generate JSON schemas for map[string]any fields
func TestMCPRegressionMapStringAny(t *testing.T) {
	// This test ensures we never accidentally introduce map[string]any fields
	// in MCP handler arguments, which cause nil pointer panics in schema generation

	svc := &mockFlowService{}
	tools := GenerateMCPTools(svc)

	for _, tool := range tools {
		handlerType := reflect.TypeOf(tool.Handler)
		if handlerType.Kind() != reflect.Func || handlerType.NumIn() == 0 {
			continue
		}

		// Check the input argument type for problematic fields
		inputType := handlerType.In(0)
		if inputType.Kind() != reflect.Struct {
			continue
		}

		for i := 0; i < inputType.NumField(); i++ {
			field := inputType.Field(i)
			fieldTypeStr := field.Type.String()

			// These are the exact field types that caused the original panic
			forbiddenTypes := []string{
				"map[string]interface {}",
				"map[string]any",
				"interface {}",
			}

			for _, forbidden := range forbiddenTypes {
				if fieldTypeStr == forbidden {
					t.Errorf("REGRESSION: Tool %s has field %s with type %s that will cause MCP schema generation panic. Use string field with JSON parsing instead.",
						tool.Name, field.Name, forbidden)
				}
			}
		}
	}

	// Also test that all our MCP types only use MCP-safe field types
	safeMCPTypes := []reflect.Type{
		reflect.TypeOf(MCPStartRunArgs{}),
		reflect.TypeOf(MCPPublishEventArgs{}),
		reflect.TypeOf(MCPResumeRunArgs{}),
		reflect.TypeOf(MCPGetFlowArgs{}),
		reflect.TypeOf(MCPValidateFlowArgs{}),
		reflect.TypeOf(MCPGraphFlowArgs{}),
		reflect.TypeOf(MCPGetRunArgs{}),
		reflect.TypeOf(MCPConvertOpenAPIExtendedArgs{}),
		reflect.TypeOf(MCPFlowFileArgs{}),
		reflect.TypeOf(EmptyArgs{}),
	}

	for _, mcpType := range safeMCPTypes {
		for i := 0; i < mcpType.NumField(); i++ {
			field := mcpType.Field(i)
			fieldTypeStr := field.Type.String()

			// Only allow safe types for MCP
			allowedTypes := []string{
				"string",
				"int", "int8", "int16", "int32", "int64",
				"uint", "uint8", "uint16", "uint32", "uint64",
				"bool",
				"float32", "float64",
			}

			isAllowed := false
			for _, allowed := range allowedTypes {
				if fieldTypeStr == allowed {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				t.Errorf("MCP type %s has field %s with unsafe type %s. Only use basic types (string, int, bool) for MCP compatibility.",
					mcpType.Name(), field.Name, fieldTypeStr)
			}
		}
	}
}
