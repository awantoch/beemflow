package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// Load environment variables from .env file.
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	_ "github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/dsl"
	beemhttp "github.com/awantoch/beemflow/http"
	"github.com/awantoch/beemflow/internal/api"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	"github.com/awantoch/beemflow/utils"
)

var (
	exit              = os.Exit
	configPath        string
	debug             bool
	mcpStartupTimeout time.Duration
	flowsDir          string
)

func main() {
	// Load .env as early as possible!
	_ = godotenv.Load()

	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ============================================================================
// ROOT COMMAND (from root.go)
// ============================================================================

// NewRootCmd creates the root 'flow' command with persistent flags and subcommands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{Use: "flow"}
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", constants.ConfigFileName, "Path to flow config JSON")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logs")
	rootCmd.PersistentFlags().DurationVar(&mcpStartupTimeout, "mcp-timeout", 60*time.Second, "Timeout for MCP server startup")
	rootCmd.PersistentFlags().StringVar(&flowsDir, "flows-dir", "", "Path to flows directory (overrides config file)")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Load environment variables from .env file, if present
		_ = godotenv.Load()

		// Load config JSON to pick up default flowsDir
		cfg, err := config.LoadConfig(configPath)
		if err == nil && cfg.FlowsDir != "" {
			api.SetFlowsDir(cfg.FlowsDir)
		}

		// CLI flag overrides config file
		if flowsDir != "" {
			api.SetFlowsDir(flowsDir)
		}
	}

	// Add all subcommands directly (no more need for CommandConstructors)
	rootCmd.AddCommand(
		newServeCmd(),
		newRunCmd(),
		newMCPCmd(),
	)

	// Add auto-generated commands from the unified system
	commands := api.GenerateCLICommands()
	for _, cmd := range commands {
		rootCmd.AddCommand(cmd)
	}

	return rootCmd
}

// ============================================================================
// SERVE COMMAND (from serve.go)
// ============================================================================

// newServeCmd creates the 'serve' subcommand.
func newServeCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the BeemFlow runtime HTTP server",
		Run: func(cmd *cobra.Command, args []string) {

			cfg, err := config.LoadConfig(constants.ConfigFileName)
			if err != nil {
				if os.IsNotExist(err) {
					cfg = &config.Config{}
				} else {
					utils.Error("Failed to load config: %v", err)
					exit(1)
				}
			}
			// Set default storage if not configured
			if cfg.Storage.Driver == "" {
				cfg.Storage.Driver = "sqlite"
				cfg.Storage.DSN = config.DefaultSQLiteDSN
			}
			if err := cfg.Validate(); err != nil {
				utils.Error("Config validation failed: %v", err)
				exit(1)
			}

			// Apply the addr flag if provided
			if addr != "" {
				if cfg.HTTP == nil {
					cfg.HTTP = &config.HTTPConfig{}
				}

				// Parse host:port format
				host, portStr, found := strings.Cut(addr, ":")
				if !found {
					utils.Error("Invalid address format: %s (expected host:port)", addr)
					exit(1)
				}

				port, err := strconv.Atoi(portStr)
				if err != nil {
					utils.Error("Invalid port number: %v", err)
					exit(1)
				}

				cfg.HTTP.Host = host
				cfg.HTTP.Port = port
			}

			utils.Info("Starting BeemFlow HTTP server...")
			// If stdout is not a terminal (e.g., piped in tests), skip starting the server to avoid blocking
			if fi, statErr := os.Stdout.Stat(); statErr == nil && fi.Mode()&os.ModeCharDevice == 0 {
				utils.User("flow serve (stub)")
				return
			}
			if err := beemhttp.StartServer(cfg); err != nil {
				utils.Error("Failed to start server: %v", err)
				exit(1)
			}
		},
	}

	// Add local flags
	cmd.Flags().StringVar(&addr, "addr", "", "Listen address in the format host:port")

	return cmd
}

// ============================================================================
// RUN COMMAND (from run.go)
// ============================================================================

// newRunCmd creates the 'run' subcommand.
func newRunCmd() *cobra.Command {
	var eventPath, eventJSON string
	cmd := &cobra.Command{
		Use:   constants.CmdRun + " [file]",
		Short: constants.DescRunFlow,
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			runFlowExecution(cmd, args, eventPath, eventJSON)
		},
	}
	cmd.Flags().StringVar(&eventPath, "event", "", "Path to event JSON file")
	cmd.Flags().StringVar(&eventJSON, "event-json", "", "Event as inline JSON string")
	return cmd
}

