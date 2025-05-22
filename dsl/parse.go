package dsl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	pproto "github.com/awantoch/beemflow/spec/proto"
	ghodssyaml "github.com/ghodss/yaml"
	"google.golang.org/protobuf/encoding/protojson"
)

// Parse reads a YAML flow file from the given path and unmarshals it into a proto.Flow.
func Parse(path string) (*pproto.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFromString(string(data))
}

// ParseFromString parses a YAML string into a proto.Flow.
func ParseFromString(yamlStr string) (*pproto.Flow, error) {
	// Decode into generic map to allow shorthand triggers
	var raw map[string]interface{}
	if err := ghodssyaml.Unmarshal([]byte(yamlStr), &raw); err != nil {
		return nil, err
	}
	// Early return for MCP event trigger shorthand
	if onVal, ok := raw["on"]; ok {
		if list, ok2 := onVal.([]interface{}); ok2 && len(list) > 0 {
			if m0, ok3 := list[0].(map[string]interface{}); ok3 {
				if src, ok4 := m0["event"].(string); ok4 {
					return &pproto.Flow{On: &pproto.Trigger{Kind: &pproto.Trigger_McpEvent{McpEvent: &pproto.McpEventTrigger{Source: src}}}}, nil
				}
			}
		}
	}
	// Handle shorthand triggers using JSON field names
	if onVal, ok := raw["on"]; ok {
		switch v := onVal.(type) {
		case bool:
			// on: true shorthand -> cli.manual
			if v {
				raw["on"] = map[string]interface{}{"cliManual": map[string]interface{}{}}
			}
		case string:
			// github-actions style: on: cli.manual
			if v == "cli.manual" {
				raw["on"] = map[string]interface{}{"cliManual": map[string]interface{}{}}
			}
		case []interface{}:
			// e.g. on: [{event: source}]
			if len(v) > 0 {
				if m, ok2 := v[0].(map[string]interface{}); ok2 {
					if evt, ok3 := m["event"].(string); ok3 {
						raw["on"] = map[string]interface{}{"mcpEvent": map[string]interface{}{"source": evt}}
					}
				}
			}
		}
	}
	// Support YAML-first step shorthands recursively: 'use'/'with' -> 'exec'; 'parallel: true' -> object
	var normalizeSteps func([]interface{})
	normalizeSteps = func(steps []interface{}) {
		for _, s := range steps {
			stepMap, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			// Normalize oneof keys to proto JSON field names
			if v, ok := stepMap["await_event"]; ok {
				stepMap["awaitEvent"] = v
				delete(stepMap, "await_event")
			}
			// Map YAML 'if' to proto 'condition'
			if v, ok := stepMap["if"]; ok {
				stepMap["condition"] = v
				delete(stepMap, "if")
			}
			if v, ok := stepMap["foreach"]; ok {
				stepMap["foreach"] = v
				delete(stepMap, "foreach")
			}
			if v, ok := stepMap["wait"]; ok {
				stepMap["wait"] = v
				delete(stepMap, "wait")
			}
			if v, ok := stepMap["parallel"]; ok {
				stepMap["parallel"] = v
				// don't delete, already normalized
			}
			if v, ok := stepMap["exec"]; ok {
				stepMap["exec"] = v
				// don't delete, already normalized
			}
			// Exec shorthand
			if useVal, exists := stepMap["use"]; exists {
				execMap := map[string]interface{}{"use": useVal}
				if withVal, ok3 := stepMap["with"]; ok3 {
					execMap["with"] = withVal
					delete(stepMap, "with")
				}
				stepMap["exec"] = execMap
				delete(stepMap, "use")
			}
			// Parallel shorthand: boolean true + sibling 'steps'
			if parallelVal, exists := stepMap["parallel"]; exists {
				if pb, ok3 := parallelVal.(bool); ok3 && pb {
					if nested, ok4 := stepMap["steps"]; ok4 {
						stepMap["parallel"] = map[string]interface{}{"steps": nested}
						delete(stepMap, "steps")
					} else {
						// If no steps, create empty steps array
						stepMap["parallel"] = map[string]interface{}{"steps": []interface{}{}}
					}
				}
			}
			// Recurse into nested parallel blocks
			if p, exists := stepMap["parallel"]; exists {
				if pMap, ok := p.(map[string]interface{}); ok {
					if nested, ok2 := pMap["steps"].([]interface{}); ok2 {
						normalizeSteps(nested)
					}
				}
			}
			// Normalize foreach block to proto shape
			if listExpr, ok := stepMap["foreach"]; ok {
				// Extract variable name from Jinja-style expression
				exprStr := ""
				switch v := listExpr.(type) {
				case string:
					// If wrapped in {{ }}, strip braces
					if strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}") {
						exprStr = strings.TrimSpace(v[2 : len(v)-2])
					} else {
						exprStr = v
					}
				default:
					// Leave raw as string for other types
					exprStr = fmt.Sprintf("%v", v)
				}
				foreachObj := map[string]interface{}{"list_expr": exprStr}
				if asVal, ok := stepMap["as"]; ok {
					foreachObj["alias"] = asVal
				}
				if doVal, ok := stepMap["do"]; ok {
					foreachObj["steps"] = doVal
				} else {
					foreachObj["steps"] = []interface{}{}
				}
				// Build a new normalized step map
				normStep := map[string]interface{}{"id": stepMap["id"], "foreach": foreachObj}
				if v, ok := stepMap["depends_on"]; ok {
					normStep["depends_on"] = v
				}
				if v, ok := stepMap["catch"]; ok {
					normStep["catch"] = v
				}
				for k, v := range stepMap {
					if k != "id" && k != "foreach" && k != "as" && k != "do" && k != "depends_on" && k != "catch" {
						normStep[k] = v
					}
				}
				// Replace the original step map with the normalized one
				for k := range stepMap {
					delete(stepMap, k)
				}
				for k, v := range normStep {
					stepMap[k] = v
				}
			}
			// Remove any leftover 'as' or 'do' keys at the step level
			delete(stepMap, "as")
			delete(stepMap, "do")
			// Recurse into catch blocks (array of steps)
			if catchVal, exists := stepMap["catch"].([]interface{}); exists {
				normalizeSteps(catchVal)
			}
			// After normalization, ensure required fields for each step type
			if exec, ok := stepMap["exec"].(map[string]interface{}); ok {
				if _, ok := exec["use"]; !ok {
					exec["use"] = ""
				}
			}
			if foreach, ok := stepMap["foreach"].(map[string]interface{}); ok {
				if _, ok := foreach["list_expr"]; !ok {
					foreach["list_expr"] = ""
				}
				if _, ok := foreach["do"]; !ok {
					foreach["do"] = []interface{}{}
				}
			}
			if awaitEvent, ok := stepMap["awaitEvent"].(map[string]interface{}); ok {
				if _, ok := awaitEvent["match"]; !ok {
					awaitEvent["match"] = map[string]interface{}{}
				}
				if _, ok := awaitEvent["source"]; !ok {
					awaitEvent["source"] = ""
				}
			}
			if wait, ok := stepMap["wait"].(map[string]interface{}); ok {
				if _, ok1 := wait["seconds"]; !ok1 {
					if _, ok2 := wait["until"]; !ok2 {
						wait["seconds"] = 0
					}
				}
			}
			if parallel, ok := stepMap["parallel"].(map[string]interface{}); ok {
				if _, ok := parallel["steps"]; !ok {
					parallel["steps"] = []interface{}{}
				}
			}
		}
	}
	if stepsVal, ok := raw["steps"].([]interface{}); ok {
		normalizeSteps(stepsVal)
	}
	// Also normalize top-level catch block if present
	if catchVal, ok := raw["catch"].([]interface{}); ok {
		normalizeSteps(catchVal)
	}
	// Remove any top-level 'true' key left over from YAML parsing
	if _, ok := raw["true"]; ok {
		delete(raw, "true")
	}
	// Wrap vars and exec.with values into proto.Value JSON shapes
	wrapMap := func(m map[string]interface{}) {
		for k, v := range m {
			switch x := v.(type) {
			case string:
				m[k] = map[string]interface{}{"s": x}
			case bool:
				m[k] = map[string]interface{}{"b": x}
			case int:
				m[k] = map[string]interface{}{"n": float64(x)}
			case float64:
				m[k] = map[string]interface{}{"n": x}
			default:
				// leave other types as is
			}
		}
	}
	// Top-level vars
	if varsVal, ok := raw["vars"].(map[string]interface{}); ok {
		wrapMap(varsVal)
	}
	// Wrap each step recursively
	var wrapSteps func([]interface{})
	wrapSteps = func(steps []interface{}) {
		for _, s := range steps {
			if stepMap, ok := s.(map[string]interface{}); ok {
				// Wrap exec.with
				if execVal, exists := stepMap["exec"]; exists {
					if execMap, ok2 := execVal.(map[string]interface{}); ok2 {
						if withVal, ok3 := execMap["with"].(map[string]interface{}); ok3 {
							wrapMap(withVal)
						}
					}
				}
				// Parallel nested
				if p, ok := stepMap["parallel"].(map[string]interface{}); ok {
					if nested, ok2 := p["steps"].([]interface{}); ok2 {
						wrapSteps(nested)
					}
				}
				// Foreach nested
				if f, ok := stepMap["foreach"].(map[string]interface{}); ok {
					if nested, ok2 := f["steps"].([]interface{}); ok2 {
						wrapSteps(nested)
					}
				}
				// Catch nested
				if catchVal, ok := stepMap["catch"].([]interface{}); ok {
					wrapSteps(catchVal)
				}
			}
		}
	}
	if stepsVal, ok := raw["steps"].([]interface{}); ok {
		wrapSteps(stepsVal)
	}
	// Marshal back to JSON for proto unmarshal
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var fb pproto.Flow
	if err := protojson.Unmarshal(jsonBytes, &fb); err != nil {
		return nil, err
	}
	// Default missing trigger to CLI manual
	if fb.On == nil {
		fb.On = &pproto.Trigger{Kind: &pproto.Trigger_CliManual{}}
	}
	// NOTE: skipping schema validation here to allow YAML-first shapes
	return &fb, nil
}
