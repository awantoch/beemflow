//go:build js && wasm

package main

import (
	_ "embed"
	"encoding/json"
	"syscall/js"

	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/graph"
)

//go:embed wasm_exec.js
var WasmExecJS []byte

// Result standardizes all WASM function returns with proper typing
type Result struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
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

// handleYamlInput provides a common pattern for YAML string input processing
func handleYamlInput(args []js.Value, processor func(string) (any, error)) any {
	if len(args) == 0 {
		return resultToJS(Result{Success: false, Error: "No arguments provided"})
	}
	
	yamlStr := args[0].String()
	data, err := processor(yamlStr)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}
	
	return resultToJS(Result{Success: true, Data: data})
}

func parseYaml(this js.Value, args []js.Value) any {
	return handleYamlInput(args, func(yamlStr string) (any, error) {
		return dsl.ParseFromString(yamlStr)
	})
}

func validateYaml(this js.Value, args []js.Value) any {
	return handleYamlInput(args, func(yamlStr string) (any, error) {
		flow, err := dsl.ParseFromString(yamlStr)
		if err != nil {
			return nil, err
		}
		
		if err := dsl.Validate(flow); err != nil {
			return nil, err
		}
		
		return "Flow is valid", nil
	})
}

func generateMermaid(this js.Value, args []js.Value) any {
	return handleYamlInput(args, func(yamlStr string) (any, error) {
		flow, err := dsl.ParseFromString(yamlStr)
		if err != nil {
			return nil, err
		}
		
		return graph.ExportMermaid(flow)
	})
}

func yamlToVisual(this js.Value, args []js.Value) any {
	return handleYamlInput(args, func(yamlStr string) (any, error) {
		flow, err := dsl.ParseFromString(yamlStr)
		if err != nil {
			return nil, err
		}
		
		return dsl.FlowToVisual(flow)
	})
}

func visualToYaml(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return resultToJS(Result{Success: false, Error: "No visual data provided"})
	}

	var visualData dsl.VisualData
	if err := unmarshalJSValue(args[0], &visualData); err != nil {
		return resultToJS(Result{Success: false, Error: "Invalid visual data format: " + err.Error()})
	}

	flow, err := dsl.VisualToFlow(&visualData)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	yamlStr, err := dsl.FlowToYAMLString(flow)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: yamlStr})
}

// unmarshalJSValue converts a JS value to a Go struct using JSON marshaling
func unmarshalJSValue(jsVal js.Value, target any) error {
	// Convert JS value to JSON string
	jsonStr := js.Global().Get("JSON").Call("stringify", jsVal).String()
	
	// Unmarshal JSON string to Go struct
	return json.Unmarshal([]byte(jsonStr), target)
}

// resultToJS converts Result to JavaScript object with proper error handling
func resultToJS(r Result) map[string]any {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		// Fallback for marshal errors
		return map[string]any{
			"success": false,
			"error":   "Failed to marshal result: " + err.Error(),
		}
	}
	
	var jsResult map[string]any
	if err := json.Unmarshal(jsonBytes, &jsResult); err != nil {
		// Fallback for unmarshal errors
		return map[string]any{
			"success": false,
			"error":   "Failed to unmarshal result: " + err.Error(),
		}
	}
	
	return jsResult
}