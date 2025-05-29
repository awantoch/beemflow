package api

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetOperation(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
		checkOp func(*OperationDefinition) bool
	}{
		{
			name:    "existing operation",
			id:      "listFlows",
			wantErr: false,
			checkOp: func(op *OperationDefinition) bool {
				return op != nil && op.ID == "listFlows"
			},
		},
		{
			name:    "non-existent operation",
			id:      "non-existent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, exists := GetOperation(tt.id)

			if !exists && !tt.wantErr {
				t.Errorf("GetOperation() operation not found, want exists")
				return
			}

			if exists && tt.wantErr {
				t.Errorf("GetOperation() operation found, want not exists")
				return
			}

			if !tt.wantErr && tt.checkOp != nil && !tt.checkOp(op) {
				t.Errorf("GetOperation() operation check failed")
			}
		})
	}
}

func TestGetAllOperations(t *testing.T) {
	ops := GetAllOperations()

	if len(ops) == 0 {
		t.Error("GetAllOperations() returned no operations")
	}

	// Check that we have some expected operations
	expectedOps := []string{"listFlows", "getFlow", "validateFlow", "startRun"}
	for _, expected := range expectedOps {
		if _, exists := ops[expected]; !exists {
			t.Errorf("GetAllOperations() missing expected operation: %s", expected)
		}
	}
}

func TestValidateFlowCLIHandler(t *testing.T) {
	// Create a temporary flow file for testing
	tempDir := t.TempDir()
	flowPath := filepath.Join(tempDir, "test.flow.yaml")
	flowContent := `name: test
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	svc := &mockFlowService{}
	cmd := &cobra.Command{}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many arguments",
			args:    []string{"arg1", "arg2"},
			wantErr: true,
		},
		{
			name:    "valid flow name",
			args:    []string{"test-flow"},
			wantErr: false,
		},
		{
			name:    "invalid flow name",
			args:    []string{"invalid-flow"},
			wantErr: true,
		},
		{
			name:    "valid file path",
			args:    []string{flowPath},
			wantErr: false,
		},
		{
			name:    "invalid file path",
			args:    []string{"/nonexistent/file.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlowCLIHandler(cmd, tt.args, svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFlowCLIHandler() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFlowHandler(t *testing.T) {
	// Create a temporary flow file for testing
	tempDir := t.TempDir()
	flowPath := filepath.Join(tempDir, "test.flow.yaml")
	flowContent := `name: test
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	ctx := context.Background()
	svc := &mockFlowService{}

	tests := []struct {
		name    string
		args    *ValidateFlowArgs
		wantErr bool
	}{
		{
			name:    "valid flow name",
			args:    &ValidateFlowArgs{Name: "test-flow"},
			wantErr: false,
		},
		{
			name:    "invalid flow name",
			args:    &ValidateFlowArgs{Name: "invalid-flow"},
			wantErr: true,
		},
		{
			name:    "valid file path",
			args:    &ValidateFlowArgs{Name: flowPath},
			wantErr: false,
		},
		{
			name:    "invalid file path",
			args:    &ValidateFlowArgs{Name: "/nonexistent/file.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateFlowHandler(ctx, svc, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFlowHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("validateFlowHandler() returned nil result for valid input")
			}
		})
	}
}

func TestGraphFlowCLIHandler(t *testing.T) {
	// Create a temporary flow file for testing
	tempDir := t.TempDir()
	flowPath := filepath.Join(tempDir, "test.flow.yaml")
	flowContent := `name: test
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	svc := &mockFlowService{}
	cmd := &cobra.Command{}
	cmd.Flags().StringP("output", "o", "", "output path")

	tests := []struct {
		name       string
		args       []string
		outputFlag string
		wantErr    bool
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many arguments",
			args:    []string{"arg1", "arg2"},
			wantErr: true,
		},
		{
			name:    "valid flow name",
			args:    []string{"test-flow"},
			wantErr: false,
		},
		{
			name:    "valid file path",
			args:    []string{flowPath},
			wantErr: false,
		},
		{
			name:       "with output flag",
			args:       []string{"test-flow"},
			outputFlag: filepath.Join(tempDir, "output.md"),
			wantErr:    false,
		},
		{
			name:    "invalid file path",
			args:    []string{"/nonexistent/file.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.outputFlag != "" {
				cmd.Flags().Set("output", tt.outputFlag)
			} else {
				cmd.Flags().Set("output", "")
			}

			err := graphFlowCLIHandler(cmd, tt.args, svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("graphFlowCLIHandler() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check if output file was created when flag was set
			if !tt.wantErr && tt.outputFlag != "" {
				if _, err := os.Stat(tt.outputFlag); os.IsNotExist(err) {
					t.Errorf("graphFlowCLIHandler() did not create output file")
				}
			}
		})
	}
}

func TestGraphFlowHandler(t *testing.T) {
	// Create a temporary flow file for testing
	tempDir := t.TempDir()
	flowPath := filepath.Join(tempDir, "test.flow.yaml")
	flowContent := `name: test
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	ctx := context.Background()
	svc := &mockFlowService{}

	tests := []struct {
		name    string
		args    *GraphFlowArgs
		wantErr bool
	}{
		{
			name:    "valid flow name",
			args:    &GraphFlowArgs{Name: "test-flow"},
			wantErr: false,
		},
		{
			name:    "valid file path",
			args:    &GraphFlowArgs{Name: flowPath},
			wantErr: false,
		},
		{
			name:    "invalid file path",
			args:    &GraphFlowArgs{Name: "/nonexistent/file.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := graphFlowHandler(ctx, svc, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("graphFlowHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("graphFlowHandler() returned nil result for valid input")
			}
		})
	}
}

