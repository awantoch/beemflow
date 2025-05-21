package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/utils"

	"github.com/awantoch/beemflow/api"
	mcpserver "github.com/awantoch/beemflow/mcp"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	mcpstdio "github.com/metoro-io/mcp-golang/transport/stdio"
)

// startMCPServer starts an in-process MCP server over stdio pipes and returns a client and cancel function.
func startMCPServer(t *testing.T) (*mcp.Client, context.CancelFunc) {
	// Create in-process stdio pipes
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	// Server transport and client transport
	serverTransport := mcpstdio.NewStdioServerTransportWithIO(serverReader, serverWriter)
	clientTransport := mcpstdio.NewStdioServerTransportWithIO(clientReader, clientWriter)
	// Create and register server
	server := mcp.NewServer(serverTransport)
	mcpserver.RegisterAllTools(server, buildMCPToolRegistrations())
	// Start server in background
	go func() {
		if err := server.Serve(); err != nil {
			t.Error("MCP server Serve failed:", err)
		}
	}()
	// Initialize client
	client := mcp.NewClient(clientTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := client.Initialize(ctx); err != nil {
		cancel()
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}
	return client, cancel
}

func TestMCPServer_ListTools(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	resp, err := client.ListTools(ctx, new(string))
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(resp.Tools) == 0 {
		t.Fatalf("Expected at least one tool, got none")
	}
}

func TestMCPServer_ListFlows(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	resp, err := client.CallTool(ctx, "listFlows", struct{}{})
	if err != nil {
		t.Fatalf("listFlows failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from listFlows")
	}
}

func TestMCPServer_GetFlow_Nonexistent(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	// Request a non-existent flow: should succeed with empty content
	params := struct{ Name string }{"nonexistent-flow"}
	resp, err := client.CallTool(ctx, "getFlow", params)
	if err != nil {
		t.Fatalf("getFlow failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response for nonexistent flow, got %+v", resp)
	}
}

func TestMCPServer_StartRun_Nonexistent(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	// Non-existent flow: should succeed with runID=nil
	params := struct {
		FlowName string
		Event    map[string]any
	}{"nonexistent-flow", map[string]any{}}
	resp, err := client.CallTool(ctx, "startRun", params)
	if err != nil {
		t.Fatalf("startRun failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response for startRun, got %+v", resp)
	}
}

func TestMCPServer_ValidateFlow(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	params := struct{ Name string }{"nonexistent-flow"}
	resp, err := client.CallTool(ctx, "validateFlow", params)
	if err != nil {
		t.Fatalf("validateFlow failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from validateFlow")
	}
}

func TestMCPServer_GraphFlow(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	params := struct{ Name string }{"dummy-flow"}
	resp, err := client.CallTool(ctx, "graphFlow", params)
	if err != nil {
		t.Fatalf("graphFlow failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from graphFlow")
	}
}

func TestMCPServer_GetRun_Nonexistent(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	params := struct{ RunID string }{"00000000-0000-0000-0000-000000000000"}
	resp, err := client.CallTool(ctx, "getRun", params)
	if err != nil {
		t.Fatalf("getRun failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from getRun")
	}
}

func TestMCPServer_PublishEvent(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	params := struct {
		Topic   string
		Payload map[string]any
	}{"test-topic", map[string]any{"foo": "bar"}}
	resp, err := client.CallTool(ctx, "publishEvent", params)
	if err != nil {
		t.Fatalf("publishEvent failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from publishEvent")
	}
}

func TestMCPServer_HappyPath_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	flowsDir := filepath.Join(tmpDir, config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	api.SetFlowsDir(flowsDir)
	flowYAML := `name: testflow
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: "hello"
`
	flowPath := filepath.Join(flowsDir, "testflow.flow.yaml")
	if err := os.WriteFile(flowPath, []byte(flowYAML), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	schemaPath := filepath.Join(tmpDir, t.Name()+"-beemflow.schema.json")
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	// Set working dir to tmpDir for this test
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()

	// getFlow
	getFlowParams := struct{ Name string }{"testflow"}
	resp, err := client.CallTool(ctx, "getFlow", getFlowParams)
	if err != nil {
		t.Fatalf("getFlow failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from getFlow")
	}
	contentStr := ""
	if resp.Content[0] != nil {
		b, _ := json.Marshal(resp.Content[0])
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err == nil {
			if txt, ok := m["text"].(string); ok {
				contentStr = txt
			}
		}
	}
	if !strings.Contains(contentStr, "testflow") {
		t.Errorf("getFlow response missing flow name: %v", contentStr)
	}

	// startRun
	startRunParams := struct {
		FlowName string
		Event    map[string]any
	}{"testflow", map[string]any{"foo": "bar"}}
	resp, err = client.CallTool(ctx, "startRun", startRunParams)
	if err != nil {
		t.Fatalf("startRun failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected runID in startRun response, got: %v", resp)
	}
	contentStr = ""
	if resp.Content[0] != nil {
		b, _ := json.Marshal(resp.Content[0])
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err == nil {
			if txt, ok := m["text"].(string); ok {
				contentStr = txt
			}
		}
	}
	if !strings.Contains(contentStr, "runID") {
		t.Fatalf("Expected runID in startRun response, got: %v", contentStr)
	}
	// Extract runID
	type runIDResp struct {
		RunID string `json:"runID"`
	}
	var rid runIDResp
	if err := json.Unmarshal([]byte(contentStr), &rid); err != nil {
		t.Fatalf("failed to parse runID: %v", err)
	}
	if rid.RunID == "" || rid.RunID == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("invalid runID returned: %v", rid.RunID)
	}

	// getRun
	getRunParams := struct{ RunID string }{rid.RunID}
	resp, err = client.CallTool(ctx, "getRun", getRunParams)
	if err != nil {
		t.Fatalf("getRun failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		// Allow run to be nil if completed
		return
	}
	if resp.Content[0] == nil {
		// Allow run to be nil if completed
		return
	}
	b, _ := json.Marshal(resp.Content[0])
	utils.Debug("getRun resp.Content[0] marshaled: %q", string(b))
	if contentStr == "null" {
		// Allow run to be nil if completed
		return
	}
	if !strings.Contains(contentStr, rid.RunID) {
		t.Fatalf("getRun response missing runID: %v", contentStr)
	}
}

func TestMCPServer_HappyPath_HTTP(t *testing.T) {
	tmpDir := t.TempDir()
	flowsDir := filepath.Join(tmpDir, config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	api.SetFlowsDir(flowsDir)
	flowYAML := `name: testflow
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: "hello"
`
	flowPath := filepath.Join(flowsDir, "testflow.flow.yaml")
	if err := os.WriteFile(flowPath, []byte(flowYAML), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	schemaPath := filepath.Join(tmpDir, t.Name()+"-beemflow.schema.json")
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	// Set working dir to tmpDir for this test
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	// Debug: print working dir and files
	wd, _ := os.Getwd()
	utils.Debug("working dir: %s", wd)
	files, _ := os.ReadDir("flows")
	for _, f := range files {
		utils.Debug(config.DefaultFlowsDir+"/ contains: %s", f.Name())
	}

	// Pick a random available port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	// Start MCP HTTP server in background
	serverTransport := mcphttp.NewHTTPTransport("/mcp").WithAddr(addr)
	server := mcp.NewServer(serverTransport)
	mcpserver.RegisterAllTools(server, buildMCPToolRegistrations())
	serverDone := make(chan struct{})
	go func() {
		err := server.Serve()
		if err != nil {
			t.Errorf("server.Serve failed: %v", err)
		}
		close(serverDone)
	}()
	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Create MCP HTTP client
	clientTransport := mcphttp.NewHTTPClientTransport("/mcp").WithBaseURL("http://" + addr)
	client := mcp.NewClient(clientTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize MCP HTTP client: %v", err)
	}

	// getFlow
	getFlowParams := struct{ Name string }{"testflow"}
	resp, err := client.CallTool(ctx, "getFlow", getFlowParams)
	if err != nil {
		t.Fatalf("getFlow failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected non-nil response from getFlow")
	}
	contentStr := ""
	if resp.Content[0] != nil {
		b, _ := json.Marshal(resp.Content[0])
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err == nil {
			if txt, ok := m["text"].(string); ok {
				contentStr = txt
			}
		}
	}
	if !strings.Contains(contentStr, "testflow") {
		t.Errorf("getFlow response missing flow name: %v", contentStr)
	}

	// startRun
	startRunParams := struct {
		FlowName string
		Event    map[string]any
	}{"testflow", map[string]any{"foo": "bar"}}
	resp, err = client.CallTool(ctx, "startRun", startRunParams)
	if err != nil {
		t.Fatalf("startRun failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("Expected runID in startRun response, got: %v", resp)
	}
	contentStr = ""
	if resp.Content[0] != nil {
		b, _ := json.Marshal(resp.Content[0])
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err == nil {
			if txt, ok := m["text"].(string); ok {
				contentStr = txt
			}
		}
	}
	if !strings.Contains(contentStr, "runID") {
		t.Fatalf("Expected runID in startRun response, got: %v", contentStr)
	}
	// Extract runID
	type runIDResp struct {
		RunID string `json:"runID"`
	}
	var rid runIDResp
	if err := json.Unmarshal([]byte(contentStr), &rid); err != nil {
		t.Fatalf("failed to parse runID: %v", err)
	}
	if rid.RunID == "" || rid.RunID == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("invalid runID returned: %v", rid.RunID)
	}

	// getRun
	getRunParams := struct{ RunID string }{rid.RunID}
	resp, err = client.CallTool(ctx, "getRun", getRunParams)
	if err != nil {
		t.Fatalf("getRun failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		// Allow run to be nil if completed
		return
	}
	if resp.Content[0] == nil {
		// Allow run to be nil if completed
		return
	}
	b, _ := json.Marshal(resp.Content[0])
	if string(b) == "null" {
		// Allow run to be nil if completed
		return
	}
	contentStr = ""
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err == nil {
		if txt, ok := m["text"].(string); ok {
			contentStr = txt
		}
	}
	if contentStr == "null" {
		// Allow run to be nil if completed
		return
	}
	if !strings.Contains(contentStr, rid.RunID) {
		t.Fatalf("getRun response missing runID: %v", contentStr)
	}
}

func TestMCPServer_HTTP_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	flowsDir := filepath.Join(tmpDir, config.DefaultFlowsDir)
	if err := os.MkdirAll(flowsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll failed: %v", err)
	}
	// Write a valid flow for later
	flowYAML := `name: testflow
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with:
      text: "hello"
`
	flowPath := filepath.Join(flowsDir, t.Name()+"-testflow.flow.yaml")
	if err := os.WriteFile(flowPath, []byte(flowYAML), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	schemaPath := filepath.Join(tmpDir, t.Name()+"-beemflow.schema.json")
	if err := os.WriteFile(schemaPath, []byte(schema), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	serverTransport := mcphttp.NewHTTPTransport("/mcp").WithAddr(addr)
	server := mcp.NewServer(serverTransport)
	mcpserver.RegisterAllTools(server, buildMCPToolRegistrations())
	go func() {
		err := server.Serve()
		if err != nil {
			t.Errorf("server.Serve failed: %v", err)
		}
	}()
	time.Sleep(200 * time.Millisecond)
	clientTransport := mcphttp.NewHTTPClientTransport("/mcp").WithBaseURL("http://" + addr)
	client := mcp.NewClient(clientTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize MCP HTTP client: %v", err)
	}

	// (1) getFlow with non-existent flow
	params := struct{ Name string }{"nope"}
	resp, err := client.CallTool(ctx, "getFlow", params)
	if err == nil && (resp == nil || len(resp.Content) == 0) {
		t.Errorf("expected error or non-empty response for missing flow")
	}

	// (1b) startRun with non-existent flow
	srParams := struct {
		FlowName string
		Event    map[string]any
	}{"nope", map[string]any{}}
	resp, err = client.CallTool(ctx, "startRun", srParams)
	if err == nil && (resp == nil || len(resp.Content) == 0) {
		t.Errorf("expected error or non-empty response for missing flow in startRun")
	}

	// (2) startRun with invalid YAML flow
	badYAML := "not: [valid: yaml"
	badPath := filepath.Join(flowsDir, t.Name()+"-bad.flow.yaml")
	if err := os.WriteFile(badPath, []byte(badYAML), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}
	srParams = struct {
		FlowName string
		Event    map[string]any
	}{"bad", map[string]any{}}
	resp, err = client.CallTool(ctx, "startRun", srParams)
	if err == nil && (resp == nil || len(resp.Content) == 0) {
		t.Errorf("expected error or non-empty response for invalid YAML in startRun")
	}

	// (3) malformed tool call (missing params)
	resp, err = client.CallTool(ctx, "getFlow", struct{}{})
	utils.Debug("malformed tool call resp: %+v, err: %v", resp, err)
	if resp != nil && len(resp.Content) > 0 {
		b, _ := json.Marshal(resp.Content[0])
		utils.Debug("malformed tool call resp.Content[0] JSON: %s", string(b))
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err == nil {
			if txt, ok := m["text"].(string); ok {
				if txt == "{\"name\":\"\",\"on\":null,\"steps\":null}" {
					// Accept default/empty flow response
					return
				}
			}
		}
	}
	if err == nil {
		// Check if the response contains an error message
		foundErr := false
		if resp != nil && len(resp.Content) > 0 {
			b, _ := json.Marshal(resp.Content[0])
			var m map[string]interface{}
			if err := json.Unmarshal(b, &m); err == nil {
				if txt, ok := m["text"].(string); ok && strings.Contains(txt, "error") {
					foundErr = true
				}
			}
		}
		if !foundErr {
			t.Errorf("expected error for malformed tool call (missing params) or default empty flow")
		}
	}
}

// TestMCPServer_ListFlows_CustomDir ensures the MCP server honors api.SetFlowsDir override.
func TestMCPServer_ListFlows_CustomDir(t *testing.T) {
	tmpDir := t.TempDir()
	custom := filepath.Join(tmpDir, "altflows")
	if err := os.MkdirAll(custom, 0755); err != nil {
		t.Fatalf("failed to create custom flows dir: %v", err)
	}
	// Write a sample flow file
	yaml := `name: altflow
on: cli.manual
steps: []`
	if err := os.WriteFile(filepath.Join(custom, "altflow.flow.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write flow YAML: %v", err)
	}
	// Override the API flowsDir
	api.SetFlowsDir(custom)
	defer api.SetFlowsDir("flows")

	// Start an in-memory MCP server and call listFlows
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()
	resp, err := client.CallTool(ctx, "listFlows", struct{}{})
	if err != nil {
		t.Fatalf("listFlows failed: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("expected non-empty response, got: %v", resp)
	}

	// Extract the 'text' field from the first content element
	var contentMap map[string]interface{}
	b, _ := json.Marshal(resp.Content[0])
	if err := json.Unmarshal(b, &contentMap); err != nil {
		t.Fatalf("failed to marshal content: %v", err)
	}
	textVal, ok := contentMap["text"].(string)
	if !ok {
		t.Fatalf("expected 'text' field in content map, got: %v", contentMap)
	}

	// Parse the JSON payload inside the text
	var out struct {
		Flows []string `json:"flows"`
	}
	if err := json.Unmarshal([]byte(textVal), &out); err != nil {
		t.Fatalf("failed to unmarshal flows JSON: %v", err)
	}
	if len(out.Flows) != 1 || out.Flows[0] != "altflow" {
		t.Errorf("expected [altflow], got %v", out.Flows)
	}
}

func TestMCPServer_DescribeTool_TypeFiltering(t *testing.T) {
	client, cancel := startMCPServer(t)
	defer cancel()
	ctx := context.Background()

	types := []string{"mcp", "http", "cli", ""}
	for _, typ := range types {
		params := struct{ Type string }{typ}
		resp, err := client.CallTool(ctx, "describe", params)
		if err != nil {
			t.Fatalf("describe failed for type %q: %v", typ, err)
		}
		if resp == nil || len(resp.Content) == 0 {
			t.Fatalf("Expected non-nil response from describe for type %q", typ)
		}
		var contentMap map[string]interface{}
		b, _ := json.Marshal(resp.Content[0])
		if err := json.Unmarshal(b, &contentMap); err != nil {
			t.Fatalf("failed to marshal content: %v", err)
		}
		textVal, ok := contentMap["text"].(string)
		if !ok {
			t.Fatalf("expected 'text' field in content map, got: %v", contentMap)
		}
		var out []map[string]interface{}
		if err := json.Unmarshal([]byte(textVal), &out); err != nil {
			t.Fatalf("failed to unmarshal describe JSON: %v", err)
		}
		for _, entry := range out {
			if typ != "" && entry["type"] != typ {
				t.Errorf("describe returned entry with type %v, expected %v", entry["type"], typ)
			}
		}
	}
}

// Add more tests for other handlers and edge cases as needed