// runFlowExecution handles the main flow execution logic using the API service
func runFlowExecution(cmd *cobra.Command, args []string, eventPath, eventJSON string) {
	// Handle stub behavior when no file argument is provided
	if len(args) == 0 {
		utils.User(constants.StubFlowRun)
		return
	}

	// Parse the flow file
	flow, err := dsl.Parse(args[0])
	if err != nil {
		utils.Error("YAML parse error: %v", err)
		exit(1)
	}

	// Load event data
	event, err := loadEvent(eventPath, eventJSON)
	if err != nil {
		utils.Error("Failed to load event: %v", err)
		exit(4)
	}

	// Use the API service instead of direct engine access
	runID, outputs, err := api.RunSpec(cmd.Context(), flow, event)
	if err != nil {
		utils.Error(constants.ErrFlowExecutionFailed, err)
		exit(5)
	}

	// Output results
	utils.Info("Run ID: %s", runID.String())
	outputFlowResults(outputs)
}

// outputFlowResults handles the output of flow execution results
func outputFlowResults(outputs map[string]any) {
	if debug {
		outputDebugResults(outputs)
	} else {
		outputEchoResults(outputs)
	}
}

// outputDebugResults outputs all results as JSON for debugging
func outputDebugResults(outputs map[string]any) {
	outJSONBytes, _ := json.MarshalIndent(outputs, "", constants.JSONIndent)
	utils.User("%s", string(outJSONBytes))
	utils.Info(constants.MsgFlowExecuted)
	utils.Info(constants.MsgStepOutputs, string(outJSONBytes))
}

// outputEchoResults outputs all step results for normal operation
func outputEchoResults(outputs map[string]any) {
	// Track what we've already output to avoid duplicates
	displayed := make(map[string]bool)

	// Output step results in a clean, user-friendly format
	for stepID, stepOutput := range outputs {
		if stepOutput == nil || displayed[stepID] {
			continue
		}

		// Try different output format handlers in order
		if outputHandled := tryOutputSpecificFormats(stepID, stepOutput, displayed); outputHandled {
			continue
		}

		// Fallback: show compact JSON for anything else
		outputFallbackJSON(stepID, stepOutput, displayed)
	}
}

// tryOutputSpecificFormats attempts to handle known output formats
func tryOutputSpecificFormats(stepID string, stepOutput any, displayed map[string]bool) bool {
	outMap, ok := stepOutput.(map[string]any)
	if !ok {
		return false
	}

	// Try each specific format handler
	if tryOutputEchoText(stepID, outMap, displayed) {
		return true
	}
	if tryOutputOpenAIResponse(stepID, outMap, displayed) {
		return true
	}
	if tryOutputMCPResponse(stepID, outMap, displayed) {
		return true
	}
	if tryOutputHTTPResponse(stepID, outMap, displayed) {
		return true
	}
	if tryOutputParallelResults(stepID, outMap, displayed) {
		return true
	}

	return false
}

// tryOutputEchoText handles core.echo outputs - just show the text
func tryOutputEchoText(stepID string, outMap map[string]any, displayed map[string]bool) bool {
	if text, ok := outMap[constants.OutputKeyText]; ok {
		utils.User("%s", text)
		displayed[stepID] = true
		return true
	}
	return false
}

// tryOutputOpenAIResponse handles OpenAI chat completions - extract the message content
func tryOutputOpenAIResponse(stepID string, outMap map[string]any, displayed map[string]bool) bool {
	choices, ok := outMap[constants.OutputKeyChoices].([]interface{})
	if !ok || len(choices) == 0 {
		return false
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return false
	}

	message, ok := choice[constants.OutputKeyMessage].(map[string]any)
	if !ok {
		return false
	}

	content, ok := message[constants.OutputKeyContent].(string)
	if !ok {
		return false
	}

	utils.User(constants.OutputPrefixAI+"%s: %s", stepID, content)
	displayed[stepID] = true
	return true
}

// tryOutputMCPResponse handles MCP responses with content array - extract text
func tryOutputMCPResponse(stepID string, outMap map[string]any, displayed map[string]bool) bool {
	content, ok := outMap[constants.OutputKeyContent].([]interface{})
	if !ok || len(content) == 0 {
		return false
	}

	contentItem, ok := content[0].(map[string]any)
	if !ok {
		return false
	}

	text, ok := contentItem[constants.OutputKeyText].(string)
	if !ok {
		return false
	}

	utils.User(constants.OutputPrefixMCP+"%s: %s", stepID, text)
	displayed[stepID] = true
	return true
}

// tryOutputHTTPResponse handles HTTP fetch responses - show just the body preview
func tryOutputHTTPResponse(stepID string, outMap map[string]any, displayed map[string]bool) bool {
	body, ok := outMap[constants.OutputKeyBody].(string)
	if !ok {
		return false
	}

	preview := body
	if len(preview) > constants.OutputPreviewLimit {
		preview = preview[:constants.OutputPreviewLimit] + constants.OutputTruncationSuffix
	}

	utils.User(constants.OutputPrefixHTTP+"%s: %s", stepID, preview)
	displayed[stepID] = true
	return true
}

