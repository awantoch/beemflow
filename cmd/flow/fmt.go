package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awantoch/beemflow/convert"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newFmtCmd creates the 'fmt' subcommand for formatting flow files.
func newFmtCmd() *cobra.Command {
	var inPlace bool

	cmd := &cobra.Command{
		Use:   "fmt <file>",
		Short: "Format flow files (YAML or Jsonnet)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			
			// Determine file type and format accordingly
			ext := strings.ToLower(filepath.Ext(filePath))
			var formatted string
			var err error

			switch ext {
			case ".yaml", ".yml":
				formatted, err = formatYAML(filePath)
			case ".jsonnet", ".libsonnet":
				formatted, err = formatJsonnet(filePath)
			default:
				utils.Error("Unsupported file format: %s", ext)
				exit(1)
			}

			if err != nil {
				utils.Error("Formatting failed: %v", err)
				exit(1)
			}

			// Output result
			if inPlace {
				if err := os.WriteFile(filePath, []byte(formatted), 0644); err != nil {
					utils.Error("Failed to write formatted file: %v", err)
					exit(1)
				}
				utils.User("Formatted %s", filePath)
			} else {
				fmt.Print(formatted)
			}
		},
	}

	cmd.Flags().BoolVarP(&inPlace, "write", "w", false, "Write result to file instead of stdout")
	return cmd
}

// formatYAML formats a YAML file
func formatYAML(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Parse and re-marshal to format
	var yamlData any
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return "", fmt.Errorf("invalid YAML: %w", err)
	}

	formatted, err := yaml.Marshal(yamlData)
	if err != nil {
		return "", err
	}

	return string(formatted), nil
}

// formatJsonnet formats a Jsonnet file
func formatJsonnet(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// For Jsonnet formatting, we'll convert to YAML and back to get consistent formatting
	// This is a simple approach - a real implementation might use jsonnetfmt
	yamlStr, err := convert.JsonnetToYAML(data)
	if err != nil {
		return "", err
	}

	// Convert back to Jsonnet for consistent formatting
	return convert.YAMLToJsonnet([]byte(yamlStr))
}