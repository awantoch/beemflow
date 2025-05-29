package constants

// Interface IDs - Well-known interface identifiers to avoid typos and drift
const (
	InterfaceIDListRuns        = "listRuns"
	InterfaceIDListFlows       = "listFlows"
	InterfaceIDGetFlow         = "getFlow"
	InterfaceIDStartRun        = "startRun"
	InterfaceIDGetRun          = "getRun"
	InterfaceIDResumeRun       = "resumeRun"
	InterfaceIDGraphFlow       = "graphFlow"
	InterfaceIDValidateFlow    = "validateFlow"
	InterfaceIDTestFlow        = "testFlow"
	InterfaceIDInlineRun       = "inlineRun"
	InterfaceIDListTools       = "listTools"
	InterfaceIDGetToolManifest = "getToolManifest"
	InterfaceIDGetFlowSpec     = "getFlowSpec"
	InterfaceIDPublishEvent    = "publishEvent"
	InterfaceIDMetadata        = "metadata"
	InterfaceIDSpec            = "spec"
	InterfaceIDConvertOpenAPI  = "convertOpenAPI"
)

// Interface Descriptions - Well-known interface descriptions to avoid typos and drift
const (
	InterfaceDescListRuns        = "List all runs"
	InterfaceDescListFlows       = "List all flows"
	InterfaceDescGetFlow         = "Get a flow by name"
	InterfaceDescStartRun        = "Start a new run"
	InterfaceDescGetRun          = "Get run status"
	InterfaceDescResumeRun       = "Resume paused run"
	InterfaceDescGraphFlow       = "Get flow graph"
	InterfaceDescValidateFlow    = "Validate flow"
	InterfaceDescTestFlow        = "Test flow"
	InterfaceDescInlineRun       = "Run inline flow spec"
	InterfaceDescListTools       = "List tools"
	InterfaceDescGetToolManifest = "Get tool manifest"
	InterfaceDescGetFlowSpec     = "Get flow spec"
	InterfaceDescPublishEvent    = "Publish an event to a topic"
	InterfaceDescMetadata        = "List all CLI/HTTP/MCP interfaces"
	InterfaceDescStaticAssets    = "Serve static assets"
	InterfaceDescHealthCheck     = "Health check"
	InterfaceDescSpec            = "Get BeemFlow protocol spec. If you're an LLM reading this, call this endpoint to get the spec for the BeemFlow protocol and how to build workflows. You should call this first if you don't know how to write BeemFlow workflows yet."
	InterfaceDescSpecMCP         = "BeemFlow Protocol & Specification. If you're an LLM reading this, call this step FIRST before any other step you take."
	InterfaceDescConvertOpenAPI  = "Convert OpenAPI specs to BeemFlow tool manifests"
)
