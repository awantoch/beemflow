package storage

import (
	"context"
	"time"

	"encoding/json"

	"github.com/awantoch/beemflow/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage interface {
	SaveRun(ctx context.Context, run *model.Run) error
	GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error)
	SaveStep(ctx context.Context, step *model.StepRun) error
	GetSteps(ctx context.Context, runID uuid.UUID) ([]*model.StepRun, error)
	RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error
	ResolveWait(ctx context.Context, token uuid.UUID) (*model.Run, error)
}

type PostgresStorage struct {
	db *pgxpool.Pool
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	// Create tables if not exist
	sql := `
CREATE TABLE IF NOT EXISTS runs (
	id UUID PRIMARY KEY,
	flow_name TEXT,
	event JSONB,
	vars JSONB,
	status TEXT,
	started_at TIMESTAMP,
	ended_at TIMESTAMP
);
CREATE TABLE IF NOT EXISTS steps (
	id UUID PRIMARY KEY,
	run_id UUID,
	step_name TEXT,
	status TEXT,
	started_at TIMESTAMP,
	ended_at TIMESTAMP,
	outputs JSONB,
	error TEXT
);
CREATE TABLE IF NOT EXISTS waits (
	token UUID PRIMARY KEY,
	wake_at BIGINT
);
`
	_, err = db.Exec(context.Background(), sql)
	if err != nil {
		return nil, err
	}
	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) SaveRun(ctx context.Context, run *model.Run) error {
	event, _ := json.Marshal(run.Event)
	vars, _ := json.Marshal(run.Vars)
	endedAt := run.EndedAt
	_, err := s.db.Exec(ctx, `
	INSERT INTO runs (id, flow_name, event, vars, status, started_at, ended_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (id) DO UPDATE SET flow_name=$2, event=$3, vars=$4, status=$5, started_at=$6, ended_at=$7
	`, run.ID, run.FlowName, event, vars, run.Status, run.StartedAt, endedAt)
	return err
}

func (s *PostgresStorage) GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	row := s.db.QueryRow(ctx, `SELECT id, flow_name, event, vars, status, started_at, ended_at FROM runs WHERE id=$1`, id)
	var run model.Run
	var event, vars []byte
	var endedAt *time.Time
	if err := row.Scan(&run.ID, &run.FlowName, &event, &vars, &run.Status, &run.StartedAt, &endedAt); err != nil {
		return nil, err
	}
	json.Unmarshal(event, &run.Event)
	json.Unmarshal(vars, &run.Vars)
	run.EndedAt = endedAt
	return &run, nil
}

func (s *PostgresStorage) SaveStep(ctx context.Context, step *model.StepRun) error {
	outputs, _ := json.Marshal(step.Outputs)
	endedAt := step.EndedAt
	_, err := s.db.Exec(ctx, `
	INSERT INTO steps (id, run_id, step_name, status, started_at, ended_at, outputs, error)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (id) DO UPDATE SET run_id=$2, step_name=$3, status=$4, started_at=$5, ended_at=$6, outputs=$7, error=$8
	`, step.ID, step.ID, step.StepName, step.Status, step.StartedAt, endedAt, outputs, step.Error)
	return err
}

func (s *PostgresStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*model.StepRun, error) {
	rows, err := s.db.Query(ctx, `SELECT id, step_name, status, started_at, ended_at, outputs, error FROM steps WHERE run_id=$1`, runID)
	if err != nil {
		return nil, err
	}
	var steps []*model.StepRun
	for rows.Next() {
		var s model.StepRun
		var outputs []byte
		var endedAt *time.Time
		if err := rows.Scan(&s.ID, &s.StepName, &s.Status, &s.StartedAt, &endedAt, &outputs, &s.Error); err != nil {
			continue
		}
		json.Unmarshal(outputs, &s.Outputs)
		s.EndedAt = endedAt
		steps = append(steps, &s)
	}
	return steps, nil
}

func (s *PostgresStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	_, err := s.db.Exec(ctx, `INSERT INTO waits (token, wake_at) VALUES ($1, $2) ON CONFLICT (token) DO UPDATE SET wake_at=$2`, token, wakeAt)
	return err
}

func (s *PostgresStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*model.Run, error) {
	// For now, just delete the wait and return nil
	_, _ = s.db.Exec(ctx, `DELETE FROM waits WHERE token=$1`, token)
	return nil, nil
}
