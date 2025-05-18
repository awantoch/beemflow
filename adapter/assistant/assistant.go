package assistant

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/awantoch/beemflow/parser"
)

//go:embed system_prompt.md
var SystemPrompt string

const schemaPath = "beemflow.schema.json"

// LLMMessage represents a chat message for the LLM.
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CallLLM is a package-level variable for LLM calls, allowing test overrides.
var CallLLM = callLLMImpl

func callLLMImpl(ctx context.Context, systemPrompt string, userMessages []string) (string, error) {
	// TODO: Implement actual LLM call (OpenAI, etc.)
	return "", fmt.Errorf("LLM call not implemented")
}

// Execute builds messages, calls LLM, validates YAML, and returns draft and errors.
func Execute(ctx context.Context, userMessages []string) (draftYAML string, validationErrors []string, err error) {
	// 1. Build messages: system ← embedded prompt, user ← passed-in messages

	// 2. Call LLM (OpenAI or other)
	draftYAML, err = CallLLM(ctx, SystemPrompt, userMessages)
	if err != nil {
		return "", nil, fmt.Errorf("LLM error: %w", err)
	}

	// 3. Validate returned YAML against your JSON-Schema (flow lint)
	flow, parseErr := parser.ParseFlowFromString(draftYAML)
	if parseErr != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("YAML parse error: %v", parseErr))
		return draftYAML, validationErrors, nil
	}

	schema := schemaPath
	if _, err := os.Stat(schema); os.IsNotExist(err) {
		schema = "../../beemflow.schema.json" // fallback for dev/test
	}

	if valErr := parser.ValidateFlow(flow, schema); valErr != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Schema validation error: %v", valErr))
	}

	return draftYAML, validationErrors, nil
}
