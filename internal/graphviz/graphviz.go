package graphviz

import (
	"github.com/awantoch/beemflow/internal/model"
)

func ExportMermaid(flow *model.Flow) (string, error) {
	if flow == nil || len(flow.Steps) == 0 {
		return "", nil
	}
	// Simple implementation: list all steps as nodes
	out := "graph TD\n"
	for name := range flow.Steps {
		out += name + "\n"
	}
	return out, nil
}
