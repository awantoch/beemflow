package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	jsonnetfmt "github.com/google/go-jsonnet/formatter"
	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newFmtCmd creates the 'fmt' subcommand (like gofmt).
func newFmtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fmt [file]",
		Short: "Format a flow file in-place (YAML or Jsonnet)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := args[0]
			if err := runFmt(path); err != nil {
				utils.Error("fmt error: %v", err)
				exit(1)
			}
			utils.User("Formatted %s", path)
		},
	}
	return cmd
}

func runFmt(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	ext := filepath.Ext(path)
	var formatted string
	switch ext {
	case ".jsonnet", ".libsonnet":
		formatted, err = jsonnetfmt.Format(path, string(data), &jsonnetfmt.Options{})
		if err != nil {
			return fmt.Errorf("jsonnet format: %w", err)
		}
	case ".yaml", ".yml":
		var obj any
		if err := yaml.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("yaml parse: %w", err)
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(obj); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		formatted = buf.String()
	default:
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	return os.WriteFile(path, []byte(formatted), 0o644)
}