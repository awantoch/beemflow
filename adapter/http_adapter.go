package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/awantoch/beemflow/logger"
	"github.com/awantoch/beemflow/registry"
)

// defaultClient is used for HTTP requests with a timeout to avoid hanging.
var defaultClient = &http.Client{Timeout: 30 * time.Second}

// HTTPPostJSON marshals body as JSON, sends it, and decodes the JSON response into result.
func HTTPPostJSON(ctx context.Context, url string, body interface{}, headers map[string]string, result interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if _, ok := headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := defaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return logger.Errorf("HTTPPostJSON: unexpected status code %d: %s", resp.StatusCode, string(data))
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return logger.Errorf("failed to decode JSON from %s: %w", url, err)
		}
	}
	return nil
}

// HTTPGetRaw performs an HTTP GET and returns the raw response body as a string.
func HTTPGetRaw(ctx context.Context, url string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := defaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", logger.Errorf("HTTPGetRaw: unexpected status code %d: %s", resp.StatusCode, string(data))
	}
	return string(data), nil
}

// HTTPAdapter is a generic HTTP-backed tool adapter.
type HTTPAdapter struct {
	AdapterID    string
	ToolManifest *registry.ToolManifest
}

// ID returns the unique identifier of the adapter.
func (a *HTTPAdapter) ID() string {
	return a.AdapterID
}

// injectDefaults merges manifest defaults into inputs for any missing fields.
func injectDefaults(params map[string]any, inputs map[string]any) {
	props, ok := params["properties"].(map[string]any)
	if !ok {
		return
	}
	for k, v := range props {
		prop, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if _, present := inputs[k]; !present {
			if def, hasDefault := prop["default"]; hasDefault {
				inputs[k] = def
			}
		}
	}
}

// Execute calls the manifest's endpoint with JSON inputs and returns parsed JSON output.
func (a *HTTPAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Fallback to generic HTTP fetch if no endpoint is defined (e.g., http.fetch)
	if a.ToolManifest == nil {
		return nil, logger.Errorf("no manifest for tool %s", a.AdapterID)
	}
	if a.ToolManifest.Endpoint == "" {
		// Use HTTPFetchAdapter for endpoints without a static manifest-endpoint
		var fetchAdapter HTTPFetchAdapter
		return fetchAdapter.Execute(ctx, inputs)
	}
	// Inject manifest defaults for missing fields
	if a.ToolManifest.Parameters != nil {
		injectDefaults(a.ToolManifest.Parameters, inputs)
	}
	// Merge headers: manifest headers (with $env expansion) + step input headers (step input wins)
	headers := map[string]string{}
	if a.ToolManifest.Headers != nil {
		for k, v := range expandEnvHeaders(a.ToolManifest.Headers) {
			headers[k] = v
		}
	}
	if h, ok := inputs["headers"].(map[string]any); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}
	var out map[string]any
	err := HTTPPostJSON(ctx, a.ToolManifest.Endpoint, inputs, headers, &out)
	return out, err
}

// HTTPFetchAdapter implements Adapter for HTTP requests, supporting GET/POST/PUT/etc.
type HTTPFetchAdapter struct{}

// ID returns the unique identifier of the HTTP request adapter.
func (a *HTTPFetchAdapter) ID() string {
	return "http"
}

// Execute performs an HTTP request on the given URL with optional method, headers, and body, and returns the response body.
func (a *HTTPFetchAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, logger.Errorf("missing url")
	}
	// Determine method
	method := "GET"
	if m, ok := inputs["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}
	// Collect headers
	headers := make(map[string]string)
	if h, ok := inputs["headers"].(map[string]any); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}
	// Execute request
	switch method {
	case "GET":
		// Perform GET, then try to unmarshal JSON
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Accept", "application/json, text/*;q=0.9, */*;q=0.8")
		resp, err := defaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, logger.Errorf("HTTP GET %s: status %d: %s", url, resp.StatusCode, string(data))
		}
		// Try JSON unmarshal
		var parsed any
		if err := json.Unmarshal(data, &parsed); err == nil {
			return map[string]any{"body": parsed}, nil
		}
		// Fallback to raw string
		return map[string]any{"body": string(data)}, nil
	case "POST", "PUT", "PATCH", "DELETE":
		// JSON body if provided
		var payload any
		if p, ok := inputs["body"]; ok {
			payload = p
		} else {
			payload = map[string]any{}
		}
		var out any
		err := HTTPPostJSON(ctx, url, payload, headers, &out)
		if err != nil {
			return nil, err
		}
		// Convert JSON result to raw string
		b, _ := json.Marshal(out)
		return map[string]any{"body": string(b)}, nil
	default:
		return nil, logger.Errorf("unsupported method %s", method)
	}
}

func (a *HTTPFetchAdapter) Manifest() *registry.ToolManifest {
	return nil
}

func (a *HTTPAdapter) Manifest() *registry.ToolManifest {
	return a.ToolManifest
}

func expandEnvHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		newVal := v
		for {
			start := strings.Index(newVal, "$env:")
			if start == -1 {
				break
			}
			end := start + 5
			for end < len(newVal) && (newVal[end] == '_' || newVal[end] == '-' || (newVal[end] >= 'A' && newVal[end] <= 'Z') || (newVal[end] >= 'a' && newVal[end] <= 'z') || (newVal[end] >= '0' && newVal[end] <= '9')) {
				end++
			}
			varName := newVal[start+5 : end]
			envVal := os.Getenv(varName)
			newVal = newVal[:start] + envVal + newVal[end:]
		}
		out[k] = newVal
	}
	return out
}
