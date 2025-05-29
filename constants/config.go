package constants

// Configuration Files
const (
	ConfigFileName     = "flow.config.json"
	BeemflowSchemaFile = "beemflow.schema.json"
	RegistryIndexFile  = "registry/index.json"
)

// Configuration Schema Keys
const (
	MCPServersKey = "mcp_servers"
	SmitheryKey   = "smithery"
)

// Adapter Names
const (
	AdapterCore = "core"
	AdapterMCP  = "mcp"
	AdapterHTTP = "http"
)

// Core Tools
const (
	CoreEcho           = "core.echo"
	CoreConvertOpenAPI = "core.convert_openapi"
)

// Environment Variables
const (
	EnvDebug        = "BEEMFLOW_DEBUG"
	EnvSmitheryKey  = "SMITHERY_API_KEY"
	EnvRegistryPath = "BEEMFLOW_REGISTRY"
)

// Storage Drivers
const (
	StorageDriverSQLite   = "sqlite"
	StorageDriverPostgres = "postgres"
)
