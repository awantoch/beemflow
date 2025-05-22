package graph

import (
	"fmt"
	"strings"

	pproto "github.com/awantoch/beemflow/spec/proto"
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

// NewGraph creates a Graph representation of the given Flow.
func NewGraph(flow *pproto.Flow) *Graph {
	g := &Graph{}
	if flow == nil || len(flow.Steps) == 0 {
		return g
	}
	for i, step := range flow.Steps {
		// Create node
		g.Nodes = append(g.Nodes, &Node{ID: step.GetId(), Label: step.GetId()})
		// Determine dependencies
		var deps []string
		if len(step.DependsOn) > 0 {
			deps = step.DependsOn
		} else if i > 0 {
			deps = []string{flow.Steps[i-1].GetId()}
		}
		// Create edges
		for _, dep := range deps {
			g.Edges = append(g.Edges, &Edge{From: dep, To: step.GetId()})
		}
	}
	return g
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

// ExportMermaid is a helper to create a Mermaid diagram from a Flow.
func ExportMermaid(flow *pproto.Flow) (string, error) {
	g := NewGraph(flow)
	renderer := &MermaidRenderer{}
	return renderer.Render(g)
}
