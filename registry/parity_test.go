package registry

import "testing"

// TestHTTPMetadataComplete ensures all HTTP interfaces are registered.
func TestHTTPMetadataComplete(t *testing.T) {
	httpIDs := []string{
		InterfaceIDListRuns,
		InterfaceIDStartRun,
		InterfaceIDGetRun,
		InterfaceIDResumeRun,
		InterfaceIDGraphFlow,
		InterfaceIDValidateFlow,
		InterfaceIDTestFlow,
		InterfaceIDInlineRun,
		InterfaceIDListTools,
		InterfaceIDGetToolManifest,
		InterfaceIDPublishEvent,
		InterfaceIDMetadata,
		InterfaceIDSpec,
	}
	found := make(map[string]bool)
	for _, m := range AllInterfaces() {
		if m.Type == HTTP {
			found[m.ID] = true
		}
	}
	for _, id := range httpIDs {
		if !found[id] {
			t.Errorf("HTTP interface %q not registered", id)
		}
	}
}

// TestMCPMetadataComplete ensures all MCP interfaces are registered.
func TestMCPMetadataComplete(t *testing.T) {
	mcpIDs := []string{
		InterfaceIDListFlows,
		InterfaceIDGetFlow,
		InterfaceIDValidateFlow,
		InterfaceIDGraphFlow,
		InterfaceIDStartRun,
		InterfaceIDGetRun,
		InterfaceIDPublishEvent,
		InterfaceIDResumeRun,
		InterfaceIDSpec,
	}
	found := make(map[string]bool)
	for _, m := range AllInterfaces() {
		if m.Type == MCP {
			found[m.ID] = true
		}
	}
	for _, id := range mcpIDs {
		if !found[id] {
			t.Errorf("MCP interface %q not registered", id)
		}
	}
}

// TestCoreOperationsParity ensures HTTP and MCP both expose the same core operations.
func TestCoreOperationsParity(t *testing.T) {
	// Mapping of logical operations to their HTTP and MCP IDs
	ops := []struct {
		name   string
		httpID string
		mcpID  string
	}{
		{"startRun", InterfaceIDStartRun, InterfaceIDStartRun},
		{"getRun", InterfaceIDGetRun, InterfaceIDGetRun},
		{"resumeRun", InterfaceIDResumeRun, InterfaceIDResumeRun},
		{"graphFlow", InterfaceIDGraphFlow, InterfaceIDGraphFlow},
		{"validateFlow", InterfaceIDValidateFlow, InterfaceIDValidateFlow},
		{"testFlow", InterfaceIDTestFlow, InterfaceIDTestFlow},
		{"inlineRun", InterfaceIDInlineRun, InterfaceIDInlineRun},
		{"listTools", InterfaceIDListTools, InterfaceIDListTools},
		{"getToolManifest", InterfaceIDGetToolManifest, InterfaceIDGetToolManifest},
		{"publishEvent", InterfaceIDPublishEvent, InterfaceIDPublishEvent},
		{"spec", InterfaceIDSpec, InterfaceIDSpec},
	}
	httpSet := make(map[string]bool)
	mcpSet := make(map[string]bool)
	for _, m := range AllInterfaces() {
		switch m.Type {
		case HTTP:
			httpSet[m.ID] = true
		case MCP:
			mcpSet[m.ID] = true
		}
	}
	for _, op := range ops {
		if !httpSet[op.httpID] {
			t.Errorf("operation %s: HTTP ID %q not registered", op.name, op.httpID)
		}
		if !mcpSet[op.mcpID] {
			t.Errorf("operation %s: MCP ID %q not registered", op.name, op.mcpID)
		}
	}
}
