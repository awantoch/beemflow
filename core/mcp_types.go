package core

// MCP-compatible argument types that avoid interface{} fields
// These are used specifically for MCP tool registration to avoid JSON schema generation issues
//
// ⚠️  CRITICAL: DO NOT USE map[string]any OR interface{} FIELDS IN THESE TYPES! ⚠️
// The MCP-golang library will panic with nil pointer dereference when generating
// JSON schemas for these types. Always use string fields and parse JSON manually.
// See TestMCPRegressionMapStringAny for the regression test.

// MCPStartRunArgs is a simplified version of StartRunArgs for MCP
type MCPStartRunArgs struct {
	FlowName string `json:"flowName" jsonschema:"required,description=Name of the flow to start"`
	Event    string `json:"event" jsonschema:"description=JSON string containing event data"`
}

// MCPPublishEventArgs is a simplified version of PublishEventArgs for MCP
type MCPPublishEventArgs struct {
	Topic   string `json:"topic" jsonschema:"required,description=Event topic"`
	Payload string `json:"payload" jsonschema:"description=JSON string containing payload data"`
}

// MCPResumeRunArgs is a simplified version of ResumeRunArgs for MCP
type MCPResumeRunArgs struct {
	Token string `json:"token" jsonschema:"required,description=Resume token"`
	Event string `json:"event" jsonschema:"description=JSON string containing event data"`
}

// MCPGetFlowArgs is a simplified version of GetFlowArgs for MCP
type MCPGetFlowArgs struct {
	Name string `json:"name" jsonschema:"required,description=Name of the flow"`
}

// MCPValidateFlowArgs is a simplified version of ValidateFlowArgs for MCP
type MCPValidateFlowArgs struct {
	Name string `json:"name" jsonschema:"required,description=Name of the flow to validate"`
}

// MCPGraphFlowArgs is a simplified version of GraphFlowArgs for MCP
type MCPGraphFlowArgs struct {
	Name string `json:"name" jsonschema:"required,description=Name of the flow to graph"`
}

// MCPGetRunArgs is a simplified version of GetRunArgs for MCP
type MCPGetRunArgs struct {
	RunID string `json:"runId" jsonschema:"required,description=ID of the run"`
}

// MCPConvertOpenAPIExtendedArgs is a simplified version of ConvertOpenAPIExtendedArgs for MCP
type MCPConvertOpenAPIExtendedArgs struct {
	Spec string `json:"spec" jsonschema:"required,description=OpenAPI specification as JSON string"`
}

// MCPFlowFileArgs is a simplified version of FlowFileArgs for MCP
type MCPFlowFileArgs struct {
	Name string `json:"name" jsonschema:"required,description=Name of the flow file"`
}
