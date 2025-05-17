package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type MCPClient interface {
	ListTools() ([]Tool, error)
	CallTool(name string, args map[string]any) (map[string]any, error)
}

// HTTPMCPClient implements MCPClient over HTTP transport
// (e.g. for Node.js MCP servers)
type HTTPMCPClient struct {
	BaseURL string
	tools   []Tool
}

func NewHTTPMCPClient(baseURL string) *HTTPMCPClient {
	return &HTTPMCPClient{BaseURL: baseURL}
}

func (c *HTTPMCPClient) ListTools() ([]Tool, error) {
	// POST { method: "tools/list" }
	body := map[string]any{"method": "tools/list"}
	b, _ := json.Marshal(body)
	resp, err := http.Post(c.BaseURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	c.tools = result.Tools
	return result.Tools, nil
}

func (c *HTTPMCPClient) CallTool(name string, args map[string]any) (map[string]any, error) {
	body := map[string]any{
		"method": "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(c.BaseURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	var result struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

// StdioMCPClient is a stub for stdio transport (to be implemented)
type StdioMCPClient struct{}

// NOTE: Only HTTP MCP servers are currently supported. Stdio transport is a placeholder for future work.

func NewStdioMCPClient(cmd string, args ...string) *StdioMCPClient {
	// TODO: implement stdio transport
	return &StdioMCPClient{}
}

func (c *StdioMCPClient) ListTools() ([]Tool, error) {
	return nil, fmt.Errorf("StdioMCPClient not implemented yet")
}

func (c *StdioMCPClient) CallTool(name string, args map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("StdioMCPClient not implemented yet")
}