func TestLintFlowCLIHandler(t *testing.T) {
	// Create a temporary flow file for testing
	tempDir := t.TempDir()
	validFlowPath := filepath.Join(tempDir, "valid.flow.yaml")
	validFlowContent := `name: valid
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(validFlowPath, []byte(validFlowContent), 0644); err != nil {
		t.Fatalf("Failed to write valid flow: %v", err)
	}

	invalidFlowPath := filepath.Join(tempDir, "invalid.flow.yaml")
	invalidFlowContent := `not: [valid: yaml`
	if err := os.WriteFile(invalidFlowPath, []byte(invalidFlowContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid flow: %v", err)
	}

	svc := &mockFlowService{}
	cmd := &cobra.Command{}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many arguments",
			args:    []string{"arg1", "arg2"},
			wantErr: true,
		},
		{
			name:    "valid flow file",
			args:    []string{validFlowPath},
			wantErr: false,
		},
		{
			name:    "invalid flow file",
			args:    []string{invalidFlowPath},
			wantErr: true,
		},
		{
			name:    "non-existent file",
			args:    []string{"/nonexistent/file.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := lintFlowCLIHandler(cmd, tt.args, svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("lintFlowCLIHandler() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLintFlowHandler(t *testing.T) {
	// Create a temporary flow file for testing
	tempDir := t.TempDir()
	validFlowPath := filepath.Join(tempDir, "valid.flow.yaml")
	validFlowContent := `name: valid
on: cli.manual
steps:
  - id: test_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(validFlowPath, []byte(validFlowContent), 0644); err != nil {
		t.Fatalf("Failed to write valid flow: %v", err)
	}

	ctx := context.Background()
	svc := &mockFlowService{}

	tests := []struct {
		name    string
		args    *FlowFileArgs
		wantErr bool
	}{
		{
			name:    "valid flow file",
			args:    &FlowFileArgs{File: validFlowPath},
			wantErr: false,
		},
		{
			name:    "non-existent file",
			args:    &FlowFileArgs{File: "/nonexistent/file.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lintFlowHandler(ctx, svc, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("lintFlowHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("lintFlowHandler() returned nil result for valid input")
			}
		})
	}
}

func TestRegisterOperation(t *testing.T) {
	// Create a test operation
	testOp := &OperationDefinition{
		ID:          "test-register-op",
		Name:        "Test Register Operation",
		Description: "Test operation for registration",
		HTTPMethod:  "GET",
		HTTPPath:    "/test-register",
		ArgsType:    reflect.TypeOf(EmptyArgs{}),
		Handler: func(ctx context.Context, svc FlowService, args any) (any, error) {
			return "test result", nil
		},
	}

	// Register the operation
	RegisterOperation(testOp)

	// Check that it was registered
	op, exists := GetOperation("test-register-op")
	if !exists {
		t.Error("RegisterOperation() operation was not registered")
		return
	}

	if op.ID != "test-register-op" {
		t.Errorf("RegisterOperation() operation ID = %v, want 'test-register-op'", op.ID)
	}

	if op.MCPName == "" {
		t.Error("RegisterOperation() should set MCPName to ID when empty")
	}
}

func TestRegisterCustomOperationHandlers(t *testing.T) {
	// This function should not panic when called
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RegisterCustomOperationHandlers() panicked: %v", r)
		}
	}()

	RegisterCustomOperationHandlers()
}

