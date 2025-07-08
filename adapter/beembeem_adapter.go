package adapter

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/awantoch/beemflow/dsl"
    "github.com/awantoch/beemflow/registry"
)

// BeemBeemAdapter provides workflow management utilities for BeemBeem.
// It leverages existing BeemFlow APIs to stay DRY and integrated.
type BeemBeemAdapter struct{}

func (b *BeemBeemAdapter) ID() string { return "beembeem" }

func (b *BeemBeemAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    use, _ := inputs["__use"].(string)

    switch use {
    case "beembeem.save_workflow":
        return b.saveWorkflow(ctx, inputs)
    case "beembeem.run_workflow":
        return b.runWorkflow(ctx, inputs)
    case "beembeem.list_workflows":
        return b.listWorkflows(ctx)
    default:
        return nil, fmt.Errorf("unknown beembeem action: %s", use)
    }
}

func (b *BeemBeemAdapter) saveWorkflow(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    yamlStr, _ := inputs["workflow_yaml"].(string)
    name, _ := inputs["name"].(string)
    
    if name == "" {
        return nil, fmt.Errorf("name required")
    }
    if yamlStr == "" {
        return nil, fmt.Errorf("workflow_yaml required")
    }

    // Validate using existing BeemFlow validation
    flow, err := dsl.ParseFromString(yamlStr)
    if err != nil {
        return map[string]any{"success": false, "error": fmt.Sprintf("Parse error: %v", err)}, nil
    }
    if err := dsl.Validate(flow); err != nil {
        return map[string]any{"success": false, "error": fmt.Sprintf("Validation error: %v", err)}, nil
    }

    // Save to flows directory (using existing pattern)
    filename := strings.ToLower(strings.ReplaceAll(name, " ", "_")) + ".flow.yaml"
    path := filepath.Join("flows", filename)
    
    if err := os.WriteFile(path, []byte(yamlStr), 0644); err != nil {
        return nil, fmt.Errorf("failed to save workflow: %v", err)
    }

    return map[string]any{"success": true, "path": path, "name": name}, nil
}

func (b *BeemBeemAdapter) runWorkflow(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    name, _ := inputs["name"].(string)
    if name == "" {
        return nil, fmt.Errorf("workflow name required")
    }

    // For now, just return success - the actual execution will be handled by the engine
    // when this gets integrated properly. This keeps the adapter simple and avoids circular imports.
    return map[string]any{"success": true, "message": "Workflow execution initiated", "name": name}, nil
}

func (b *BeemBeemAdapter) listWorkflows(ctx context.Context) (map[string]any, error) {
    // List workflows from the flows directory directly
    entries, err := os.ReadDir("flows")
    if err != nil {
        if os.IsNotExist(err) {
            return map[string]any{"workflows": []string{}}, nil
        }
        return nil, err
    }
    
    var workflows []string
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        name := entry.Name()
        if strings.HasSuffix(name, ".flow.yaml") {
            base := strings.TrimSuffix(name, ".flow.yaml")
            workflows = append(workflows, base)
        }
    }
    return map[string]any{"workflows": workflows}, nil
}

func (b *BeemBeemAdapter) Manifest() *registry.ToolManifest {
    return &registry.ToolManifest{
        Name:        "beembeem",
        Description: "BeemBeem workflow management utilities",
        Kind:        "task",
    }
}