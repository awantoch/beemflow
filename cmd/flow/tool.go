package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// convertOpenAPISpec is a DRY helper that calls the core adapter for OpenAPI conversion
func convertOpenAPISpec(openapiData []byte, apiName, baseURL string) (map[string]any, error) {
	coreAdapter := &adapter.CoreAdapter{}
	inputs := map[string]any{
		"__use":    constants.CoreConvertOpenAPI,
		"openapi":  string(openapiData),
		"api_name": apiName,
		"base_url": baseURL,
	}
	return coreAdapter.Execute(context.Background(), inputs)
}

// newToolCmd creates the 'tool' subcommand and its scaffolding commands.
func newToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.CmdTool,
		Short: constants.DescToolingCommands,
		Run: func(cmd *cobra.Command, args []string) {
			utils.User(constants.StubFlowTool)
		},
	}

	cmd.AddCommand(
		newToolScaffoldCmd(),
		newToolConvertCmd(),
		newToolListCmd(),
	)
	return cmd
}

// newToolScaffoldCmd creates the scaffold subcommand
func newToolScaffoldCmd() *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdScaffold,
		Short: constants.DescScaffoldTool,
		Run: func(cmd *cobra.Command, args []string) {
			utils.User(constants.StubFlowTool)
		},
	}
}

// newToolConvertCmd creates the convert subcommand with flags
func newToolConvertCmd() *cobra.Command {
	convertCmd := &cobra.Command{
		Use:   constants.CmdConvert + " [openapi_file]",
		Short: constants.DescConvertOpenAPI,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runToolConvert,
	}

	// Add flags to convert command
	convertCmd.Flags().String("api-name", "", "Name prefix for generated tools (default: "+constants.DefaultAPIName+")")
	convertCmd.Flags().String("base-url", "", "Base URL override (default: extracted from spec)")
	convertCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")

	return convertCmd
}

// runToolConvert handles the convert functionality
func runToolConvert(cmd *cobra.Command, args []string) error {
	// Read OpenAPI spec from file or stdin
	openapiData, err := readOpenAPIData(args)
	if err != nil {
		return err
	}

	// Get flags
	apiName, _ := cmd.Flags().GetString("api-name")
	baseURL, _ := cmd.Flags().GetString("base-url")
	output, _ := cmd.Flags().GetString("output")

	if apiName == "" {
		apiName = constants.DefaultAPIName
	}

	// Convert using the DRY helper
	result, err := convertOpenAPISpec(openapiData, apiName, baseURL)
	if err != nil {
		return fmt.Errorf(constants.ErrConversionFailed, err)
	}

	// Output the result
	return outputConversionResult(result, output)
}

// readOpenAPIData reads OpenAPI spec from file or stdin
func readOpenAPIData(args []string) ([]byte, error) {
	if len(args) > 0 {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return nil, fmt.Errorf(constants.ErrReadFileFailed, err)
		}
		return data, nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf(constants.ErrReadStdinFailed, err)
	}
	return data, nil
}

// outputConversionResult outputs the conversion result to file or stdout
func outputConversionResult(result map[string]any, outputPath string) error {
	resultJSON, err := json.MarshalIndent(result, "", constants.JSONIndent)
	if err != nil {
		return fmt.Errorf(constants.ErrMarshalFailed, err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, resultJSON, constants.FilePermission); err != nil {
			return fmt.Errorf(constants.ErrWriteOutputFailed, err)
		}
		utils.User(constants.MsgSpecConverted, outputPath)
	} else {
		utils.User("%s", string(resultJSON))
	}

	return nil
}

// newToolListCmd creates the list subcommand
func newToolListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdList,
		Short: constants.DescListTools,
		RunE:  runToolList,
	}
}

// runToolList handles the list functionality
func runToolList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	tools, err := api.ListTools(ctx)
	if err != nil {
		return err
	}

	// Header
	utils.User(constants.HeaderTools)
	for _, t := range tools {
		name, _ := t["name"].(string)
		kind, _ := t["kind"].(string)
		desc, _ := t["description"].(string)
		endpoint, _ := t["endpoint"].(string)
		utils.User(constants.OutputFormatFour, name, kind, desc, endpoint)
	}
	return nil
}
