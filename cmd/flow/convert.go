package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/awantoch/beemflow/convert"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newConvertCmd creates the 'convert' subcommand.
func newConvertCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "convert [input]",
		Short: "Convert flow definition between YAML and Jsonnet formats",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inPath := args[0]
			if outPath == "" {
				outPath = defaultOutPath(inPath)
			}

			if err := runConvert(inPath, outPath); err != nil {
				utils.Error("convert error: %v", err)
				exit(1)
			}
			utils.User("Converted %s → %s", inPath, outPath)
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "Output file (default: switch extension)")
	return cmd
}

func runConvert(inPath, outPath string) error {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}

	ext := filepath.Ext(inPath)
	switch ext {
	case ".yaml", ".yml":
		jsonnetStr, err := convert.YAMLToJsonnet(data)
		if err != nil {
			return err
		}
		return os.WriteFile(outPath, []byte(jsonnetStr), 0o644)
	case ".jsonnet", ".libsonnet":
		yamlStr, err := convert.JsonnetToYAML(data)
		if err != nil {
			return err
		}
		return os.WriteFile(outPath, []byte(yamlStr), 0o644)
	default:
		return fmt.Errorf("unsupported input extension: %s", ext)
	}
}

func defaultOutPath(in string) string {
	ext := filepath.Ext(in)
	switch ext {
	case ".yaml", ".yml":
		return in[:len(in)-len(ext)] + ".flow.jsonnet"
	case ".jsonnet", ".libsonnet":
		return in[:len(in)-len(ext)] + ".flow.yaml"
	default:
		return in + ".out"
	}
}