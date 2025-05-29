package registry

import (
	"testing"

	"github.com/awantoch/beemflow/constants"
)

// TestHTTPMetadataComplete ensures all HTTP interfaces are registered.
func TestHTTPMetadataComplete(t *testing.T) {
	httpIDs := []string{
		constants.InterfaceIDListRuns,
		constants.InterfaceIDStartRun,
		constants.InterfaceIDGetRun,
		constants.InterfaceIDResumeRun,
		constants.InterfaceIDGraphFlow,
		constants.InterfaceIDValidateFlow,
		constants.InterfaceIDTestFlow,
		constants.InterfaceIDInlineRun,
		constants.InterfaceIDListTools,
		constants.InterfaceIDGetToolManifest,
		constants.InterfaceIDPublishEvent,
		constants.InterfaceIDMetadata,
		constants.InterfaceIDSpec,
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
		constants.InterfaceIDListFlows,
		constants.InterfaceIDGetFlow,
		constants.InterfaceIDValidateFlow,
		constants.InterfaceIDGraphFlow,
		constants.InterfaceIDStartRun,
		constants.InterfaceIDGetRun,
		constants.InterfaceIDPublishEvent,
		constants.InterfaceIDResumeRun,
		constants.InterfaceIDSpec,
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
		{"startRun", constants.InterfaceIDStartRun, constants.InterfaceIDStartRun},
		{"getRun", constants.InterfaceIDGetRun, constants.InterfaceIDGetRun},
		{"resumeRun", constants.InterfaceIDResumeRun, constants.InterfaceIDResumeRun},
		{"graphFlow", constants.InterfaceIDGraphFlow, constants.InterfaceIDGraphFlow},
		{"validateFlow", constants.InterfaceIDValidateFlow, constants.InterfaceIDValidateFlow},
		{"testFlow", constants.InterfaceIDTestFlow, constants.InterfaceIDTestFlow},
		{"inlineRun", constants.InterfaceIDInlineRun, constants.InterfaceIDInlineRun},
		{"listTools", constants.InterfaceIDListTools, constants.InterfaceIDListTools},
		{"getToolManifest", constants.InterfaceIDGetToolManifest, constants.InterfaceIDGetToolManifest},
		{"publishEvent", constants.InterfaceIDPublishEvent, constants.InterfaceIDPublishEvent},
		{"spec", constants.InterfaceIDSpec, constants.InterfaceIDSpec},
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
