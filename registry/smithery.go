package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type SmitheryRegistry struct {
	APIKey  string
	BaseURL string // e.g. https://registry.smithery.ai/servers
}

func NewSmitheryRegistry(apiKey, baseURL string) *SmitheryRegistry {
	if baseURL == "" {
		baseURL = "https://registry.smithery.ai/servers"
	}
	return &SmitheryRegistry{APIKey: apiKey, BaseURL: baseURL}
}

func (s *SmitheryRegistry) ListServers(ctx context.Context, opts ListOptions) ([]RegistryEntry, error) {
	params := url.Values{}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	} else {
		params.Set("pageSize", "50")
	}
	if opts.Query != "" {
		params.Set("q", opts.Query)
	}
	endpoint := fmt.Sprintf("%s?%s", s.BaseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if s.APIKey == "" {
		s.APIKey = os.Getenv("SMITHERY_API_KEY")
	}
	if s.APIKey == "" {
		return nil, fmt.Errorf("Smithery API key not set")
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Smithery registry returned status %s", resp.Status)
	}
	var data struct {
		Servers []struct {
			QualifiedName string `json:"qualifiedName"`
			DisplayName   string `json:"displayName"`
			Description   string `json:"description"`
			Homepage      string `json:"homepage"`
			IsDeployed    bool   `json:"isDeployed"`
		} `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	var entries []RegistryEntry
	for _, s := range data.Servers {
		entries = append(entries, RegistryEntry{
			Type:        "mcp_server",
			Name:        s.QualifiedName,
			Description: s.Description,
			Kind:        "smithery",
			Endpoint:    s.Homepage,
		})
	}
	return entries, nil
}

func (s *SmitheryRegistry) GetServer(ctx context.Context, name string) (*RegistryEntry, error) {
	endpoint := fmt.Sprintf("%s/%s", strings.TrimSuffix(s.BaseURL, "/"), url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if s.APIKey == "" {
		s.APIKey = os.Getenv("SMITHERY_API_KEY")
	}
	if s.APIKey == "" {
		return nil, fmt.Errorf("Smithery API key not set")
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Smithery registry returned status %s", resp.Status)
	}
	var data struct {
		QualifiedName string `json:"qualifiedName"`
		DisplayName   string `json:"displayName"`
		Description   string `json:"description"`
		Homepage      string `json:"homepage"`
		Connections   []struct {
			Type          string         `json:"type"`
			Url           string         `json:"url"`
			ConfigSchema  map[string]any `json:"configSchema"`
			Published     bool           `json:"published"`
			StdioFunction string         `json:"stdioFunction"`
		} `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	// Find HTTP or stdio connection if available
	for _, conn := range data.Connections {
		if (conn.Type == "http" || conn.Type == "stdio") && conn.Url != "" {
			return &RegistryEntry{
				Type:        "mcp_server",
				Name:        data.QualifiedName,
				Description: data.Description,
				Kind:        conn.Type,
				Endpoint:    conn.Url,
				Parameters:  conn.ConfigSchema,
			}, nil
		}
	}
	return nil, fmt.Errorf("no suitable connection found for server %s", name)
}
