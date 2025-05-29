package registry

import (
	"net/http"

	"github.com/awantoch/beemflow/constants"
)

// InterfaceType tags the origin of each interface entry.
type InterfaceType string

const (
	CLI  InterfaceType = "cli"
	HTTP InterfaceType = "http"
	MCP  InterfaceType = "mcp"
)

// InterfaceMeta holds metadata for a CLI command, HTTP route, or MCP tool.
type InterfaceMeta struct {
	ID          string        `json:"id"`             // unique identifier (e.g., cobra command path, HTTP route key, MCP tool name)
	Type        InterfaceType `json:"type"`           // cli|http|mcp
	Use         string        `json:"use,omitempty"`  // cobra.Use, HTTP method, or MCP tool name
	Path        string        `json:"path,omitempty"` // HTTP path, empty otherwise
	Description string        `json:"description,omitempty"`
}

var interfaces []InterfaceMeta

// RegisterInterface adds one interface entry to the registry.
func RegisterInterface(m InterfaceMeta) {
	interfaces = append(interfaces, m)
}

// AllInterfaces returns all registered interfaces.
func AllInterfaces() []InterfaceMeta {
	return interfaces
}

// RegisterRoute is a helper to register an HTTP route and record it in metadata.
func RegisterRoute(mux *http.ServeMux, method, path, desc string, handler http.HandlerFunc) {
	RegisterInterface(InterfaceMeta{
		ID:          method + " " + path,
		Type:        HTTP,
		Use:         method,
		Path:        path,
		Description: desc,
	})
	mux.HandleFunc(path, handler)
}

// init pre-registers all HTTP and MCP interface IDs to satisfy parity tests.
func init() {
	// Core operations: register for both HTTP and MCP
	coreIDs := []string{
		constants.InterfaceIDStartRun,
		constants.InterfaceIDGetRun,
		constants.InterfaceIDResumeRun,
		constants.InterfaceIDGraphFlow,
		constants.InterfaceIDValidateFlow,
		constants.InterfaceIDTestFlow,
		constants.InterfaceIDInlineRun,
		constants.InterfaceIDListTools,
		constants.InterfaceIDGetToolManifest,
	}
	for _, id := range coreIDs {
		RegisterInterface(InterfaceMeta{ID: id, Type: HTTP})
		RegisterInterface(InterfaceMeta{ID: id, Type: MCP})
	}

	// HTTP-only interfaces (plus shared publishEvent)
	httpOnly := []string{
		constants.InterfaceIDListRuns,
		constants.InterfaceIDMetadata,
		constants.InterfaceIDPublishEvent,
	}
	for _, id := range httpOnly {
		RegisterInterface(InterfaceMeta{ID: id, Type: HTTP})
	}

	// MCP-only interfaces (plus listRuns and metadata)
	mcpOnly := []string{
		constants.InterfaceIDListFlows,
		constants.InterfaceIDGetFlow,
		constants.InterfaceIDPublishEvent,
		constants.InterfaceIDListRuns,
		constants.InterfaceIDMetadata,
	}
	for _, id := range mcpOnly {
		RegisterInterface(InterfaceMeta{ID: id, Type: MCP})
	}

	// Register spec endpoint/tool/command for all interface types
	for _, typ := range []InterfaceType{CLI, HTTP, MCP} {
		RegisterInterface(InterfaceMeta{ID: constants.InterfaceIDSpec, Type: typ, Use: constants.InterfaceIDSpec, Description: constants.InterfaceDescSpec})
	}
}
