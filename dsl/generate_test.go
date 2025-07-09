package dsl

import (
	"testing"

	"github.com/awantoch/beemflow/model"
)

func TestFlowToVisual(t *testing.T) {
	flow := &model.Flow{
		Name: "test_flow",
		On:   "cli.manual",
		Steps: []model.Step{
			{ID: "step1", Use: "test.action1"},
			{ID: "step2", Use: "test.action2"},
		},
	}

	visualData, err := FlowToVisual(flow)
	if err != nil {
		t.Fatalf("FlowToVisual failed: %v", err)
	}

	if len(visualData.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(visualData.Nodes))
	}

	if len(visualData.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(visualData.Edges))
	}

	if visualData.Flow == nil {
		t.Error("Expected flow data to be present")
	}
}

func TestVisualToFlow(t *testing.T) {
	visualData := &VisualData{
		Nodes: []VisualNode{
			{
				ID:   "node1",
				Type: "stepNode",
				Data: VisualNodeData{
					ID:  "step1",
					Use: "test.action1",
				},
			},
			{
				ID:   "node2",
				Type: "stepNode",
				Data: VisualNodeData{
					ID: "step2",
					Use: "test.action2",
					If:  "condition",
				},
			},
		},
	}

	flow, err := VisualToFlow(visualData)
	if err != nil {
		t.Fatalf("VisualToFlow failed: %v", err)
	}

	if flow.Name != "editor_flow" {
		t.Errorf("Expected flow name 'editor_flow', got '%s'", flow.Name)
	}

	if len(flow.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(flow.Steps))
	}

	if flow.Steps[0].ID != "step1" || flow.Steps[0].Use != "test.action1" {
		t.Errorf("Step 1 not converted correctly: %+v", flow.Steps[0])
	}

	if flow.Steps[1].ID != "step2" || flow.Steps[1].Use != "test.action2" || flow.Steps[1].If != "condition" {
		t.Errorf("Step 2 not converted correctly: %+v", flow.Steps[1])
	}
}

func TestVisualToFlowRoundTrip(t *testing.T) {
	originalFlow := &model.Flow{
		Name: "test_flow",
		On:   "cli.manual",
		Steps: []model.Step{
			{ID: "step1", Use: "test.action1"},
			{ID: "step2", Use: "test.action2", If: "condition"},
		},
	}

	// Convert to visual
	visualData, err := FlowToVisual(originalFlow)
	if err != nil {
		t.Fatalf("FlowToVisual failed: %v", err)
	}

	// Convert back to flow
	convertedFlow, err := VisualToFlow(&VisualData{
		Nodes: []VisualNode{
			{
				ID:   visualData.Nodes[0].ID,
				Type: visualData.Nodes[0].Type,
				Data: VisualNodeData{
					ID:  visualData.Nodes[0].Data.ID,
					Use: visualData.Nodes[0].Data.Use,
				},
			},
			{
				ID:   visualData.Nodes[1].ID,
				Type: visualData.Nodes[1].Type,
				Data: VisualNodeData{
					ID: visualData.Nodes[1].Data.ID,
					Use: visualData.Nodes[1].Data.Use,
					If:  visualData.Nodes[1].Data.If,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("VisualToFlow failed: %v", err)
	}

	// Check that steps are preserved
	if len(convertedFlow.Steps) != len(originalFlow.Steps) {
		t.Errorf("Expected %d steps, got %d", len(originalFlow.Steps), len(convertedFlow.Steps))
	}

	for i, step := range convertedFlow.Steps {
		if step.ID != originalFlow.Steps[i].ID {
			t.Errorf("Step %d ID mismatch: expected '%s', got '%s'", i, originalFlow.Steps[i].ID, step.ID)
		}
		if step.Use != originalFlow.Steps[i].Use {
			t.Errorf("Step %d Use mismatch: expected '%s', got '%s'", i, originalFlow.Steps[i].Use, step.Use)
		}
		if step.If != originalFlow.Steps[i].If {
			t.Errorf("Step %d If mismatch: expected '%s', got '%s'", i, originalFlow.Steps[i].If, step.If)
		}
	}
}