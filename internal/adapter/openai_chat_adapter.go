package adapter

import (
	"context"
	"fmt"
)

// OpenAIAdapter implements Adapter for OpenAI's Chat Completions API (v1/chat/completions).
type OpenAIAdapter struct {
	ManifestField *ToolManifest
}

// ID returns the adapter ID.
func (a *OpenAIAdapter) ID() string {
	return "openai"
}

// Execute calls the OpenAI chat completions API and returns the full JSON response.
func (a *OpenAIAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	apiKey, ok := inputs["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("openai: missing api_key")
	}
	model, ok := inputs["model"].(string)
	if !ok || model == "" {
		return nil, fmt.Errorf("openai: missing model")
	}
	messages, ok := inputs["messages"].([]any)
	if !ok || len(messages) == 0 {
		return nil, fmt.Errorf("openai: missing messages")
	}
	// Build request body including all parameters except api_key
	reqBody := make(map[string]any)
	for k, v := range inputs {
		if k == "api_key" {
			continue
		}
		reqBody[k] = v
	}
	var out map[string]any
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	err := HTTPPostJSON(ctx, "https://api.openai.com/v1/chat/completions", reqBody, headers, &out)
	return out, err
}

func (a *OpenAIAdapter) Manifest() *ToolManifest {
	return a.ManifestField
}
