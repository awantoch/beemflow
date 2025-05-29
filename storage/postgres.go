package storage

import (
	"context"
	"fmt"

	"github.com/awantoch/beemflow/model"
	"github.com/google/uuid"
)

type PostgresStorage struct {
	// Stub implementation - no fields needed until implementation
}

var _ Storage = (*PostgresStorage)(nil)

const postgresNotImplementedMsg = "PostgresStorage is not yet implemented - use SqliteStorage or MemoryStorage instead"

func (s *PostgresStorage) SavePausedRun(token string, paused any) error {
	return fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) LoadPausedRuns() (map[string]any, error) {
	return nil, fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) DeletePausedRun(token string) error {
	return fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) SaveRun(ctx context.Context, run *model.Run) error {
	return fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	return nil, fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) SaveStep(ctx context.Context, step *model.StepRun) error {
	return fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*model.StepRun, error) {
	return nil, fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	return fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*model.Run, error) {
	return nil, fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return nil, fmt.Errorf(postgresNotImplementedMsg)
}

func (s *PostgresStorage) DeleteRun(ctx context.Context, id uuid.UUID) error {
	return fmt.Errorf(postgresNotImplementedMsg)
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	return nil, fmt.Errorf(postgresNotImplementedMsg + " - constructor disabled")
}
