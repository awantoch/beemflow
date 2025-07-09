package dsl

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/model"
)

// FlowToYAML converts a Flow struct to YAML bytes
func FlowToYAML(flow *model.Flow) ([]byte, error) {
	return yaml.Marshal(flow)
}

// FlowToYAMLString converts a Flow struct to a YAML string
func FlowToYAMLString(flow *model.Flow) (string, error) {
	bytes, err := FlowToYAML(flow)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// FlowToVisual converts a Flow to visual editor format using React Flow
func FlowToVisual(flow *model.Flow) (map[string]interface{}, error) {
	return graph.ExportReactFlow(flow)
}

// VisualToFlow converts visual editor data back to a Flow
func VisualToFlow(visualData map[string]interface{}) (*model.Flow, error) {
	// Extract nodes from visual data
	nodesData, ok := visualData["nodes"]
	if !ok {
		return nil, fmt.Errorf("no nodes data provided")
	}
	
	// Handle both []interface{} and []map[string]interface{} formats
	var nodes []interface{}
	switch v := nodesData.(type) {
	case []interface{}:
		nodes = v
	case []map[string]interface{}:
		nodes = make([]interface{}, len(v))
		for i, node := range v {
			nodes[i] = node
		}
	default:
		return nil, fmt.Errorf("invalid nodes data format")
	}
	
	// Convert nodes to steps
	var steps []model.Step
	for _, nodeData := range nodes {
		node, ok := nodeData.(map[string]interface{})
		if !ok {
			continue
		}
		
		data, ok := node["data"].(map[string]interface{})
		if !ok {
			continue
		}
		
		step := model.Step{
			ID:  getString(data, "id"),
			Use: getString(data, "use"),
		}
		
		// Handle 'with' parameters
		if withData, exists := data["with"]; exists {
			step.With = withData.(map[string]interface{})
		}
		
		// Handle 'if' condition
		if ifCondition := getString(data, "if"); ifCondition != "" {
			step.If = ifCondition
		}
		
		steps = append(steps, step)
	}
	
	// Create flow with minimal structure
	flow := &model.Flow{
		Name:  "editor_flow",
		On:    "cli.manual",
		Steps: steps,
	}
	
	return flow, nil
}

// Helper function to safely get string from map
func getString(data map[string]interface{}, key string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}