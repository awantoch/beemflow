package adapter

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/awantoch/beemflow/registry"
    "gopkg.in/yaml.v2"
)

// BeemBeemAdapter implements conversational helper utilities that aren't normal HTTP/MCP calls.
// It is intentionally minimal – most logic lives in BeemFlow YAML workflows that call these
// helpers for validation / persistence.
//
// Available actions (via `__use`):
//   • beembeem.validate_workflow
//   • beembeem.save_workflow
//   • beembeem.continue_chat   (stub – placeholder for future context mgmt)
//
// IMPORTANT: this adapter does *no* external I/O except local disk writes.
// That keeps it safe to run in serverless / sandbox environments.

type BeemBeemAdapter struct{}

func (b *BeemBeemAdapter) ID() string { return "beembeem" }

func (b *BeemBeemAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    use, _ := inputs["__use"].(string)

    switch use {
    case "beembeem.validate_workflow":
        return b.validate(inputs)
    case "beembeem.save_workflow":
        return b.save(inputs)
    case "beembeem.continue_chat":
        return map[string]any{"status": "ok"}, nil
    default:
        return nil, fmt.Errorf("unknown beembeem action: %s", use)
    }
}

func (b *BeemBeemAdapter) validate(inputs map[string]any) (map[string]any, error) {
    yamlStr, _ := inputs["workflow_yaml"].(string)
    var m map[string]any
    if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
        return map[string]any{"valid": false, "error": err.Error()}, nil
    }
    if _, ok := m["name"]; !ok {
        return map[string]any{"valid": false, "error": "missing name"}, nil
    }
    if _, ok := m["steps"]; !ok {
        return map[string]any{"valid": false, "error": "missing steps"}, nil
    }
    return map[string]any{"valid": true}, nil
}

func (b *BeemBeemAdapter) save(inputs map[string]any) (map[string]any, error) {
    yamlStr, _ := inputs["workflow_yaml"].(string)
    wname, _ := inputs["workflow_name"].(string)
    if wname == "" {
        return nil, fmt.Errorf("workflow_name required")
    }
    filename := strings.ToLower(strings.ReplaceAll(wname, " ", "_")) + ".flow.yaml"
    dir := filepath.Join("flows", "generated")
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, err
    }
    path := filepath.Join(dir, filename)
    if err := os.WriteFile(path, []byte(yamlStr), 0644); err != nil {
        return nil, err
    }
    return map[string]any{"saved": true, "path": path}, nil
}

func (b *BeemBeemAdapter) Manifest() *registry.ToolManifest {
    return &registry.ToolManifest{
        Name:        "beembeem",
        Description: "Internal helper for BeemBeem conversational workflows",
        Kind:        "task",
    }
}