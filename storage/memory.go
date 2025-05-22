package storage

import (
	"context"
	"database/sql"
	"sync"

	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/google/uuid"
)

// MemoryStorage implements Storage in-memory (for fallback/dev mode)
type MemoryStorage struct {
	runs   map[uuid.UUID]*pproto.Run
	steps  map[uuid.UUID][]*pproto.StepRun // runID -> steps
	mu     sync.RWMutex                    // RWMutex is sufficient for most use cases; consider context-aware primitives if high concurrency or cancellation is needed.
	paused map[string]any                  // token -> paused run
}

var _ Storage = (*MemoryStorage)(nil)

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		runs:   make(map[uuid.UUID]*pproto.Run),
		steps:  make(map[uuid.UUID][]*pproto.StepRun),
		paused: make(map[string]any),
	}
}

func (m *MemoryStorage) SaveRun(ctx context.Context, run *pproto.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, err := uuid.Parse(run.Id)
	if err != nil {
		return err
	}
	m.runs[id] = run
	return nil
}

func (m *MemoryStorage) GetRun(ctx context.Context, id uuid.UUID) (*pproto.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.runs[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return run, nil
}

func (m *MemoryStorage) SaveStep(ctx context.Context, step *pproto.StepRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	runID, err := uuid.Parse(step.RunId)
	if err != nil {
		return err
	}
	m.steps[runID] = append(m.steps[runID], step)
	return nil
}

func (m *MemoryStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*pproto.StepRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.steps[runID], nil
}

func (m *MemoryStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	return nil
}

func (m *MemoryStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*pproto.Run, error) {
	return nil, nil
}

func (m *MemoryStorage) ListRuns(ctx context.Context) ([]*pproto.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*pproto.Run
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
	m.mu.RLock()
	defer m.mu.RUnlock()
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

func (m *MemoryStorage) DeleteRun(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runs, id)
	delete(m.steps, id)
	return nil
}
