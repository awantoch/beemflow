package graphviz

import (
	"github.com/awantoch/beemflow/model"
)

func ExportMermaid(flow *model.Flow) (string, error) {
	if flow == nil || len(flow.Steps) == 0 {
		return "", nil
	}
	// Simple implementation: list all steps as nodes
	out := "graph TD\n"
	for _, step := range flow.Steps {
		out += step.ID + "\n"
	}
	return out, nil
}
