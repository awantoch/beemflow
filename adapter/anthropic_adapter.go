package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/awantoch/beemflow/registry"
)

// AnthropicAdapter handles Anthropic Claude API calls
type AnthropicAdapter struct{}

// ID returns the adapter ID
func (a *AnthropicAdapter) ID() string {
	return "anthropic"
}

// Anthropic API structures
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Execute handles Anthropic tool execution
func (a *AnthropicAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	use, ok := inputs["__use"].(string)
	if !ok {
		return nil, fmt.Errorf("missing __use for AnthropicAdapter")
	}

	switch use {
	case "anthropic.chat_completion":
		return a.executeChatCompletion(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown Anthropic tool: %s", use)
	}
}

// executeChatCompletion handles chat completion requests
func (a *AnthropicAdapter) executeChatCompletion(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}

	// Extract required parameters
	model, ok := inputs["model"].(string)
	if !ok || model == "" {
		model = "claude-3-haiku-20240307" // Default model
	}

	messagesRaw, ok := inputs["messages"]
	if !ok {
		return nil, fmt.Errorf("messages parameter is required")
	}

	// Convert messages to Anthropic format
	messages, systemPrompt, err := a.convertMessages(messagesRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid messages format: %w", err)
	}

	// Build request
	req := AnthropicRequest{
		Model:     model,
		MaxTokens: 1024, // Default max tokens
		Messages:  messages,
		System:    systemPrompt,
	}

	// Optional parameters
	if maxTokens, ok := inputs["max_tokens"].(float64); ok {
		req.MaxTokens = int(maxTokens)
	}

	// Make API call
	return a.callAnthropic(ctx, apiKey, req)
}

// convertMessages converts various message formats to Anthropic format
func (a *AnthropicAdapter) convertMessages(messagesRaw any) ([]AnthropicMessage, string, error) {
	var messages []AnthropicMessage
	var systemPrompt string

	switch v := messagesRaw.(type) {
	case []any:
		for _, msgRaw := range v {
			if msgMap, ok := msgRaw.(map[string]any); ok {
				role, hasRole := msgMap["role"].(string)
				content, hasContent := msgMap["content"].(string)
				
				if !hasRole || !hasContent {
					return nil, "", fmt.Errorf("each message must have 'role' and 'content' fields")
				}
				
				// Handle system messages separately for Anthropic
				if role == "system" {
					systemPrompt = content
				} else {
					messages = append(messages, AnthropicMessage{
						Role:    role,
						Content: content,
					})
				}
			} else {
				return nil, "", fmt.Errorf("each message must be an object")
			}
		}
	case []map[string]any:
		for _, msgMap := range v {
			role, hasRole := msgMap["role"].(string)
			content, hasContent := msgMap["content"].(string)
			
			if !hasRole || !hasContent {
				return nil, "", fmt.Errorf("each message must have 'role' and 'content' fields")
			}
			
			// Handle system messages separately for Anthropic
			if role == "system" {
				systemPrompt = content
			} else {
				messages = append(messages, AnthropicMessage{
					Role:    role,
					Content: content,
				})
			}
		}
	default:
		return nil, "", fmt.Errorf("messages must be an array of objects")
	}

	if len(messages) == 0 {
		return nil, "", fmt.Errorf("at least one non-system message is required")
	}

	return messages, systemPrompt, nil
}

// callAnthropic makes the actual API call to Anthropic
func (a *AnthropicAdapter) callAnthropic(ctx context.Context, apiKey string, req AnthropicRequest) (map[string]any, error) {
	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Make request with timeout
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Anthropic API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var anthropicResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode Anthropic response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Anthropic API error: status %d", resp.StatusCode)
	}

	// Convert to standard format compatible with OpenAI structure
	return map[string]any{
		"id":      anthropicResp.ID,
		"object":  "chat.completion",
		"model":   anthropicResp.Model,
		"content": anthropicResp.Content,
		"usage":   anthropicResp.Usage,
	}, nil
}

// Manifest returns the tool manifest
func (a *AnthropicAdapter) Manifest() *registry.ToolManifest {
	return nil
}