// tryOutputParallelResults handles parallel step outputs - extract individual step results
func tryOutputParallelResults(stepID string, outMap map[string]any, displayed map[string]bool) bool {
	foundParallelOutputs := false

	for subStepID, subOutput := range outMap {
		if displayed[subStepID] {
			continue
		}

		if handleParallelSubstep(subStepID, subOutput, displayed) {
			foundParallelOutputs = true
		}
	}

	if foundParallelOutputs {
		displayed[stepID] = true
		return true
	}

	return false
}

// handleParallelSubstep processes individual parallel substeps
func handleParallelSubstep(subStepID string, subOutput any, displayed map[string]bool) bool {
	subOutputMap, ok := subOutput.(map[string]any)
	if !ok {
		return false
	}

	// Check if this looks like an OpenAI response
	return tryOutputOpenAIResponse(subStepID, subOutputMap, displayed)
}

// outputFallbackJSON handles fallback JSON output for unrecognized formats
func outputFallbackJSON(stepID string, stepOutput any, displayed map[string]bool) {
	outJSONBytes, err := json.MarshalIndent(stepOutput, "", "  ")
	if err == nil && len(outJSONBytes) < constants.OutputJSONSizeLimit {
		utils.User(constants.OutputPrefixJSON+"%s: %s", stepID, string(outJSONBytes))
	} else {
		utils.User(constants.OutputPrefixJSON+"%s: %s", stepID, constants.OutputTooLargeMessage)
	}
	displayed[stepID] = true
}

// loadEvent loads event data from a file or an inline JSON string.
func loadEvent(path, inline string) (map[string]any, error) {
	var event map[string]any
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		return event, nil
	}
	if inline != "" {
		if err := json.Unmarshal([]byte(inline), &event); err != nil {
			return nil, err
		}
		return event, nil
	}
	// No event provided: return empty event for flows that don't use event data
	return map[string]any{}, nil
}

// ============================================================================
// SHARED UTILITIES FOR MCP AND TOOLS COMMANDS (DRY)
// ============================================================================

// registrySearchOptions holds parameters for registry searches
type registrySearchOptions struct {
	query          string
	filterKind     string // "mcp" for MCP servers, "tool" for tool manifests
	headerFormat   string
	threeColFormat string
}

// runRegistrySearch handles search functionality for both MCP servers and tools
func runRegistrySearch(opts registrySearchOptions) error {
	ctx := context.Background()

	// Use federated registry system instead of just Smithery
	factory := registry.NewFactory()
	cfg, _ := config.LoadConfig(configPath) // Ignore errors, use defaults
	manager := factory.CreateStandardManager(ctx, cfg)

	entries, err := manager.ListAllServers(ctx, registry.ListOptions{
		Query:    opts.query,
		PageSize: constants.DefaultMCPPageSize,
	})
	if err != nil {
		return err
	}

	utils.User(opts.headerFormat, "Name", "Description", "Endpoint")
	for _, s := range entries {
		// Apply filtering based on type (not kind)
		switch {
		case opts.filterKind == "mcp" && s.Type == "mcp_server":
			utils.User(opts.threeColFormat, s.Name, s.Description, s.Endpoint)
		case opts.filterKind == "tool" && s.Type == "tool":
			utils.User(opts.threeColFormat, s.Name, s.Description, s.Endpoint)
		case opts.filterKind == "":
			// No filtering, show all
			utils.User(opts.threeColFormat, s.Name, s.Description, s.Endpoint)
		}
	}
	return nil
}

// runRegistryInstall handles installation for MCP servers to config file
func runRegistryInstall(itemName, configFile, successMsg string) error {
	// Read existing config as raw JSON (preserve only user overrides)
	doc, err := loadConfigAsMap(configFile)
	if err != nil {
		return err
	}

	// Ensure mcpServers map exists (tools and servers share the same registry)
	mcpMap := ensureMCPServersMap(doc)

	// Fetch spec from Smithery (same registry for tools and servers)
	spec, err := fetchServerSpec(itemName)
	if err != nil {
		return err
	}

	// Update configuration
	mcpMap[itemName] = spec
	doc[constants.MCPServersKey] = mcpMap

	// Write updated config
	if err := writeConfigMap(doc, configFile); err != nil {
		return err
	}

	// Success message
	utils.User(successMsg, itemName, configFile)
	return nil
}

// ============================================================================
// SHARED UTILITY FUNCTIONS
// ============================================================================

