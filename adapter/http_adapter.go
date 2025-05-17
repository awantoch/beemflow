package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
		return fmt.Errorf("HTTPPostJSON: unexpected status code %d: %s", resp.StatusCode, string(data))
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("failed to decode JSON from %s: %w", url, err)
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
		return "", fmt.Errorf("HTTPGetRaw: unexpected status code %d: %s", resp.StatusCode, string(data))
	}
	return string(data), nil
}

// HTTPAdapter is a generic HTTP-backed tool adapter.
type HTTPAdapter struct {
	id       string
	manifest *ToolManifest
}

// ID returns the unique identifier of the adapter.
func (a *HTTPAdapter) ID() string {
	return a.id
}

// Execute calls the manifest's endpoint with JSON inputs and returns parsed JSON output.
func (a *HTTPAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if a.manifest == nil || a.manifest.Endpoint == "" {
		return nil, fmt.Errorf("no endpoint for tool %s", a.id)
	}
	var out map[string]any
	err := HTTPPostJSON(ctx, a.manifest.Endpoint, inputs, nil, &out)
	return out, err
}

// HTTPFetchAdapter implements Adapter for http.fetch, returning the response body.
type HTTPFetchAdapter struct{}

// ID returns the adapter ID.
func (a *HTTPFetchAdapter) ID() string {
	return "http.fetch"
}

// Execute performs an HTTP GET on the given URL with optional headers and returns the body.
func (a *HTTPFetchAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	url, ok := inputs["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("http.fetch: missing url")
	}
	headers := make(map[string]string)
	if h, ok := inputs["headers"].(map[string]any); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}
	body, err := HTTPGetRaw(ctx, url, headers)
	if err != nil {
		return nil, err
	}
	return map[string]any{"body": body}, nil
}

func (a *HTTPFetchAdapter) Manifest() *ToolManifest {
	return nil
}
