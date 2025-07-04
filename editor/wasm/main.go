//go:build js && wasm

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/graph"
	"github.com/awantoch/beemflow/model"
)

//go:embed wasm_exec.js
var WasmExecJS []byte

// Result standardizes all WASM function returns
type Result struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func main() {
	// Register BeemFlow functions for JavaScript
	js.Global().Set("beemflowParseYaml", js.FuncOf(parseYaml))
	js.Global().Set("beemflowValidateYaml", js.FuncOf(validateYaml))
	js.Global().Set("beemflowGenerateMermaid", js.FuncOf(generateMermaid))
	js.Global().Set("beemflowYamlToVisual", js.FuncOf(yamlToVisual))
	js.Global().Set("beemflowVisualToYaml", js.FuncOf(visualToYaml))

	// Keep the WASM module alive
	<-make(chan bool)
}

func parseYaml(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return result(false, nil, "No YAML provided")
	}

	yaml := args[0].String()
	flow, err := dsl.ParseFromString(yaml)
	if err != nil {
		return result(false, nil, err.Error())
	}

	return result(true, flowToMap(flow), "")
}

func validateYaml(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return result(false, nil, "No YAML provided")
	}

	yaml := args[0].String()
	flow, err := dsl.ParseFromString(yaml)
	if err != nil {
		return result(false, nil, err.Error())
	}

	if err := dsl.Validate(flow); err != nil {
		return result(false, nil, err.Error())
	}

	return result(true, "Flow is valid", "")
}

func generateMermaid(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return result(false, nil, "No YAML provided")
	}

	yaml := args[0].String()
	flow, err := dsl.ParseFromString(yaml)
	if err != nil {
		return result(false, nil, err.Error())
	}

	diagram, err := graph.ExportMermaid(flow)
	if err != nil {
		return result(false, nil, err.Error())
	}

	return result(true, diagram, "")
}

func yamlToVisual(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return result(false, nil, "No YAML provided")
	}

	yaml := args[0].String()
	flow, err := dsl.ParseFromString(yaml)
	if err != nil {
		return result(false, nil, err.Error())
	}

	nodes := []map[string]any{}
	edges := []map[string]any{}

	// Convert steps to visual nodes
	for i, step := range flow.Steps {
		node := map[string]any{
			"id":   step.ID,
			"type": "stepNode",
			"position": map[string]any{
				"x": float64(i * 300),
				"y": 100.0,
			},
			"data": map[string]any{
				"id":   step.ID,
				"use":  step.Use,
				"with": step.With,
				"if":   step.If,
			},
		}
		nodes = append(nodes, node)

		// Create edges based on dependencies
		if len(step.DependsOn) > 0 {
			for _, dep := range step.DependsOn {
				edge := map[string]any{
					"id":     fmt.Sprintf("%s-%s", dep, step.ID),
					"source": dep,
					"target": step.ID,
				}
				edges = append(edges, edge)
			}
		} else if i > 0 {
			// Sequential dependency
			prevStep := flow.Steps[i-1]
			edge := map[string]any{
				"id":     fmt.Sprintf("%s-%s", prevStep.ID, step.ID),
				"source": prevStep.ID,
				"target": step.ID,
			}
			edges = append(edges, edge)
		}
	}

	return result(true, map[string]any{
		"nodes": nodes,
		"edges": edges,
		"flow":  flowToMap(flow),
	}, "")
}

func visualToYaml(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return result(false, nil, "No visual data provided")
	}

	// Parse the visual data from JavaScript
	visualData := args[0]
	
	// Extract nodes array
	nodesJS := visualData.Get("nodes")
	if nodesJS.Type() != js.TypeObject {
		return result(false, nil, "Invalid nodes data")
	}

	// Convert nodes to steps
	var steps []model.Step
	nodesLength := nodesJS.Length()
	
	for i := 0; i < nodesLength; i++ {
		nodeJS := nodesJS.Index(i)
		dataJS := nodeJS.Get("data")
		
		step := model.Step{
			ID:  dataJS.Get("id").String(),
			Use: dataJS.Get("use").String(),
		}

		// Extract 'with' parameters if they exist
		withJS := dataJS.Get("with")
		if withJS.Type() == js.TypeObject {
			step.With = jsObjectToMap(withJS)
		}

		// Extract 'if' condition if it exists
		ifJS := dataJS.Get("if")
		if ifJS.Type() == js.TypeString && ifJS.String() != "" {
			step.If = ifJS.String()
		}

		steps = append(steps, step)
	}

	// Create flow
	flow := &model.Flow{
		Name:  "editor_flow",
		On:    "cli.manual",
		Steps: steps,
	}

	// Generate YAML
	yamlBytes, err := dsl.FlowToYAML(flow)
	if err != nil {
		return result(false, nil, err.Error())
	}

	return result(true, string(yamlBytes), "")
}

// Helper functions
func result(success bool, data interface{}, errorMsg string) map[string]any {
	r := Result{Success: success}
	if success {
		r.Data = data
	} else {
		r.Error = errorMsg
	}
	
	// Convert to map for JS consumption
	jsonBytes, _ := json.Marshal(r)
	var resultMap map[string]any
	json.Unmarshal(jsonBytes, &resultMap)
	return resultMap
}

func flowToMap(flow *model.Flow) map[string]any {
	return map[string]any{
		"name":    flow.Name,
		"version": flow.Version,
		"on":      flow.On,
		"vars":    flow.Vars,
		"steps":   stepsToMaps(flow.Steps),
		"catch":   stepsToMaps(flow.Catch),
	}
}

func stepsToMaps(steps []model.Step) []map[string]any {
	result := make([]map[string]any, len(steps))
	for i, step := range steps {
		result[i] = map[string]any{
			"id":         step.ID,
			"use":        step.Use,
			"with":       step.With,
			"depends_on": step.DependsOn,
			"parallel":   step.Parallel,
			"if":         step.If,
			"foreach":    step.Foreach,
			"as":         step.As,
			"steps":      stepsToMaps(step.Steps),
		}
	}
	return result
}

func jsObjectToMap(obj js.Value) map[string]any {
	result := make(map[string]any)
	
	// Get all property names
	keys := js.Global().Get("Object").Call("keys", obj)
	keysLength := keys.Length()
	
	for i := 0; i < keysLength; i++ {
		key := keys.Index(i).String()
		value := obj.Get(key)
		
		switch value.Type() {
		case js.TypeString:
			result[key] = value.String()
		case js.TypeNumber:
			result[key] = value.Float()
		case js.TypeBoolean:
			result[key] = value.Bool()
		case js.TypeObject:
			result[key] = jsObjectToMap(value)
		}
	}
	
	return result
}