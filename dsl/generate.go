package dsl

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/model"
)

// VisualData represents the structure for visual editor data
type VisualData struct {
	Nodes []VisualNode `json:"nodes"`
	Edges []VisualEdge `json:"edges"`
	Flow  *model.Flow  `json:"flow,omitempty"`
}

// VisualNode represents a node in the visual editor
type VisualNode struct {
	ID   string        `json:"id"`
	Type string        `json:"type"`
	Data VisualNodeData `json:"data"`
}

// VisualEdge represents an edge in the visual editor
type VisualEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// VisualNodeData represents the data payload of a visual node
type VisualNodeData struct {
	ID    string                 `json:"id"`
	Label string                 `json:"label"`
	Use   string                 `json:"use,omitempty"`
	With  map[string]interface{} `json:"with,omitempty"`
	If    string                 `json:"if,omitempty"`
}

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

// FlowToVisual converts a Flow to React Flow format for the visual editor
func FlowToVisual(flow *model.Flow) (*graph.ReactFlowData, error) {
	return graph.ExportReactFlow(flow)
}

// VisualToFlow converts visual editor data back to a Flow
func VisualToFlow(visualData *VisualData) (*model.Flow, error) {
	if visualData == nil {
		return nil, fmt.Errorf("no visual data provided")
	}
	
	if len(visualData.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes data provided")
	}
	
	steps := make([]model.Step, 0, len(visualData.Nodes))
	for _, node := range visualData.Nodes {
		step := model.Step{
			ID:  node.Data.ID,
			Use: node.Data.Use,
		}
		
		if node.Data.With != nil {
			step.With = node.Data.With
		}
		
		if node.Data.If != "" {
			step.If = node.Data.If
		}
		
		steps = append(steps, step)
	}
	
	return &model.Flow{
		Name:  "editor_flow",
		On:    "cli.manual",
		Steps: steps,
	}, nil
}