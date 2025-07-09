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

	// Use dsl function
	flow, err := dsl.ParseFromString(yamlStr)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: flow})
}

func validateYaml(this js.Value, args []js.Value) any {
	yamlStr, errResult := getYamlFromArgs(args)
	if errResult != nil {
		return resultToJS(*errResult)
	}

	// Use dsl functions
	flow, err := dsl.ParseFromString(yamlStr)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

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

	// Use dsl and graph functions
	flow, err := dsl.ParseFromString(yamlStr)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

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

	// Use dsl functions
	flow, err := dsl.ParseFromString(yamlStr)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	visualData, err := dsl.FlowToVisual(flow)
	if err != nil {
		return resultToJS(Result{Success: false, Error: err.Error()})
	}

	return resultToJS(Result{Success: true, Data: visualData})
}

func visualToYaml(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return resultToJS(Result{Success: false, Error: "No visual data provided"})
	}

	// Convert JS object to Go map
	visualData := jsValueToMap(args[0])
	
	// Use dsl functions
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

// Convert JS Value to Go map (minimal implementation for transport)
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
			if value.Get("length").Type() == js.TypeNumber {
				// Handle arrays
				length := value.Get("length").Int()
				arr := make([]interface{}, length)
				for j := 0; j < length; j++ {
					arr[j] = jsValueToInterface(value.Index(j))
				}
				result[key] = arr
			} else {
				// Handle objects
				result[key] = jsValueToMap(value)
			}
		default:
			result[key] = value.String()
		}
	}
	
	return result
}

// Convert JS Value to Go interface{} (helper for arrays)
func jsValueToInterface(jsVal js.Value) interface{} {
	switch jsVal.Type() {
	case js.TypeString:
		return jsVal.String()
	case js.TypeNumber:
		return jsVal.Float()
	case js.TypeBoolean:
		return jsVal.Bool()
	case js.TypeObject:
		return jsValueToMap(jsVal)
	default:
		return jsVal.String()
	}
}

// Convert Result to JavaScript object
func resultToJS(r Result) map[string]interface{} {
	// Use Go's built-in JSON marshaling and unmarshaling
	jsonBytes, _ := json.Marshal(r)
	var jsResult map[string]interface{}
	json.Unmarshal(jsonBytes, &jsResult)
	return jsResult
}