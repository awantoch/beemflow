package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// smitheryListResp represents the response schema for listing servers from Smithery
type smitheryListResp struct {
	Servers []struct {
		QualifiedName string `json:"qualifiedName"`
		DisplayName   string `json:"displayName"`
		Description   string `json:"description"`
		Homepage      string `json:"homepage"`
		IsDeployed    bool   `json:"isDeployed"`
		CreatedAt     string `json:"createdAt"`
	} `json:"servers"`
}

// newMCPSmitherySearchCmd creates the 'mcp search' subcommand to discover servers via Smithery.
func newMCPSmitherySearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search [query]",
		Short: "Search for MCP servers in the Smithery registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			// Build the URL with optional query parameter
			base := "https://registry.smithery.ai/servers"
			params := url.Values{}
			params.Set("pageSize", "50")
			if query != "" {
				params.Set("q", query)
			}
			endpoint := fmt.Sprintf("%s?%s", base, params.Encode())

			req, err := http.NewRequest("GET", endpoint, nil)
			if err != nil {
				return err
			}
			// Require API key from environment
			apiKey := os.Getenv("SMITHERY_API_KEY")
			if apiKey == "" {
				return fmt.Errorf("environment variable SMITHERY_API_KEY must be set")
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("Smithery registry returned status %s", resp.Status)
			}

			var data smitheryListResp
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				return err
			}

			// Print results in a table
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME	DISPLAY_NAME	DESCRIPTION	HOMEPAGE")
			for _, s := range data.Servers {
				fmt.Fprintf(w, "%s	%s	%s	%s\n", s.QualifiedName, s.DisplayName, s.Description, s.Homepage)
			}
			w.Flush()
			return nil
		},
	}
}

// newMCPSmitheryInstallCmd creates the 'mcp install' subcommand to install an MCP server from the Smithery registry.
func newMCPSmitheryInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <serverName>",
		Short: "Install an MCP server from the Smithery registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qn := args[0]
			endpoint := fmt.Sprintf("https://registry.smithery.ai/servers/%s", url.PathEscape(qn))
			req, err := http.NewRequest("GET", endpoint, nil)
			if err != nil {
				return err
			}
			apiKey := os.Getenv("SMITHERY_API_KEY")
			if apiKey == "" {
				return fmt.Errorf("environment variable SMITHERY_API_KEY must be set")
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("Smithery registry returned status %s", resp.Status)
			}
			var data struct {
				QualifiedName string `json:"qualifiedName"`
				Connections   []struct {
					Type          string         `json:"type"`
					ConfigSchema  map[string]any `json:"configSchema"`
					Published     bool           `json:"published"`
					StdioFunction string         `json:"stdioFunction"`
				} `json:"connections"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				return err
			}
			var connSpec struct {
				StdioFunction string
				Found         bool
			}
			for _, c := range data.Connections {
				if c.Type == "stdio" && c.Published {
					connSpec.StdioFunction = c.StdioFunction
					connSpec.Found = true
					break
				}
			}
			if !connSpec.Found {
				return fmt.Errorf("no stdio connection found for server %s", qn)
			}
			fn := connSpec.StdioFunction
			start := strings.Index(fn, "({")
			end := strings.LastIndex(fn, "})")
			if start < 0 || end < 0 || end <= start+1 {
				return fmt.Errorf("invalid stdioFunction format: %s", fn)
			}
			obj := fn[start+1 : end+1]
			// Replace single quotes with double quotes and wrap unquoted keys for valid JSON
			interim := strings.ReplaceAll(obj, "'", "\"")
			re := regexp.MustCompile(`(\w+)\s*:`)
			jsonObj := re.ReplaceAllString(interim, `"$1":`)
			// Parse JSON into map
			var m map[string]any
			if err := json.Unmarshal([]byte(jsonObj), &m); err != nil {
				return fmt.Errorf("failed to parse stdioFunction object: %w", err)
			}
			cmdVal, ok := m["command"].(string)
			if !ok {
				return fmt.Errorf("stdioFunction object missing command")
			}
			var argsList []string
			if arr, ok2 := m["args"].([]any); ok2 {
				for _, ai := range arr {
					if s, sok := ai.(string); sok {
						argsList = append(argsList, s)
					}
				}
			}
			cfgMap := map[string]map[string]any{
				data.QualifiedName: {
					"command": cmdVal,
					"args":    argsList,
				},
			}
			bytesOut, err := json.MarshalIndent(cfgMap, "", "  ")
			if err != nil {
				return err
			}
			filePath := filepath.Join("mcp_servers", data.QualifiedName+".json")
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filePath, bytesOut, 0644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Installed MCP server %s to %s\n", data.QualifiedName, filePath)
			return nil
		},
	}
}
