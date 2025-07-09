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

// ReactFlowRenderer outputs Graphs in React Flow format for the visual editor.
type ReactFlowRenderer struct{}

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

// Render renders the graph in React Flow format.
func (r *ReactFlowRenderer) Render(g *Graph) (string, error) {
	// This will be implemented to return JSON for React Flow
	// For now, return empty to maintain interface
	return "", nil
}

// ExportReactFlow is a helper to create React Flow data from a Flow.
func ExportReactFlow(flow *model.Flow) (map[string]interface{}, error) {
	g := NewGraph(flow)
	
	// Convert nodes to React Flow format
	reactNodes := make([]map[string]interface{}, len(g.Nodes))
	stepMap := make(map[string]model.Step)
	
	// Create step lookup map
	for _, step := range flow.Steps {
		stepMap[step.ID] = step
	}
	
	for i, node := range g.Nodes {
		step, exists := stepMap[node.ID]
		nodeData := map[string]interface{}{
			"id":    node.ID,
			"label": node.Label,
		}
		
		// Add step data if it exists
		if exists {
			nodeData["use"] = step.Use
			if step.With != nil {
				nodeData["with"] = step.With
			}
			if step.If != "" {
				nodeData["if"] = step.If
			}
		}
		
		reactNodes[i] = map[string]interface{}{
			"id":   node.ID,
			"type": "stepNode",
			"position": map[string]interface{}{
				"x": float64(i * 300), // Simple horizontal layout
				"y": 100.0,
			},
			"data": nodeData,
		}
	}
	
	// Convert edges to React Flow format
	reactEdges := make([]map[string]interface{}, len(g.Edges))
	for i, edge := range g.Edges {
		reactEdges[i] = map[string]interface{}{
			"id":     fmt.Sprintf("%s-%s", edge.From, edge.To),
			"source": edge.From,
			"target": edge.To,
			"label":  edge.Label,
		}
	}
	
	return map[string]interface{}{
		"nodes": reactNodes,
		"edges": reactEdges,
		"flow":  flow,
	}, nil
}

// ExportMermaid is a helper to create a Mermaid diagram from a Flow.
func ExportMermaid(flow *model.Flow) (string, error) {
	g := NewGraph(flow)
	renderer := &MermaidRenderer{}
	return renderer.Render(g)
}
