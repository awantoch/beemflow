package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

func captureOutput(f func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	utils.SetUserOutput(w)
	f()
	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		log.Printf("buf.ReadFrom failed: %v", err)
	}
	os.Stdout = orig
	utils.SetUserOutput(orig)
	return buf.String()
}

func captureStderrExit(f func()) (output string, code int) {
	origStderr := os.Stderr
	origExit := exit
	r, w, _ := os.Pipe()
	os.Stderr = w
	utils.SetInternalOutput(w)
	var buf bytes.Buffer
	var out string
	exitCode := 0
	exit = func(code int) {
		exitCode = code
		w.Close()
		panic("exit")
	}
	func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic occurred: %v", err)
			}
		}()
		f()
	}()
	w.Close()
	if _, err := io.Copy(&buf, r); err != nil {
		log.Printf("io.Copy failed: %v", err)
	}
	os.Stderr = origStderr
	utils.SetInternalOutput(origStderr)
	exit = origExit
	out = buf.String()
	return out, exitCode
}

func TestMainCommands(t *testing.T) {
	cases := []struct {
		args        []string
		wantsOutput bool
	}{
		{[]string{"flow", "serve"}, true},
		{[]string{"flow", "run"}, true},
		{[]string{"flow", "test"}, true},
		{[]string{"flow", "convert", "--help"}, true},
	}
	for _, c := range cases {
		os.Args = c.args
		out := captureOutput(func() {
			if err := NewRootCmd().Execute(); err != nil {
				log.Printf("Execute failed: %v", err)
			}
		})
		if c.wantsOutput && out == "" {
			t.Errorf("expected output for %v, got empty", c.args)
		}
	}
}

func TestMain_LintValidateCommands(t *testing.T) {
	t.Skip("Temporarily skipping while unified system is being finalized")
	// Valid flow file
	valid := `name: test
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with: {text: hi}`
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, t.Name()+"-valid.flow.yaml")
	tmp, err := os.Create(tmpPath)
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(valid); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	os.Args = []string{"flow", "lint", tmpPath}
	out := captureOutput(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
			t.Errorf("lint command failed: %v", err)
		}
	})
	t.Logf("lint output: %q", out)
	if !strings.Contains(out, "Lint OK") {
		t.Errorf("expected Lint OK, got %q", out)
	}

	os.Args = []string{"flow", "validate", tmpPath}
	out = captureOutput(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
			t.Errorf("validate command failed: %v", err)
		}
	})
	t.Logf("validate output: %q", out)
	if !strings.Contains(out, "Validation OK") {
		t.Errorf("expected Validation OK, got %q", out)
	}

	// Missing file
	os.Args = []string{"flow", "lint", "/nonexistent/file.yaml"}
	stderr, code := captureStderrExit(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	t.Logf("Missing file test - code: %d, stderr: %q", code, stderr)
	if code != 1 || !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("expected exit 1 and YAML parse error, got code=%d, stderr=%q", code, stderr)
	}

	// Invalid YAML (schema error, but valid YAML)
	bad := "name: bad\nsteps: [this is: not valid yaml]"
	tmpDir = t.TempDir()
	tmp2Path := filepath.Join(tmpDir, t.Name()+"-bad.flow.yaml")
	tmp2, err := os.Create(tmp2Path)
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp2Path)
	if _, err := tmp2.WriteString(bad); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp2.Close()
	os.Args = []string{"flow", "lint", tmp2Path}
	stderr, code = captureStderrExit(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if code != 2 || !strings.Contains(stderr, "Schema validation error") {
		t.Errorf("expected exit 2 and schema error, got code=%d, stderr=%q", code, stderr)
	}

	// Truly invalid YAML (parse error)
	badYAML := "name: bad\nsteps: [this is: not valid yaml"
	tmpDir = t.TempDir()
	tmp3Path := filepath.Join(tmpDir, t.Name()+"-bad2.flow.yaml")
	tmp3, err := os.Create(tmp3Path)
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp3Path)
	if _, err := tmp3.WriteString(badYAML); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp3.Close()
	os.Args = []string{"flow", "lint", tmp3Path}
	stderr, code = captureStderrExit(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if code != 1 || !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("expected exit 1 and YAML parse error, got code=%d, stderr=%q", code, stderr)
	}
}

