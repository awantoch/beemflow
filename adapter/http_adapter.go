package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
)

// defaultClient is used for HTTP requests with a timeout to avoid hanging.
var defaultClient = &http.Client{Timeout: 30 * time.Second}

// Environment variable pattern for safe parsing
var envVarPattern = regexp.MustCompile(`\$env:([A-Za-z_][A-Za-z0-9_]*)`)

// HTTPAdapter is a unified HTTP adapter that handles both manifest-based and generic HTTP requests.
type HTTPAdapter struct {
	AdapterID    string
	ToolManifest *registry.ToolManifest
}

// HTTPRequest represents a prepared HTTP request
type HTTPRequest struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string]string
}

// HTTPResponse represents a processed HTTP response
type HTTPResponse struct {
	StatusCode int
	Body       any
	Headers    map[string]string
}

// ID returns the unique identifier of the adapter.
func (a *HTTPAdapter) ID() string {
	return a.AdapterID
}

// Execute performs HTTP requests based on manifest or generic parameters.
func (a *HTTPAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Handle manifest-based requests
	if a.ToolManifest != nil && a.ToolManifest.Endpoint != "" {
		return a.executeManifestRequest(ctx, inputs)
	}

	// Handle generic HTTP requests
	return a.executeGenericRequest(ctx, inputs)
}

// executeManifestRequest handles requests with a predefined manifest
func (a *HTTPAdapter) executeManifestRequest(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Create a copy of inputs to avoid mutation
	enrichedInputs := a.enrichInputsWithDefaults(inputs)

	// Prepare headers
	headers := a.prepareManifestHeaders(enrichedInputs)

	// Create request
	req := HTTPRequest{
		Method:  "POST",
		URL:     a.ToolManifest.Endpoint,
		Headers: headers,
	}

	// Marshal body
	body, err := json.Marshal(enrichedInputs)
	if err != nil {
		return nil, utils.Errorf("failed to marshal request body: %w", err)
	}
	req.Body = body

	// Execute request
	return a.executeHTTPRequest(ctx, req)
}

// executeGenericRequest handles generic HTTP requests
func (a *HTTPAdapter) executeGenericRequest(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	url, ok := safeStringAssert(inputs["url"])
	if !ok || url == "" {
		return nil, utils.Errorf("missing or invalid url")
	}

	method := a.extractMethod(inputs)
	headers := a.extractHeaders(inputs)

	req := HTTPRequest{
		Method:  method,
		URL:     url,
		Headers: headers,
	}

	// Add body for non-GET requests
	if method != "GET" {
		if body := inputs["body"]; body != nil {
			bodyBytes, err := json.Marshal(body)
			if err != nil {
				return nil, utils.Errorf("failed to marshal request body: %w", err)
			}
			req.Body = bodyBytes
		}
	}

	return a.executeHTTPRequest(ctx, req)
}

// executeHTTPRequest executes an HTTP request and returns the response
func (a *HTTPAdapter) executeHTTPRequest(ctx context.Context, req HTTPRequest) (map[string]any, error) {
	// Create HTTP request
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, utils.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set default headers if not provided
	if req.Method != "GET" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", "application/json, text/*;q=0.9, */*;q=0.8")
	}

	// Execute request
	resp, err := defaultClient.Do(httpReq)
	if err != nil {
		return nil, utils.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Process response
	return a.processHTTPResponse(resp, req.Method, req.URL)
}

// processHTTPResponse processes an HTTP response and returns structured data
func (a *HTTPAdapter) processHTTPResponse(resp *http.Response, method, url string) (map[string]any, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, utils.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, utils.Errorf("HTTP %s %s: status %d: %s", method, url, resp.StatusCode, string(data))
	}

	// Try to parse as JSON first
	var parsed any
	if err := json.Unmarshal(data, &parsed); err == nil {
		// If it's a JSON object, return it directly for backward compatibility
		if obj, ok := parsed.(map[string]any); ok {
			return obj, nil
		}
		// For non-object JSON (arrays, primitives), wrap in body
		return map[string]any{"body": parsed}, nil
	}

	// Fallback to raw string wrapped in body
	return map[string]any{"body": string(data)}, nil
}

// enrichInputsWithDefaults creates a copy of inputs with defaults applied (no mutation)
func (a *HTTPAdapter) enrichInputsWithDefaults(inputs map[string]any) map[string]any {
	// Create a copy to avoid mutating the original
	enriched := make(map[string]any, len(inputs))
	maps.Copy(enriched, inputs)

	if a.ToolManifest.Parameters == nil {
		return enriched
	}

	props, ok := safeMapAssert(a.ToolManifest.Parameters["properties"])
	if !ok {
		return enriched
	}

	for k, v := range props {
		prop, ok := safeMapAssert(v)
		if !ok {
			continue
		}

		// Only apply default if key is not present
		if _, present := enriched[k]; !present {
			if def, hasDefault := prop["default"]; hasDefault {
				// Expand environment variables in default values if they're strings
				if defStr, ok := safeStringAssert(def); ok {
					enriched[k] = expandEnvValue(defStr)
				} else {
					enriched[k] = def
				}
			}
		}
	}

	return enriched
}

// prepareManifestHeaders prepares headers for manifest-based requests
func (a *HTTPAdapter) prepareManifestHeaders(inputs map[string]any) map[string]string {
	headers := make(map[string]string)

	// Add manifest headers with environment variable expansion
	if a.ToolManifest.Headers != nil {
		for k, v := range a.ToolManifest.Headers {
			headers[k] = expandEnvValue(v)
		}
	}

	// Override with input headers
	if h, ok := safeMapAssert(inputs["headers"]); ok {
		for k, v := range h {
			if s, ok := safeStringAssert(v); ok {
				headers[k] = s
			}
		}
	}

	return headers
}

// extractMethod extracts HTTP method from inputs with safe default
func (a *HTTPAdapter) extractMethod(inputs map[string]any) string {
	if m, ok := safeStringAssert(inputs["method"]); ok && m != "" {
		return strings.ToUpper(m)
	}
	return "GET"
}

// extractHeaders extracts headers from inputs safely
func (a *HTTPAdapter) extractHeaders(inputs map[string]any) map[string]string {
	headers := make(map[string]string)
	if h, ok := safeMapAssert(inputs["headers"]); ok {
		for k, v := range h {
			if s, ok := safeStringAssert(v); ok {
				headers[k] = s
			}
		}
	}
	return headers
}

// expandEnvValue expands environment variables in a value string using regex for safety
func expandEnvValue(value string) string {
	return envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name (everything after $env:)
		varName := match[5:] // Remove "$env:" prefix
		if envVal := os.Getenv(varName); envVal != "" {
			return envVal
		}
		return match // Keep original if env var not found
	})
}

func (a *HTTPAdapter) Manifest() *registry.ToolManifest {
	return a.ToolManifest
}

// Safe type assertion helpers to prevent panics
func safeStringAssert(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func safeMapAssert(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}
