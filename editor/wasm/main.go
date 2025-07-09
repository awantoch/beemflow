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

// Common helper for YAML parsing with error handling
func parseAndValidateYaml(yamlStr string) (*model.Flow, *Result) {
	if yamlStr == "" {
		return nil, &Result{Success: false, Error: "No YAML provided"}
	}

	flow, err := dsl.ParseFromString(yamlStr)
	if err != nil {
		return nil, &Result{Success: false, Error: err.Error()}
	}

	return flow, nil
}

// Common helper for extracting YAML string from JS args
func getYamlFromArgs(args []js.Value) (string, *Result) {
	if len(args) == 0 {
		return "", &Result{Success: false, Error: "No arguments provided"}
	}
	return args[0].String(), nil
}

func parseYaml(this js.Value, args []js.Value) any {
	yamlStr, errResult := getYamlFromArgs(args)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	flow, errResult := parseAndValidateYaml(yamlStr)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	// Use native Go JSON marshaling since Flow has json tags
	return resultToJS(Result{Success: true, Data: flow})
}

func validateYaml(this js.Value, args []js.Value) any {
	yamlStr, errResult := getYamlFromArgs(args)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	flow, errResult := parseAndValidateYaml(yamlStr)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	// Use existing BeemFlow validation
	if err := dsl.Validate(flow); err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: "Flow is valid"})
}

func generateMermaid(this js.Value, args []js.Value) any {
	yamlStr, errResult := getYamlFromArgs(args)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	flow, errResult := parseAndValidateYaml(yamlStr)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	// Use existing BeemFlow Mermaid generation
	diagram, err := graph.ExportMermaid(flow)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: diagram})
}

func yamlToVisual(this js.Value, args []js.Value) any {
	yamlStr, errResult := getYamlFromArgs(args)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	flow, errResult := parseAndValidateYaml(yamlStr)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	// Use existing BeemFlow graph generation
	g := graph.NewGraph(flow)
	
	// Convert to React Flow format
	visualData := map[string]interface{}{
		"nodes": graphNodesToReactFlow(g.Nodes, flow),
		"edges": graphEdgesToReactFlow(g.Edges),
		"flow":  flow, // Flow already has JSON tags
	}

	return resultToJS(Result{Success: true, Data: visualData})
}

func visualToYaml(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return resultToJS(Result{Success: false, Error: "No visual data provided"})
	}

	// Extract nodes from JavaScript
	visualData := args[0]
	nodesJS := visualData.Get("nodes")
	if nodesJS.Type() != js.TypeObject {
		return resultToJS(Result{Success: false, Error: "Invalid nodes data"})
	}

	// Convert JS nodes to model.Step (minimal conversion)
	var steps []model.Step
	nodesLength := nodesJS.Length()
	
	for i := 0; i < nodesLength; i++ {
		nodeJS := nodesJS.Index(i)
		dataJS := nodeJS.Get("data")
		
		step := model.Step{
			ID:  dataJS.Get("id").String(),
			Use: dataJS.Get("use").String(),
		}

		// Handle 'with' parameters
		if withJS := dataJS.Get("with"); withJS.Type() == js.TypeObject {
			step.With = jsValueToMap(withJS)
		}

		// Handle 'if' condition
		if ifJS := dataJS.Get("if"); ifJS.Type() == js.TypeString && ifJS.String() != "" {
			step.If = ifJS.String()
		}

		steps = append(steps, step)
	}

	// Create flow with minimal structure
	flow := &model.Flow{
		Name:  "editor_flow",
		On:    "cli.manual",
		Steps: steps,
	}

	// Use existing BeemFlow YAML generation
	yamlBytes, err := dsl.FlowToYAML(flow)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: string(yamlBytes)})
}

// Convert graph nodes to React Flow format
func graphNodesToReactFlow(nodes []*graph.Node, flow *model.Flow) []map[string]interface{} {
	reactNodes := make([]map[string]interface{}, len(nodes))
	
	// Create a map of step ID to step for quick lookup
	stepMap := make(map[string]model.Step)
	for _, step := range flow.Steps {
		stepMap[step.ID] = step
	}
	
	for i, node := range nodes {
		// Get the corresponding step data
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
	
	return reactNodes
}

// Convert graph edges to React Flow format
func graphEdgesToReactFlow(edges []*graph.Edge) []map[string]interface{} {
	reactEdges := make([]map[string]interface{}, len(edges))
	
	for i, edge := range edges {
		reactEdges[i] = map[string]interface{}{
			"id":     fmt.Sprintf("%s-%s", edge.From, edge.To),
			"source": edge.From,
			"target": edge.To,
			"label":  edge.Label,
		}
	}
	
	return reactEdges
}

// Convert JS Value to Go map (minimal implementation)
func jsValueToMap(jsVal js.Value) map[string]interface{} {
	result := make(map[string]interface{})
	
	keys := js.Global().Get("Object").Call("keys", jsVal)
	for i := 0; i < keys.Length(); i++ {
		key := keys.Index(i).String()
		value := jsVal.Get(key)
		
		switch value.Type() {
		case js.TypeString:
			result[key] = value.String()
		case js.TypeNumber:
			result[key] = value.Float()
		case js.TypeBoolean:
			result[key] = value.Bool()
		case js.TypeObject:
			result[key] = jsValueToMap(value)
		default:
			result[key] = value.String()
		}
	}
	
	return result
}

// Convert Result to JavaScript object
func resultToJS(r Result) map[string]interface{} {
	// Use Go's built-in JSON marshaling and unmarshaling
	jsonBytes, _ := json.Marshal(r)
	var jsResult map[string]interface{}
	json.Unmarshal(jsonBytes, &jsResult)
	return jsResult
}