func TestMain_ToolStub(t *testing.T) {
	// Test that unified commands work instead of the old tool subcommand
	os.Args = []string{"flow", "spec"}
	out := captureOutput(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if out == "" {
		t.Errorf("expected spec output, got empty string")
	}
}

func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd()

	if cmd.Use != "flow" {
		t.Errorf("Expected root command name 'flow', got %s", cmd.Use)
	}

	// Check that persistent flags are set
	flags := cmd.PersistentFlags()
	if flags.Lookup("config") == nil {
		t.Error("Expected --config flag to be defined")
	}
	if flags.Lookup("debug") == nil {
		t.Error("Expected --debug flag to be defined")
	}
	if flags.Lookup("mcp-timeout") == nil {
		t.Error("Expected --mcp-timeout flag to be defined")
	}
	if flags.Lookup("flows-dir") == nil {
		t.Error("Expected --flows-dir flag to be defined")
	}

	// Check that subcommands are added
	commands := cmd.Commands()
	expectedCommands := []string{"serve", "run", "mcp"}

	for _, expected := range expectedCommands {
		found := false
		for _, cmd := range commands {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command '%s' to be added", expected)
		}
	}
}

func TestNewServeCmd(t *testing.T) {
	cmd := newServeCmd()

	if cmd.Use != "serve" {
		t.Errorf("Expected command name 'serve', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty short description")
	}

	// Check that --addr flag is defined
	if cmd.Flags().Lookup("addr") == nil {
		t.Error("Expected --addr flag to be defined")
	}
}

func TestNewRunCmd(t *testing.T) {
	cmd := newRunCmd()

	if cmd.Use != "run [file]" {
		t.Errorf("Expected command use 'run [file]', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty short description")
	}

	// Check that flags are defined
	if cmd.Flags().Lookup("event") == nil {
		t.Error("Expected --event flag to be defined")
	}
	if cmd.Flags().Lookup("event-json") == nil {
		t.Error("Expected --event-json flag to be defined")
	}
}

func TestRunFlowExecution_NoArgs(t *testing.T) {
	out := captureOutput(func() {
		runFlowExecution(&cobra.Command{}, []string{}, "", "")
	})

	if !strings.Contains(out, "flow run") {
		t.Errorf("Expected stub output, got: %s", out)
	}
}

func TestRunFlowExecution_WithFile(t *testing.T) {
	// Create a temporary flow file
	tmpDir := t.TempDir()
	flowPath := filepath.Join(tmpDir, "test.flow.yaml")
	flowContent := `name: test
on: cli.manual
steps:
  - id: echo_step
    use: core.echo
    with:
      text: "hello world"
`
	if err := os.WriteFile(flowPath, []byte(flowContent), 0644); err != nil {
		t.Fatalf("Failed to write test flow: %v", err)
	}

	// Test with valid flow file
	stderr, code := captureStderrExit(func() {
		runFlowExecution(&cobra.Command{}, []string{flowPath}, "", "")
	})

	// Should execute successfully or give a reasonable error
	t.Logf("Flow execution - code: %d, stderr: %s", code, stderr)
}

func TestRunFlowExecution_InvalidFile(t *testing.T) {
	stderr, code := captureStderrExit(func() {
		runFlowExecution(&cobra.Command{}, []string{"/nonexistent/file.yaml"}, "", "")
	})

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}

	if !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("Expected YAML parse error, got: %s", stderr)
	}
}

