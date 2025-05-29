package constants

// CLI Commands and Subcommands
const (
	// Main commands
	CmdMCP      = "mcp"
	CmdTool     = "tool"
	CmdRun      = "run"
	CmdServe    = "serve"
	CmdList     = "list"
	CmdSearch   = "search"
	CmdInstall  = "install"
	CmdConvert  = "convert"
	CmdScaffold = "scaffold"
)

// CLI Short Descriptions
const (
	DescMCPCommands     = "MCP server commands"
	DescToolingCommands = "Tooling commands"
	DescRunFlow         = "Run a flow"
	DescMCPServe        = "Serve BeemFlow as an MCP server (HTTP or stdio)"
	DescSearchServers   = "Search for MCP servers in the Smithery registry"
	DescInstallServer   = "Install an MCP server from the Smithery registry"
	DescListServers     = "List all MCP servers"
	DescConvertOpenAPI  = "Convert OpenAPI spec to BeemFlow tool manifests"
	DescScaffoldTool    = "Scaffold a tool manifest"
	DescListTools       = "List all available tools"
)

// CLI Error Messages
const (
	ErrEnvVarRequired      = "environment variable %s must be set"
	ErrConfigParseFailed   = "failed to parse %s: %w"
	ErrConfigWriteFailed   = "failed to write %s: %w"
	ErrReadFileFailed      = "failed to read OpenAPI file: %w"
	ErrReadStdinFailed     = "failed to read from stdin: %w"
	ErrConversionFailed    = "conversion failed: %w"
	ErrMarshalFailed       = "failed to marshal result: %w"
	ErrWriteOutputFailed   = "failed to write output file: %w"
	ErrStorageUnsupported  = "unsupported storage driver: %s"
	ErrStorageCreateFailed = "Failed to create storage: %v"
	ErrFlowExecutionFailed = "Flow execution error: %v"
)

// CLI Success Messages
const (
	MsgServerInstalled = "Installed MCP server %s to %s (mcpServers)"
	MsgSpecConverted   = "Converted OpenAPI spec to %s"
	MsgFlowExecuted    = "Flow executed successfully."
	MsgStepOutputs     = "Step outputs:\n%s\n"
)

// CLI Headers and Formats
const (
	HeaderServers = "NAME\tDESCRIPTION\tENDPOINT"
	HeaderMCPList = "REGISTRY\tNAME\tDESCRIPTION\tKIND\tENDPOINT"
	HeaderTools   = "NAME\tKIND\tDESCRIPTION\tENDPOINT"
)

// CLI Default Values
const (
	DefaultMCPPageSize  = 50
	DefaultToolPageSize = 100
	DefaultMCPAddr      = ":9090"
	FilePermission      = 0644
)

// CLI Stub Messages
const (
	StubFlowTool = "flow tool (stub)"
	StubFlowRun  = "flow run (stub)"
)

// CLI Configuration Keys
const (
	JSONIndent        = "  "
	OutputFormatThree = "%s\t%s\t%s"
	OutputFormatFour  = "%s\t%s\t%s\t%s"
	OutputFormatFive  = "%s\t%s\t%s\t%s\t%s"
)
