package storage

import (
	"context"
	"database/sql"
	"sync"

	"github.com/awantoch/beemflow/model"
	"github.com/google/uuid"
)

// MemoryStorage implements Storage in-memory (for fallback/dev mode)
type MemoryStorage struct {
	runs   map[uuid.UUID]*model.Run
	steps  map[uuid.UUID][]*model.StepRun // runID -> steps
	mu     sync.Mutex
	paused map[string]any // token -> paused run
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		runs:   make(map[uuid.UUID]*model.Run),
		steps:  make(map[uuid.UUID][]*model.StepRun),
		paused: make(map[string]any),
	}
}

func (m *MemoryStorage) SaveRun(ctx context.Context, run *model.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.ID] = run
	return nil
}

func (m *MemoryStorage) GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return run, nil
}

func (m *MemoryStorage) SaveStep(ctx context.Context, step *model.StepRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.steps[step.RunID] = append(m.steps[step.RunID], step)
	return nil
}

func (m *MemoryStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*model.StepRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.steps[runID], nil
}

func (m *MemoryStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	return nil
}

func (m *MemoryStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*model.Run, error) {
	return nil, nil
}

func (m *MemoryStorage) ListRuns(ctx context.Context) ([]*model.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*model.Run
	for _, run := range m.runs {
		out = append(out, run)
	}
	return out, nil
}

func (m *MemoryStorage) SavePausedRun(token string, paused any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused[token] = paused
	return nil
}

func (m *MemoryStorage) LoadPausedRuns() (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]any, len(m.paused))
	for k, v := range m.paused {
		out[k] = v
	}
	return out, nil
}

func (m *MemoryStorage) DeletePausedRun(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.paused, token)
	return nil
}