func TestLoadEvent(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		inline   string
		wantErr  bool
		expected map[string]any
	}{
		{
			name:     "no event",
			path:     "",
			inline:   "",
			wantErr:  false,
			expected: map[string]any{},
		},
		{
			name:     "inline JSON",
			path:     "",
			inline:   `{"key": "value"}`,
			wantErr:  false,
			expected: map[string]any{"key": "value"},
		},
		{
			name:    "invalid inline JSON",
			path:    "",
			inline:  `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "non-existent file",
			path:    "/nonexistent/file.json",
			inline:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file if needed
			if tt.path != "" && !strings.Contains(tt.path, "nonexistent") {
				tmpDir := t.TempDir()
				tt.path = filepath.Join(tmpDir, "event.json")
				content := `{"file": "data"}`
				if err := os.WriteFile(tt.path, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
				tt.expected = map[string]any{"file": "data"}
			}

			result, err := loadEvent(tt.path, tt.inline)

			if (err != nil) != tt.wantErr {
				t.Errorf("loadEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result) != len(tt.expected) {
					t.Errorf("loadEvent() result length = %d, want %d", len(result), len(tt.expected))
				}
				for k, v := range tt.expected {
					if result[k] != v {
						t.Errorf("loadEvent() result[%s] = %v, want %v", k, result[k], v)
					}
				}
			}
		})
	}
}

func TestOutputFlowResults(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]any
		debug   bool
	}{
		{
			name:    "simple outputs",
			outputs: map[string]any{"step1": "result1"},
			debug:   false,
		},
		{
			name:    "debug mode",
			outputs: map[string]any{"step1": "result1"},
			debug:   true,
		},
		{
			name:    "empty outputs",
			outputs: map[string]any{},
			debug:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origDebug := debug
			debug = tt.debug
			defer func() { debug = origDebug }()

			out := captureOutput(func() {
				outputFlowResults(tt.outputs)
			})

			// Should not panic and should produce some output
			t.Logf("Output: %s", out)
		})
	}
}

func TestOutputEchoResults(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]any
		check   func(output string) bool
	}{
		{
			name:    "echo step",
			outputs: map[string]any{"echo": map[string]any{"text": "hello world"}},
			check: func(output string) bool {
				return strings.Contains(output, "hello world")
			},
		},
		{
			name: "openai response",
			outputs: map[string]any{
				"chat": map[string]any{
					"choices": []interface{}{
						map[string]any{
							"message": map[string]any{
								"content": "AI response",
							},
						},
					},
				},
			},
			check: func(output string) bool {
				return strings.Contains(output, "🤖") && strings.Contains(output, "AI response")
			},
		},
		{
			name: "mcp response",
			outputs: map[string]any{
				"mcp": map[string]any{
					"content": []interface{}{
						map[string]any{
							"text": "MCP response",
						},
					},
				},
			},
			check: func(output string) bool {
				return strings.Contains(output, "📡") && strings.Contains(output, "MCP response")
			},
		},
		{
			name: "http response",
			outputs: map[string]any{
				"http": map[string]any{
					"body": "HTTP response body",
				},
			},
			check: func(output string) bool {
				return strings.Contains(output, "🌐") && strings.Contains(output, "HTTP response")
			},
		},
		{
			name:    "large output",
			outputs: map[string]any{"large": strings.Repeat("x", 2000)},
			check: func(output string) bool {
				return strings.Contains(output, "too large to display")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureOutput(func() {
				outputEchoResults(tt.outputs)
			})

			if tt.check != nil && !tt.check(out) {
				t.Errorf("Output check failed for %s: %s", tt.name, out)
			}
		})
	}
}

func TestOutputDebugResults(t *testing.T) {
	outputs := map[string]any{"step1": "result1", "step2": map[string]any{"key": "value"}}

	out := captureOutput(func() {
		outputDebugResults(outputs)
	})

	// Should contain JSON output
	if !strings.Contains(out, "step1") || !strings.Contains(out, "step2") {
		t.Errorf("Expected JSON output with step names, got: %s", out)
	}
}

func TestNewMCPCmd(t *testing.T) {
	cmd := newMCPCmd()

	if cmd.Use != "mcp" {
		t.Errorf("Expected command name 'mcp', got %s", cmd.Use)
	}

	// Check subcommands
	commands := cmd.Commands()
	expectedSubcommands := []string{"serve", "search", "install", "list"}

	for _, expected := range expectedSubcommands {
		found := false
		for _, subcmd := range commands {
			if subcmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected MCP subcommand '%s' to be added", expected)
		}
	}
}

func TestNewMCPSearchCmd(t *testing.T) {
	cmd := newMCPSearchCmd()

	if cmd.Use != "search [query]" {
		t.Errorf("Expected command use 'search [query]', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty short description")
	}
}

func TestNewMCPInstallCmd(t *testing.T) {
	configFile := "test-config.json"
	cmd := newMCPInstallCmd(&configFile)

	if cmd.Use != "install <serverName>" {
		t.Errorf("Expected command use 'install <serverName>', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty short description")
	}
}

func TestNewMCPListCmd(t *testing.T) {
	configFile := "test-config.json"
	cmd := newMCPListCmd(&configFile)

	if cmd.Use != "list" {
		t.Errorf("Expected command use 'list', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty short description")
	}
}

func TestNewMCPServeCmd(t *testing.T) {
	cmd := newMCPServeCmd()

	if cmd.Use != "serve" {
		t.Errorf("Expected command use 'serve', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected non-empty short description")
	}
}

func TestLoadConfigAsMap(t *testing.T) {
	tests := []struct {
		name       string
		createFile bool
		content    string
		wantErr    bool
	}{
		{
			name:       "non-existent file",
			createFile: false,
			wantErr:    false, // Should return empty map
		},
		{
			name:       "valid JSON",
			createFile: true,
			content:    `{"key": "value"}`,
			wantErr:    false,
		},
		{
			name:       "invalid JSON",
			createFile: true,
			content:    `{invalid json}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")

			if tt.createFile {
				if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to write test config: %v", err)
				}
			}

			result, err := loadConfigAsMap(configPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfigAsMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("loadConfigAsMap() returned nil result")
			}
		})
	}
}

