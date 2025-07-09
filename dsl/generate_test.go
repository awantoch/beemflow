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

	// Check that we have nodes and edges
	if nodes, ok := visualData["nodes"]; !ok {
		t.Error("Expected 'nodes' in visual data")
	} else if nodesList, ok := nodes.([]map[string]interface{}); !ok {
		t.Error("Expected nodes to be []map[string]interface{}")
	} else if len(nodesList) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodesList))
	}

	if edges, ok := visualData["edges"]; !ok {
		t.Error("Expected 'edges' in visual data")
	} else if edgesList, ok := edges.([]map[string]interface{}); !ok {
		t.Error("Expected edges to be []map[string]interface{}")
	} else if len(edgesList) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(edgesList))
	}

	if _, ok := visualData["flow"]; !ok {
		t.Error("Expected 'flow' in visual data")
	}
}

func TestVisualToFlow(t *testing.T) {
	visualData := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{
				"id": "node1",
				"data": map[string]interface{}{
					"id":  "step1",
					"use": "test.action1",
				},
			},
			map[string]interface{}{
				"id": "node2",
				"data": map[string]interface{}{
					"id":  "step2",
					"use": "test.action2",
					"if":  "condition",
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
	convertedFlow, err := VisualToFlow(visualData)
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