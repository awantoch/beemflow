package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// convertOpenAPISpec is a DRY helper that calls the core adapter for OpenAPI conversion
func convertOpenAPISpec(openapiData []byte, apiName, baseURL string) (map[string]any, error) {
	coreAdapter := &adapter.CoreAdapter{}
	inputs := map[string]any{
		"__use":    "core.convert_openapi",
		"openapi":  string(openapiData),
		"api_name": apiName,
		"base_url": baseURL,
	}
	return coreAdapter.Execute(context.Background(), inputs)
}

// newToolCmd creates the 'tool' subcommand and its scaffolding commands.
func newToolCmd() *cobra.Command {
	// Not implemented yet. Planned for a future release.
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Tooling commands",
		Run: func(cmd *cobra.Command, args []string) {
			utils.User("flow tool (stub)")
		},
	}

	// Create convert command with flags
	convertCmd := &cobra.Command{
		Use:   "convert [openapi_file]",
		Short: "Convert OpenAPI spec to BeemFlow tool manifests",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var openapiData []byte
			var err error

			// Read OpenAPI spec from file or stdin
			if len(args) > 0 {
				openapiData, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("failed to read OpenAPI file: %w", err)
				}
			} else {
				openapiData, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
			}

			// Get flags
			apiName, _ := cmd.Flags().GetString("api-name")
			baseURL, _ := cmd.Flags().GetString("base-url")
			output, _ := cmd.Flags().GetString("output")

			if apiName == "" {
				apiName = "api" // Default name
			}

			// Convert using the DRY helper
			result, err := convertOpenAPISpec(openapiData, apiName, baseURL)
			if err != nil {
				return fmt.Errorf("conversion failed: %w", err)
			}

			// Prepare result
			resultJSON, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal result: %w", err)
			}

			if output != "" {
				if err := os.WriteFile(output, resultJSON, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				utils.User("Converted OpenAPI spec to %s", output)
			} else {
				utils.User("%s", string(resultJSON))
			}

			return nil
		},
	}

	// Add flags to convert command
	convertCmd.Flags().String("api-name", "", "Name prefix for generated tools (default: api)")
	convertCmd.Flags().String("base-url", "", "Base URL override (default: extracted from spec)")
	convertCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "scaffold",
			Short: "Scaffold a tool manifest",
			// Not implemented yet. Planned for a future release.
			Run: func(cmd *cobra.Command, args []string) {
				utils.User("flow tool (stub)")
			},
		},
		convertCmd,
		&cobra.Command{
			Use:   "list",
			Short: "List all available tools",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				tools, err := api.ListTools(ctx)
				if err != nil {
					return err
				}
				// Header
				utils.User("NAME\tKIND\tDESCRIPTION\tENDPOINT")
				for _, t := range tools {
					name, _ := t["name"].(string)
					kind, _ := t["kind"].(string)
					desc, _ := t["description"].(string)
					endpoint, _ := t["endpoint"].(string)
					utils.User("%s\t%s\t%s\t%s", name, kind, desc, endpoint)
				}
				return nil
			},
		},
	)
	return cmd
}
