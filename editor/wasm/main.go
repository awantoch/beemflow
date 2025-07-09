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

// Common pattern: YAML string input → Result output
func handleYamlInput(args []js.Value, processor func(string) (interface{}, error)) any {
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
	return handleYamlInput(args, func(yamlStr string) (interface{}, error) {
		return dsl.ParseFromString(yamlStr)
	})
}

func validateYaml(this js.Value, args []js.Value) any {
	return handleYamlInput(args, func(yamlStr string) (interface{}, error) {
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
	return handleYamlInput(args, func(yamlStr string) (interface{}, error) {
		flow, err := dsl.ParseFromString(yamlStr)
		if err != nil {
			return nil, err
		}
		
		return graph.ExportMermaid(flow)
	})
}

func yamlToVisual(this js.Value, args []js.Value) any {
	return handleYamlInput(args, func(yamlStr string) (interface{}, error) {
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

	visualData := jsValueToMap(args[0])
	flow, err := dsl.VisualToFlow(visualData)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	yamlStr, err := dsl.FlowToYAMLString(flow)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: yamlStr})
}

// Convert JS Value to Go map (transport layer only)
func jsValueToMap(jsVal js.Value) map[string]interface{} {
	result := make(map[string]interface{})
	
	keys := js.Global().Get("Object").Call("keys", jsVal)
	for i := 0; i < keys.Length(); i++ {
		key := keys.Index(i).String()
		value := jsVal.Get(key)
		result[key] = jsValueToInterface(value)
	}
	
	return result
}

// Convert JS Value to Go interface{} (recursive helper)
func jsValueToInterface(jsVal js.Value) interface{} {
	switch jsVal.Type() {
	case js.TypeString:
		return jsVal.String()
	case js.TypeNumber:
		return jsVal.Float()
	case js.TypeBoolean:
		return jsVal.Bool()
	case js.TypeObject:
		if jsVal.Get("length").Type() == js.TypeNumber {
			// Handle arrays
			length := jsVal.Get("length").Int()
			arr := make([]interface{}, length)
			for j := 0; j < length; j++ {
				arr[j] = jsValueToInterface(jsVal.Index(j))
			}
			return arr
		}
		// Handle objects
		return jsValueToMap(jsVal)
	default:
		return jsVal.String()
	}
}

// Convert Result to JavaScript object
func resultToJS(r Result) map[string]interface{} {
	jsonBytes, _ := json.Marshal(r)
	var jsResult map[string]interface{}
	json.Unmarshal(jsonBytes, &jsResult)
	return jsResult
}