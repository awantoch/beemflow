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

// OpenAIAdapter handles OpenAI API calls
type OpenAIAdapter struct{}

// ID returns the adapter ID
func (a *OpenAIAdapter) ID() string {
	return "openai"
}

// OpenAI API structures
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream"`
}

type OpenAIChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type OpenAIResponse struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Execute handles OpenAI tool execution
func (a *OpenAIAdapter) Execute(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	use, ok := inputs["__use"].(string)
	if !ok {
		return nil, fmt.Errorf("missing __use for OpenAIAdapter")
	}

	switch use {
	case "openai.chat_completion":
		return a.executeChatCompletion(ctx, inputs)
	default:
		return nil, fmt.Errorf("unknown OpenAI tool: %s", use)
	}
}

// executeChatCompletion handles chat completion requests
func (a *OpenAIAdapter) executeChatCompletion(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	// Extract required parameters
	model, ok := inputs["model"].(string)
	if !ok || model == "" {
		model = "gpt-3.5-turbo" // Default model
	}

	messagesRaw, ok := inputs["messages"]
	if !ok {
		return nil, fmt.Errorf("messages parameter is required")
	}

	// Convert messages to OpenAI format
	messages, err := a.convertMessages(messagesRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid messages format: %w", err)
	}

	// Build request
	req := OpenAIRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	// Optional parameters
	if temp, ok := inputs["temperature"].(float64); ok {
		req.Temperature = &temp
	}
	if maxTokens, ok := inputs["max_tokens"].(float64); ok {
		maxTokensInt := int(maxTokens)
		req.MaxTokens = &maxTokensInt
	}

	// Make API call
	return a.callOpenAI(ctx, apiKey, req)
}

// convertMessages converts various message formats to OpenAI format
func (a *OpenAIAdapter) convertMessages(messagesRaw any) ([]OpenAIMessage, error) {
	var messages []OpenAIMessage

	switch v := messagesRaw.(type) {
	case []any:
		for _, msgRaw := range v {
			if msgMap, ok := msgRaw.(map[string]any); ok {
				role, hasRole := msgMap["role"].(string)
				content, hasContent := msgMap["content"].(string)
				
				if !hasRole || !hasContent {
					return nil, fmt.Errorf("each message must have 'role' and 'content' fields")
				}
				
				messages = append(messages, OpenAIMessage{
					Role:    role,
					Content: content,
				})
			} else {
				return nil, fmt.Errorf("each message must be an object")
			}
		}
	case []map[string]any:
		for _, msgMap := range v {
			role, hasRole := msgMap["role"].(string)
			content, hasContent := msgMap["content"].(string)
			
			if !hasRole || !hasContent {
				return nil, fmt.Errorf("each message must have 'role' and 'content' fields")
			}
			
			messages = append(messages, OpenAIMessage{
				Role:    role,
				Content: content,
			})
		}
	default:
		return nil, fmt.Errorf("messages must be an array of objects")
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}

	return messages, nil
}

// callOpenAI makes the actual API call to OpenAI
func (a *OpenAIAdapter) callOpenAI(ctx context.Context, apiKey string, req OpenAIRequest) (map[string]any, error) {
	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	// Make request with timeout
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var openaiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OpenAI API error: status %d", resp.StatusCode)
	}

	// Convert to standard format
	return map[string]any{
		"id":      openaiResp.ID,
		"object":  openaiResp.Object,
		"created": openaiResp.Created,
		"model":   openaiResp.Model,
		"choices": openaiResp.Choices,
		"usage":   openaiResp.Usage,
	}, nil
}

// Manifest returns the tool manifest
func (a *OpenAIAdapter) Manifest() *registry.ToolManifest {
	return nil
}