package registry

import "net/http"

// InterfaceType tags the origin of each interface entry.
type InterfaceType string

const (
	CLI  InterfaceType = "cli"
	HTTP InterfaceType = "http"
	MCP  InterfaceType = "mcp"
)

// Well-known interface IDs to avoid typos and drift
const (
	InterfaceIDListRuns         = "listRuns"
	InterfaceIDListFlows        = "listFlows"
	InterfaceIDGetFlow          = "getFlow"
	InterfaceIDStartRun         = "startRun"
	InterfaceIDGetRun           = "getRun"
	InterfaceIDResumeRun        = "resumeRun"
	InterfaceIDGraphFlow        = "graphFlow"
	InterfaceIDValidateFlow     = "validateFlow"
	InterfaceIDTestFlow         = "testFlow"
	InterfaceIDAssistantChat    = "assistantChat"
	InterfaceIDInlineRun        = "inlineRun"
	InterfaceIDListTools        = "listTools"
	InterfaceIDGetToolManifest  = "getToolManifest"
	InterfaceIDListFlowsHTTP    = "listFlowsHTTP"
	InterfaceIDGetFlowSpec      = "getFlowSpec"
	InterfaceIDPublishEventHTTP = "publishEventHTTP"
	InterfaceIDPublishEvent     = "publishEvent"
	InterfaceIDDescribe         = "describe"
	InterfaceIDMetadata         = "metadata"
)

// Well-known interface descriptions to avoid typos and drift
const (
	InterfaceDescListRuns         = "List all runs"
	InterfaceDescListFlows        = "List all flows"
	InterfaceDescListFlowsHTTP    = "List flows"
	InterfaceDescGetFlow          = "Get a flow by name"
	InterfaceDescStartRun         = "Start a new run"
	InterfaceDescGetRun           = "Get run status"
	InterfaceDescResumeRun        = "Resume paused run"
	InterfaceDescGraphFlow        = "Get flow graph"
	InterfaceDescValidateFlow     = "Validate flow"
	InterfaceDescTestFlow         = "Test flow"
	InterfaceDescAssistantChat    = "Assistant chat"
	InterfaceDescInlineRun        = "Run inline flow spec"
	InterfaceDescListTools        = "List tools"
	InterfaceDescGetToolManifest  = "Get tool manifest"
	InterfaceDescGetFlowSpec      = "Get flow spec"
	InterfaceDescPublishEventHTTP = "Publish event"
	InterfaceDescPublishEvent     = "Publish an event to a topic"
	InterfaceDescMetadata         = "List all CLI/HTTP/MCP interfaces"
	InterfaceDescStaticAssets     = "Serve static assets"
	InterfaceDescHealthCheck      = "Health check"
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
