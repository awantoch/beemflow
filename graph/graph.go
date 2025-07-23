package graph

import (
	"fmt"
	"strings"

	"github.com/awantoch/beemflow/model"
)

// Node is a vertex in the graph.
type Node struct {
	ID    string
	Label string
}

// Edge is a directed connection between two nodes.
type Edge struct {
	From  string
	To    string
	Label string
}

// Graph is a directed graph composed of nodes and edges.
type Graph struct {
	Nodes []*Node
	Edges []*Edge
}

// Renderer renders a Graph into a specific output format.
type Renderer interface {
	Render(g *Graph) (string, error)
}

// MermaidRenderer outputs Graphs in Mermaid flowchart syntax.
type MermaidRenderer struct{}

// ReactFlowData represents the data structure for React Flow visual editor
type ReactFlowData struct {
	Nodes []ReactFlowNode `json:"nodes"`
	Edges []ReactFlowEdge `json:"edges"`
	Flow  *model.Flow     `json:"flow"`
}

// ReactFlowNode represents a node in React Flow format
type ReactFlowNode struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Position ReactFlowPosition    `json:"position"`
	Data     ReactFlowNodeData    `json:"data"`
}

// ReactFlowEdge represents an edge in React Flow format
type ReactFlowEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

// ReactFlowPosition represents position coordinates
type ReactFlowPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ReactFlowNodeData represents the data payload of a React Flow node
type ReactFlowNodeData struct {
	ID    string                 `json:"id"`
	Label string                 `json:"label"`
	Use   string                 `json:"use,omitempty"`
	With  map[string]interface{} `json:"with,omitempty"`
	If    string                 `json:"if,omitempty"`
}

// NewGraph creates a Graph representation of the given Flow.
func NewGraph(flow *model.Flow) *Graph {
	g := &Graph{}
	if flow == nil || len(flow.Steps) == 0 {
		return g
	}

	g.processSteps(flow.Steps, "")
	return g
}

// processSteps recursively processes steps and their nested parallel steps
func (g *Graph) processSteps(steps []model.Step, parentID string) {
	for i, step := range steps {
		// Create node
		g.Nodes = append(g.Nodes, &Node{ID: step.ID, Label: step.ID})

		// Handle parallel steps by recursing into nested steps
		if step.Parallel && len(step.Steps) > 0 {
			g.processSteps(step.Steps, step.ID)
			continue
		}

		// Determine dependencies
		var deps []string
		switch {
		case len(step.DependsOn) > 0:
			deps = step.DependsOn
		case parentID != "":
			// If we're in a parallel block, depend on the parent
			deps = []string{parentID}
		case i > 0:
			// Sequential dependency on previous step
			deps = []string{steps[i-1].ID}
		}

		// Create edges
		for _, dep := range deps {
			g.Edges = append(g.Edges, &Edge{From: dep, To: step.ID})
		}
	}
}

// Render renders the graph using Mermaid syntax.
func (r *MermaidRenderer) Render(g *Graph) (string, error) {
	if len(g.Nodes) == 0 {
		return "", nil
	}
	var sb strings.Builder
	sb.WriteString("graph TD\n")
	// Output node definitions
	for _, node := range g.Nodes {
		sb.WriteString(fmt.Sprintf("%s[%s]\n", node.ID, node.Label))
	}
	// Output edges
	for _, edge := range g.Edges {
		if edge.Label != "" {
			sb.WriteString(fmt.Sprintf("%s -->|%s| %s\n", edge.From, edge.Label, edge.To))
		} else {
			sb.WriteString(fmt.Sprintf("%s --> %s\n", edge.From, edge.To))
		}
	}
	return sb.String(), nil
}

// ExportReactFlow creates React Flow data from a Flow with proper typing
func ExportReactFlow(flow *model.Flow) (*ReactFlowData, error) {
	g := NewGraph(flow)
	stepMap := createStepMap(flow.Steps)
	
	return &ReactFlowData{
		Nodes: convertNodesToReactFlow(g.Nodes, stepMap),
		Edges: convertEdgesToReactFlow(g.Edges),
		Flow:  flow,
	}, nil
}

// createStepMap creates a lookup map for steps
func createStepMap(steps []model.Step) map[string]model.Step {
	stepMap := make(map[string]model.Step, len(steps))
	for _, step := range steps {
		stepMap[step.ID] = step
	}
	return stepMap
}

// convertNodesToReactFlow converts graph nodes to React Flow format with proper typing
func convertNodesToReactFlow(nodes []*Node, stepMap map[string]model.Step) []ReactFlowNode {
	reactNodes := make([]ReactFlowNode, len(nodes))
	
	for i, node := range nodes {
		nodeData := ReactFlowNodeData{
			ID:    node.ID,
			Label: node.Label,
		}
		
		// Add step data if it exists
		if step, exists := stepMap[node.ID]; exists {
			nodeData.Use = step.Use
			if step.With != nil {
				nodeData.With = step.With
			}
			if step.If != "" {
				nodeData.If = step.If
			}
		}
		
		reactNodes[i] = ReactFlowNode{
			ID:   node.ID,
			Type: "stepNode",
			Position: ReactFlowPosition{
				X: float64(i * 300), // Simple horizontal layout
				Y: 100.0,
			},
			Data: nodeData,
		}
	}
	
	return reactNodes
}

// convertEdgesToReactFlow converts graph edges to React Flow format with proper typing
func convertEdgesToReactFlow(edges []*Edge) []ReactFlowEdge {
	reactEdges := make([]ReactFlowEdge, len(edges))
	
	for i, edge := range edges {
		reactEdges[i] = ReactFlowEdge{
			ID:     fmt.Sprintf("%s-%s", edge.From, edge.To),
			Source: edge.From,
			Target: edge.To,
			Label:  edge.Label,
		}
	}
	
	return reactEdges
}

// ExportMermaid is a helper to create a Mermaid diagram from a Flow.
func ExportMermaid(flow *model.Flow) (string, error) {
	g := NewGraph(flow)
	renderer := &MermaidRenderer{}
	return renderer.Render(g)
}