func TestEnsureMCPServersMap(t *testing.T) {
	tests := []struct {
		name string
		doc  map[string]any
		want map[string]any
	}{
		{
			name: "existing mcpServers",
			doc:  map[string]any{"mcp_servers": map[string]any{"server1": "config1"}},
			want: map[string]any{"server1": "config1"},
		},
		{
			name: "no mcpServers",
			doc:  map[string]any{"other": "value"},
			want: map[string]any{},
		},
		{
			name: "invalid mcpServers type",
			doc:  map[string]any{"mcp_servers": "not a map"},
			want: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureMCPServersMap(tt.doc)

			if len(result) != len(tt.want) {
				t.Errorf("ensureMCPServersMap() length = %d, want %d", len(result), len(tt.want))
			}

			for k, v := range tt.want {
				if result[k] != v {
					t.Errorf("ensureMCPServersMap()[%s] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestWriteConfigMap(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	doc := map[string]any{
		"key1": "value1",
		"key2": map[string]any{"nested": "value"},
	}

	err := writeConfigMap(doc, configPath)
	if err != nil {
		t.Errorf("writeConfigMap() error = %v", err)
		return
	}

	// Verify file was written
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("writeConfigMap() did not create config file")
		return
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("Failed to read written config: %v", err)
		return
	}

	var result map[string]any
	if err := json.Unmarshal(content, &result); err != nil {
		t.Errorf("Written config is not valid JSON: %v", err)
		return
	}

	if result["key1"] != "value1" {
		t.Errorf("Written config key1 = %v, want 'value1'", result["key1"])
	}
}

func TestMain(m *testing.M) {
	utils.WithCleanDirs(m, ".beemflow", config.DefaultConfigDir)
}
