# BeemFlow Visual Editor Architecture Improvements

## Problem Identified
The WASM layer contained significant application logic (332 lines) including:
- Custom visual conversion functions (`graphNodesToReactFlow`, `graphEdgesToReactFlow`)
- Complex JavaScript-to-Go data transformation logic
- Duplicate functionality not leveraging existing BeemFlow functions

## Solution: Proper Separation of Concerns

### 1. Enhanced Graph Package (`graph/graph.go`)
- **Added `ReactFlowRenderer`**: Following the same pattern as `MermaidRenderer`
- **Added `ExportReactFlow()`**: Converts Flow → React Flow format using existing graph infrastructure
- **Maintains consistency**: Uses same `NewGraph()` → `Renderer` pattern

### 2. Enhanced DSL Package (`dsl/generate.go`)
- **Added `FlowToVisual()`**: Thin wrapper around `graph.ExportReactFlow()`
- **Added `VisualToFlow()`**: Converts React Flow format → Flow using proper validation
- **All business logic**: Stays in main runtime, not WASM

### 3. Ultra-Thin WASM Layer (`editor/wasm/main.go`)
**Reduced from 332 lines to 199 lines (40% reduction)**

**Before:**
```go
// Custom logic in WASM
func yamlToVisual(this js.Value, args []js.Value) any {
    // ... parsing logic ...
    g := graph.NewGraph(flow)
    // ... custom conversion logic ...
    visualData := map[string]interface{}{
        "nodes": graphNodesToReactFlow(g.Nodes, flow),
        "edges": graphEdgesToReactFlow(g.Edges),
        "flow":  flow,
    }
    return resultToJS(Result{Success: true, Data: visualData})
}
```

**After:**
```go
// Thin transport layer
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
```

## Key Benefits

### 1. **Proper Architecture**
- ✅ WASM is just a build target, not a separate application
- ✅ All business logic in main runtime (`dsl`, `graph` packages)
- ✅ WASM layer is pure transport/marshaling

### 2. **Maintainability**
- ✅ Single source of truth for visual conversion logic
- ✅ Easier to test (Go tests vs WASM tests)
- ✅ Consistent with existing BeemFlow patterns

### 3. **Extensibility**
- ✅ Easy to add new renderers (following `Renderer` interface)
- ✅ Visual conversion logic can be used by other parts of BeemFlow
- ✅ No duplication between WASM and main runtime

### 4. **Testing**
- ✅ Added comprehensive tests for `FlowToVisual` and `VisualToFlow`
- ✅ Round-trip testing ensures data integrity
- ✅ All tests pass

## Functions Added

### `graph/graph.go`
- `ReactFlowRenderer` struct
- `ExportReactFlow(flow *model.Flow) (map[string]interface{}, error)`

### `dsl/generate.go`
- `FlowToVisual(flow *model.Flow) (map[string]interface{}, error)`
- `VisualToFlow(visualData map[string]interface{}) (*model.Flow, error)`
- `getString(data map[string]interface{}, key string) string` (helper)

### `dsl/generate_test.go`
- `TestFlowToVisual()`
- `TestVisualToFlow()`
- `TestVisualToFlowRoundTrip()`

## Result
- **40% reduction in WASM code** (332 → 199 lines)
- **Proper separation of concerns**
- **Leverages existing BeemFlow infrastructure**
- **All tests passing**
- **WASM build successful** (12.9MB)

This architecture now follows the principle: "WASM is just a build target, not a separate application."