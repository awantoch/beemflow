package storage

import (
	"context"
	"database/sql"

	"github.com/awantoch/beemflow/model"
	"github.com/google/uuid"
)

type PostgresStorage struct {
	db *sql.DB
}

var _ Storage = (*PostgresStorage)(nil)

func (s *PostgresStorage) SavePausedRun(token string, paused any) error {
	// TODO: implement paused run persistence for Postgres
	return nil
}

func (s *PostgresStorage) LoadPausedRuns() (map[string]any, error) {
	// TODO: implement paused run persistence for Postgres
	return map[string]any{}, nil
}

func (s *PostgresStorage) DeletePausedRun(token string) error {
	// TODO: implement paused run persistence for Postgres
	return nil
}

func (s *PostgresStorage) SaveRun(ctx context.Context, run *model.Run) error { return nil }
func (s *PostgresStorage) GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	return nil, sql.ErrNoRows
}
func (s *PostgresStorage) SaveStep(ctx context.Context, step *model.StepRun) error { return nil }
func (s *PostgresStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*model.StepRun, error) {
	return nil, nil
}
func (s *PostgresStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	return nil
}
func (s *PostgresStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*model.Run, error) {
	return nil, nil
}
func (s *PostgresStorage) ListRuns(ctx context.Context) ([]*model.Run, error) { return nil, nil }
func (s *PostgresStorage) DeleteRun(ctx context.Context, id uuid.UUID) error {
	// TODO: implement real deletion logic
	return nil
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	// TODO: implement real Postgres connection
	return &PostgresStorage{db: nil}, nil
}
