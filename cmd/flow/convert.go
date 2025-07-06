package main

import (
	"os"
	"path/filepath"

	"github.com/awantoch/beemflow/convert"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newConvertCmd creates the 'convert' subcommand for format conversion.
func newConvertCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "convert <input-file>",
		Short: "Convert between YAML and Jsonnet flow formats",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			inputPath := args[0]
			
			// Read input file
			inputData, err := os.ReadFile(inputPath)
			if err != nil {
				utils.Error("Failed to read input file: %v", err)
				exit(1)
			}
			
			// Determine input and output formats based on file extensions
			inputExt := filepath.Ext(inputPath)
			var outputExt string
			var result string

			// If output path not specified, generate one
			if outputPath == "" {
				switch inputExt {
				case ".yaml", ".yml":
					outputPath = changeExtension(inputPath, ".jsonnet")
				case ".jsonnet", ".libsonnet":
					outputPath = changeExtension(inputPath, ".yaml")
				default:
					utils.Error("Unsupported input format: %s", inputExt)
					exit(1)
				}
			}
			outputExt = filepath.Ext(outputPath)

			// Perform conversion
			switch {
			case (inputExt == ".yaml" || inputExt == ".yml") && outputExt == ".jsonnet":
				result, err = convert.YAMLToJsonnet(inputData)
			case (inputExt == ".jsonnet" || inputExt == ".libsonnet") && (outputExt == ".yaml" || outputExt == ".yml"):
				result, err = convert.JsonnetToYAML(inputData)
			default:
				utils.Error("Unsupported conversion: %s to %s", inputExt, outputExt)
				exit(1)
			}

			if err != nil {
				utils.Error("Conversion failed: %v", err)
				exit(1)
			}

			// Write output
			if err := os.WriteFile(outputPath, []byte(result), 0644); err != nil {
				utils.Error("Failed to write output file: %v", err)
				exit(1)
			}

			utils.User("Converted %s to %s", inputPath, outputPath)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (auto-detected if not specified)")
	return cmd
}

// changeExtension changes the file extension while preserving the base name
func changeExtension(path, newExt string) string {
	base := path[:len(path)-len(filepath.Ext(path))]
	return base + newExt
}