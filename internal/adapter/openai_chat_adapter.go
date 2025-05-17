package adapter

import (
	"context"
	"fmt"
)

// OpenAIChatAdapter implements Adapter for openai.chat.
type OpenAIChatAdapter struct {
	ManifestField *ToolManifest
}

// ID returns the adapter ID.
func (a *OpenAIChatAdapter) ID() string {
	return "openai.chat"
}

// Execute calls the OpenAI chat completions API and returns the full JSON response.
func (a *OpenAIChatAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	apiKey, ok := inputs["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("openai.chat: missing api_key")
	}
	model, ok := inputs["model"].(string)
	if !ok || model == "" {
		return nil, fmt.Errorf("openai.chat: missing model")
	}
	messages, ok := inputs["messages"].([]any)
	if !ok || len(messages) == 0 {
		return nil, fmt.Errorf("openai.chat: missing messages")
	}
	reqBody := map[string]any{
		"model":    model,
		"messages": messages,
	}
	var out map[string]any
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	err := HTTPPostJSON(ctx, "https://api.openai.com/v1/chat/completions", reqBody, headers, &out)
	return out, err
}

func (a *OpenAIChatAdapter) Manifest() *ToolManifest {
	return a.ManifestField
}
