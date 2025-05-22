package api

import (
	"context"
	"encoding/json"
	"os"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/google/uuid"
)

// FlowService defines the full API surface for flows and runs.
type FlowService interface {
	ListFlows(ctx context.Context) ([]string, error)
	GetFlow(ctx context.Context, name string) (pproto.Flow, error)
	ValidateFlow(ctx context.Context, name string) error
	GraphFlow(ctx context.Context, name string) (string, error)
	StartRun(ctx context.Context, flowName string, event map[string]any) (uuid.UUID, error)
	GetRun(ctx context.Context, runID uuid.UUID) (*pproto.Run, error)
	ListRuns(ctx context.Context) ([]*pproto.Run, error)
	DeleteRun(ctx context.Context, id uuid.UUID) error
	PublishEvent(ctx context.Context, topic string, payload map[string]any) error
	ResumeRun(ctx context.Context, token string, event map[string]any) (map[string]any, error)
	RunSpec(ctx context.Context, flow *pproto.Flow, event map[string]any) (uuid.UUID, map[string]any, error)
	AssistantChat(ctx context.Context, systemPrompt string, userMessages []string) (string, []string, error)
	ListTools(ctx context.Context) ([]registry.ToolManifest, error)
	GetToolManifest(ctx context.Context, name string) (*registry.ToolManifest, error)
}

// defaultService is the default implementation of FlowService.
type defaultService struct{}

// Compile-time check
var _ FlowService = (*defaultService)(nil)

// NewFlowService returns the default FlowService implementation.
func NewFlowService() FlowService {
	return &defaultService{}
}

func (s *defaultService) ListFlows(ctx context.Context) ([]string, error) {
	return ListFlows(ctx)
}
func (s *defaultService) GetFlow(ctx context.Context, name string) (pproto.Flow, error) {
	return GetFlow(ctx, name)
}
func (s *defaultService) ValidateFlow(ctx context.Context, name string) error {
	return ValidateFlow(ctx, name)
}
func (s *defaultService) GraphFlow(ctx context.Context, name string) (string, error) {
	return GraphFlow(ctx, name)
}
func (s *defaultService) StartRun(ctx context.Context, flowName string, event map[string]any) (uuid.UUID, error) {
	return StartRun(ctx, flowName, event)
}
func (s *defaultService) GetRun(ctx context.Context, runID uuid.UUID) (*pproto.Run, error) {
	return GetRun(ctx, runID)
}
func (s *defaultService) ListRuns(ctx context.Context) ([]*pproto.Run, error) {
	return ListRuns(ctx)
}
func (s *defaultService) DeleteRun(ctx context.Context, id uuid.UUID) error {
	// Delegate to underlying storage via inline flow spec: remove direct run storage
	// For HTTP, CLI, MCP, this is implemented in HTTP handler, so just no-op here
	return nil
}
func (s *defaultService) PublishEvent(ctx context.Context, topic string, payload map[string]any) error {
	return PublishEvent(ctx, topic, payload)
}
func (s *defaultService) ResumeRun(ctx context.Context, token string, event map[string]any) (map[string]any, error) {
	return ResumeRun(ctx, token, event)
}
func (s *defaultService) RunSpec(ctx context.Context, flow *pproto.Flow, event map[string]any) (uuid.UUID, map[string]any, error) {
	return RunSpec(ctx, flow, event)
}
func (s *defaultService) AssistantChat(ctx context.Context, systemPrompt string, userMessages []string) (string, []string, error) {
	// Use parser for systemPrompt and adapter for call
	draft, errs, err := adapter.Execute(ctx, userMessages)
	return draft, errs, err
}
func (s *defaultService) ListTools(ctx context.Context) ([]registry.ToolManifest, error) {
	data, err := os.ReadFile("registry/index.json")
	if err != nil {
		return nil, err
	}
	var entries []registry.ToolManifest
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
func (s *defaultService) GetToolManifest(ctx context.Context, name string) (*registry.ToolManifest, error) {
	entries, err := s.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Name == name {
			return &entry, nil
		}
	}
	return nil, nil
}