func TestConvertOpenAPICLIHandler(t *testing.T) {
	svc := &mockFlowService{}
	cmd := &cobra.Command{}

	// Add required flags
	cmd.Flags().String("openapi", "", "OpenAPI spec")
	cmd.Flags().String("api-name", "", "API name")
	cmd.Flags().String("base-url", "", "Base URL")
	cmd.Flags().String("output", "", "Output path")

	tests := []struct {
		name    string
		args    []string
		setup   func()
		wantErr bool
	}{
		{
			name:    "no arguments and no flags",
			args:    []string{},
			wantErr: true,
		},
		{
			name: "with openapi flag",
			args: []string{},
			setup: func() {
				cmd.Flags().Set("openapi", `{"openapi": "3.0.0", "info": {"title": "Test API", "version": "1.0.0"}, "paths": {}}`)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			cmd.Flags().Set("openapi", "")
			cmd.Flags().Set("api-name", "")
			cmd.Flags().Set("base-url", "")
			cmd.Flags().Set("output", "")

			if tt.setup != nil {
				tt.setup()
			}

			err := convertOpenAPICLIHandler(cmd, tt.args, svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertOpenAPICLIHandler() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOperationRegistrations(t *testing.T) {
	// Test that all expected operations are registered
	expectedOps := []string{
		"listFlows",
		"getFlow",
		"validateFlow",
		"graphFlow",
		"startRun",
		"getRun",
		"listRuns",
		"publishEvent",
		"resumeRun",
		"listTools",
		"getToolManifest",
		"spec",
		"lintFlow",
		"testFlow",
	}

	ops := GetAllOperations()

	for _, expected := range expectedOps {
		if _, exists := ops[expected]; !exists {
			t.Errorf("Expected operation '%s' not registered", expected)
		}
	}

	// Test that each registered operation has required fields
	for id, op := range ops {
		// Skip test operations created during testing
		if strings.HasPrefix(id, "test-") {
			continue
		}

		if op.ID == "" {
			t.Errorf("Operation %s has empty ID", id)
		}
		if op.Name == "" {
			t.Errorf("Operation %s has empty Name", id)
		}
		if op.ArgsType == nil {
			t.Errorf("Operation %s has nil ArgsType", id)
		}
		if op.Handler == nil {
			t.Errorf("Operation %s has nil Handler", id)
		}
		if op.MCPName == "" {
			t.Errorf("Operation %s has empty MCPName", id)
		}
	}
}

func TestArgumentTypes(t *testing.T) {
	// Test that argument type structs can be instantiated
	argTypes := []reflect.Type{
		reflect.TypeOf(EmptyArgs{}),
		reflect.TypeOf(GetFlowArgs{}),
		reflect.TypeOf(ValidateFlowArgs{}),
		reflect.TypeOf(GraphFlowArgs{}),
		reflect.TypeOf(StartRunArgs{}),
		reflect.TypeOf(GetRunArgs{}),
		reflect.TypeOf(PublishEventArgs{}),
		reflect.TypeOf(ResumeRunArgs{}),
		reflect.TypeOf(ConvertOpenAPIArgs{}),
		reflect.TypeOf(FlowFileArgs{}),
	}

	for _, argType := range argTypes {
		// Should be able to create new instances
		instance := reflect.New(argType).Interface()
		if instance == nil {
			t.Errorf("Failed to create instance of %v", argType)
		}

		// Should be able to get the type name
		if argType.Name() == "" {
			t.Errorf("Type %v has empty name", argType)
		}
	}
}

func TestOutputConvertResult(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "output.txt")

	tests := []struct {
		name       string
		result     any
		outputPath string
		wantErr    bool
	}{
		{
			name:       "string result with output path",
			result:     "test output",
			outputPath: outputPath,
			wantErr:    false,
		},
		{
			name:       "string result without output path",
			result:     "test output",
			outputPath: "",
			wantErr:    false,
		},
		{
			name:       "struct result with output path",
			result:     map[string]string{"key": "value"},
			outputPath: outputPath,
			wantErr:    false,
		},
		{
			name:       "nil result",
			result:     nil,
			outputPath: outputPath,
			wantErr:    false,
		},
		{
			name:       "invalid output path",
			result:     "test",
			outputPath: "/invalid/path/that/does/not/exist/file.txt",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := outputConvertResult(tt.result, tt.outputPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("outputConvertResult() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check file was created when path provided and no error
			if !tt.wantErr && tt.outputPath != "" && !strings.Contains(tt.outputPath, "invalid") {
				if _, err := os.Stat(tt.outputPath); os.IsNotExist(err) {
					t.Errorf("outputConvertResult() did not create output file")
				}
			}
		})
	}
}
