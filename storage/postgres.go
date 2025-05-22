package storage

import (
	"context"
	"database/sql"
	"fmt"

	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/google/uuid"
)

type PostgresStorage struct {
	db *sql.DB
}

var _ Storage = (*PostgresStorage)(nil)

func (s *PostgresStorage) SavePausedRun(token string, paused any) error {
	return fmt.Errorf("SavePausedRun not implemented for PostgresStorage")
}

func (s *PostgresStorage) LoadPausedRuns() (map[string]any, error) {
	return nil, fmt.Errorf("LoadPausedRuns not implemented for PostgresStorage")
}

func (s *PostgresStorage) DeletePausedRun(token string) error {
	return fmt.Errorf("DeletePausedRun not implemented for PostgresStorage")
}

func (s *PostgresStorage) SaveRun(ctx context.Context, run *pproto.Run) error { return nil }
func (s *PostgresStorage) GetRun(ctx context.Context, id uuid.UUID) (*pproto.Run, error) {
	return nil, sql.ErrNoRows
}
func (s *PostgresStorage) SaveStep(ctx context.Context, step *pproto.StepRun) error { return nil }
func (s *PostgresStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*pproto.StepRun, error) {
	return nil, nil
}
func (s *PostgresStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	return nil
}
func (s *PostgresStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*pproto.Run, error) {
	return nil, nil
}
func (s *PostgresStorage) ListRuns(ctx context.Context) ([]*pproto.Run, error) { return nil, nil }
func (s *PostgresStorage) DeleteRun(ctx context.Context, id uuid.UUID) error {
	return fmt.Errorf("DeleteRun not implemented for PostgresStorage")
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	// TODO: implement real Postgres connection
	return &PostgresStorage{db: nil}, nil
}