// loadConfigAsMap loads configuration file as a generic map
func loadConfigAsMap(configFile string) (map[string]any, error) {
	var doc map[string]any
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf(constants.ErrConfigParseFailed, configFile, err)
	}
	return doc, nil
}

// ensureMCPServersMap ensures the mcpServers map exists in the config
func ensureMCPServersMap(doc map[string]any) map[string]any {
	mcpMap, ok := doc[constants.MCPServersKey].(map[string]any)
	if !ok {
		mcpMap = map[string]any{}
	}
	return mcpMap
}

// fetchServerSpec fetches server specification from Smithery registry
func fetchServerSpec(serverName string) (any, error) {
	ctx := context.Background()
	apiKey := os.Getenv(constants.EnvSmitheryKey)
	if apiKey == "" {
		return nil, fmt.Errorf(constants.ErrEnvVarRequired, constants.EnvSmitheryKey)
	}

	client := registry.NewSmitheryRegistry(apiKey, "")
	return client.GetServerSpec(ctx, serverName)
}

// writeConfigMap writes the configuration map to file
func writeConfigMap(doc map[string]any, configFile string) error {
	out, err := json.MarshalIndent(doc, "", constants.JSONIndent)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configFile, out, constants.FilePermission); err != nil {
		return fmt.Errorf(constants.ErrConfigWriteFailed, configFile, err)
	}
	return nil
}

// ============================================================================
// MCP COMMANDS
// ============================================================================

// newMCPCmd creates the 'mcp' subcommand and its subcommands.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   constants.CmdMCP,
		Short: constants.DescMCPCommands,
	}

	var configFile = &configPath

	cmd.AddCommand(
		newMCPServeCmd(),
		newMCPSearchCmd(),
		newMCPInstallCmd(configFile),
		newMCPListCmd(configFile),
	)
	return cmd
}

// newMCPSearchCmd creates the search subcommand for MCP servers
func newMCPSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdSearch + " [query]",
		Short: constants.DescSearchServers,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runMCPSearch,
	}
}

// runMCPSearch handles the search functionality for MCP servers
func runMCPSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	return runRegistrySearch(registrySearchOptions{
		query:          query,
		filterKind:     "mcp",
		headerFormat:   constants.HeaderServers,
		threeColFormat: constants.FormatThreeColumns,
	})
}

// newMCPInstallCmd creates the install subcommand for MCP servers
func newMCPInstallCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdInstall + " <serverName>",
		Short: constants.DescInstallServer,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPInstall(args[0], *configFile)
		},
	}
}

// runMCPInstall handles the installation of MCP servers
func runMCPInstall(serverName, configFile string) error {
	return runRegistryInstall(serverName, configFile, constants.MsgServerInstalled)
}

// newMCPListCmd creates the list subcommand for MCP servers
func newMCPListCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   constants.CmdList,
		Short: constants.DescListServers,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPList(*configFile)
		},
	}
}

// runMCPList handles listing all MCP servers
func runMCPList(configFile string) error {
	// Load config to get installed MCP servers
	cfg, err := config.LoadConfig(configFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	ctx := context.Background()
	utils.User(constants.HeaderMCPList, "Source", "Name", "Description", "Type", "Endpoint")

	// List servers from config
	if cfg != nil && cfg.MCPServers != nil {
		for name, spec := range cfg.MCPServers {
			utils.User(constants.FormatFiveColumns, "config", name, "", spec.Transport, spec.Endpoint)
		}
	}

	// List servers from all registries using factory
	factory := registry.NewFactory()
	manager := factory.CreateStandardManager(ctx, cfg)
	allEntries, err := manager.ListAllServers(ctx, registry.ListOptions{
		PageSize: constants.DefaultToolPageSize,
	})
	if err == nil {
		// Filter for MCP servers only
		for _, s := range allEntries {
			if s.Type == "mcp_server" {
				utils.User(constants.FormatFiveColumns, s.Registry, s.Name, s.Description, s.Kind, s.Endpoint)
			}
		}
	}
	return nil
}

// newMCPServeCmd creates the serve subcommand for MCP
func newMCPServeCmd() *cobra.Command {
	var stdio bool
	var addr string
	cmd := &cobra.Command{
		Use:   constants.CmdServe,
		Short: constants.DescMCPServe,
		RunE: func(cmd *cobra.Command, args []string) error {
			tools := api.GenerateMCPTools()
			return mcpserver.Serve(configPath, debug, stdio, addr, tools)
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", true, "serve over stdin/stdout instead of HTTP (default)")
	cmd.Flags().StringVar(&addr, "addr", constants.DefaultMCPAddr, "listen address for HTTP mode")
	return cmd
}

// ============================================================================
// END OF FILE - Clean Architecture Achieved
// ============================================================================
