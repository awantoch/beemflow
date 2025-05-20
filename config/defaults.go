package config

// Default directories and file paths for beemflow.
const (
	// DefaultConfigDir is the base directory for storing beemflow artifacts.
	DefaultConfigDir = ".beemflow"
	// DefaultBlobDir is the default directory for filesystem blobs.
	DefaultBlobDir = DefaultConfigDir + "/files"
	// DefaultLocalRegistryPath is the default path for the local registry file.
	DefaultLocalRegistryPath = DefaultConfigDir + "/registry.json"
	// DefaultSQLiteDSN is the default data source name for SQLite storage.
	DefaultSQLiteDSN = DefaultConfigDir + "/flow.db"
	// DefaultFlowsDir is the default directory for flow YAMLs.
	DefaultFlowsDir = "flows"
)
