package api

import (
	"context"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/registry"
	"github.com/google/uuid"
)

// FlowService defines the full API surface for flows and runs.
type FlowService interface {
	ListFlows(ctx context.Context) ([]string, error)
	GetFlow(ctx context.Context, name string) (model.Flow, error)
	ValidateFlow(ctx context.Context, name string) error
	GraphFlow(ctx context.Context, name string) (string, error)
	StartRun(ctx context.Context, flowName string, event map[string]any) (uuid.UUID, error)
	GetRun(ctx context.Context, runID uuid.UUID) (*model.Run, error)
	ListRuns(ctx context.Context) ([]*model.Run, error)
	DeleteRun(ctx context.Context, id uuid.UUID) error
	PublishEvent(ctx context.Context, topic string, payload map[string]any) error
	ResumeRun(ctx context.Context, token string, event map[string]any) (map[string]any, error)
	RunSpec(ctx context.Context, flow *model.Flow, event map[string]any) (uuid.UUID, map[string]any, error)
	ListTools(ctx context.Context) ([]registry.ToolManifest, error)
	GetToolManifest(ctx context.Context, name string) (*registry.ToolManifest, error)
}

// defaultService is the default implementation of FlowService.
type defaultService struct{}

// Compile-time check.
var _ FlowService = (*defaultService)(nil)

// NewFlowService returns the default FlowService implementation.
func NewFlowService() FlowService {
	return &defaultService{}
}

func (s *defaultService) ListFlows(ctx context.Context) ([]string, error) {
	return ListFlows(ctx)
}
func (s *defaultService) GetFlow(ctx context.Context, name string) (model.Flow, error) {
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
func (s *defaultService) GetRun(ctx context.Context, runID uuid.UUID) (*model.Run, error) {
	return GetRun(ctx, runID)
}
func (s *defaultService) ListRuns(ctx context.Context) ([]*model.Run, error) {
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
func (s *defaultService) RunSpec(ctx context.Context, flow *model.Flow, event map[string]any) (uuid.UUID, map[string]any, error) {
	return RunSpec(ctx, flow, event)
}
func (s *defaultService) ListTools(ctx context.Context) ([]registry.ToolManifest, error) {
	// Load tool manifests from the local registry index
	local := registry.NewLocalRegistry("")
	entries, err := local.ListServers(ctx, registry.ListOptions{})
	if err != nil {
		return nil, err
	}
	var manifests []registry.ToolManifest
	for _, entry := range entries {
		manifests = append(manifests, registry.ToolManifest{
			Name:        entry.Name,
			Description: entry.Description,
			Kind:        entry.Kind,
			Parameters:  entry.Parameters,
			Endpoint:    entry.Endpoint,
			Headers:     entry.Headers,
		})
	}
	return manifests, nil
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
