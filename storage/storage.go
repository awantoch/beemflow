package storage

import (
	"context"
	"database/sql"
	"time"

	"encoding/json"

	"github.com/awantoch/beemflow/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/mattn/go-sqlite3"
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
	`, step.ID, step.RunID, step.StepName, step.Status, step.StartedAt, endedAt, outputs, step.Error)
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

// SqliteStorage implements Storage using SQLite as the backend.
type SqliteStorage struct {
	db *sql.DB
}

func NewSqliteStorage(dsn string) (*SqliteStorage, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	// Create tables if not exist
	sqlStmt := `
CREATE TABLE IF NOT EXISTS runs (
	id TEXT PRIMARY KEY,
	flow_name TEXT,
	event JSON,
	vars JSON,
	status TEXT,
	started_at INTEGER,
	ended_at INTEGER
);
CREATE TABLE IF NOT EXISTS steps (
	id TEXT PRIMARY KEY,
	run_id TEXT,
	step_name TEXT,
	status TEXT,
	started_at INTEGER,
	ended_at INTEGER,
	outputs JSON,
	error TEXT
);
CREATE TABLE IF NOT EXISTS waits (
	token TEXT PRIMARY KEY,
	wake_at INTEGER
);
CREATE TABLE IF NOT EXISTS paused_runs (
	token TEXT PRIMARY KEY,
	flow JSON,
	step_idx INTEGER,
	step_ctx JSON,
	outputs JSON
);
`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}
	return &SqliteStorage{db: db}, nil
}

func (s *SqliteStorage) SaveRun(ctx context.Context, run *model.Run) error {
	event, _ := json.Marshal(run.Event)
	vars, _ := json.Marshal(run.Vars)
	var endedAt interface{}
	if run.EndedAt != nil {
		endedAt = run.EndedAt.Unix()
	} else {
		endedAt = nil
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO runs (id, flow_name, event, vars, status, started_at, ended_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET flow_name=excluded.flow_name, event=excluded.event, vars=excluded.vars, status=excluded.status, started_at=excluded.started_at, ended_at=excluded.ended_at
`, run.ID.String(), run.FlowName, event, vars, run.Status, run.StartedAt.Unix(), endedAt)
	return err
}

func (s *SqliteStorage) GetRun(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, flow_name, event, vars, status, started_at, ended_at FROM runs WHERE id=?`, id.String())
	var run model.Run
	var event, vars []byte
	var startedAt, endedAtInt int64
	var endedAtPtr *time.Time
	var endedAt sql.NullInt64
	if err := row.Scan(&run.ID, &run.FlowName, &event, &vars, &run.Status, &startedAt, &endedAt); err != nil {
		return nil, err
	}
	json.Unmarshal(event, &run.Event)
	json.Unmarshal(vars, &run.Vars)
	run.StartedAt = time.Unix(startedAt, 0)
	if endedAt.Valid {
		endedAtInt = endedAt.Int64
		t := time.Unix(endedAtInt, 0)
		endedAtPtr = &t
	}
	run.EndedAt = endedAtPtr
	return &run, nil
}

func (s *SqliteStorage) SaveStep(ctx context.Context, step *model.StepRun) error {
	outputs, _ := json.Marshal(step.Outputs)
	var endedAt interface{}
	if step.EndedAt != nil {
		endedAt = step.EndedAt.Unix()
	} else {
		endedAt = nil
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO steps (id, run_id, step_name, status, started_at, ended_at, outputs, error)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id, step_name=excluded.step_name, status=excluded.status, started_at=excluded.started_at, ended_at=excluded.ended_at, outputs=excluded.outputs, error=excluded.error
`, step.ID.String(), step.RunID.String(), step.StepName, step.Status, step.StartedAt.Unix(), endedAt, outputs, step.Error)
	return err
}

func (s *SqliteStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*model.StepRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, step_name, status, started_at, ended_at, outputs, error FROM steps WHERE run_id=?`, runID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []*model.StepRun
	for rows.Next() {
		var srun model.StepRun
		var outputs []byte
		var startedAt, endedAtInt int64
		var endedAt sql.NullInt64
		var endedAtPtr *time.Time
		if err := rows.Scan(&srun.ID, &srun.StepName, &srun.Status, &startedAt, &endedAt, &outputs, &srun.Error); err != nil {
			continue
		}
		json.Unmarshal(outputs, &srun.Outputs)
		srun.StartedAt = time.Unix(startedAt, 0)
		if endedAt.Valid {
			endedAtInt = endedAt.Int64
			t := time.Unix(endedAtInt, 0)
			endedAtPtr = &t
		}
		srun.EndedAt = endedAtPtr
		steps = append(steps, &srun)
	}
	return steps, nil
}

func (s *SqliteStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO waits (token, wake_at) VALUES (?, ?) ON CONFLICT(token) DO UPDATE SET wake_at=excluded.wake_at`, token.String(), wakeAt)
	return err
}

func (s *SqliteStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*model.Run, error) {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM waits WHERE token=?`, token.String())
	return nil, nil
}

// Define a local struct for paused run persistence to avoid import cycle
type PausedRunPersist struct {
	Flow    *model.Flow    `json:"flow"`
	StepIdx int            `json:"step_idx"`
	StepCtx map[string]any `json:"step_ctx"`
	Outputs map[string]any `json:"outputs"`
	Token   string         `json:"token"`
	RunID   string         `json:"run_id"`
}

// Update SavePausedRun to use PausedRunPersist
func (s *SqliteStorage) SavePausedRun(token string, paused any) error {
	// paused is expected to be *engine.PausedRun, but we avoid import cycle by using reflection or map conversion
	// For now, require the caller to pass a map[string]any with the correct fields
	b, err := json.Marshal(paused)
	if err != nil {
		return err
	}
	var persist PausedRunPersist
	if err := json.Unmarshal(b, &persist); err != nil {
		return err
	}
	flowBytes, err := json.Marshal(persist.Flow)
	if err != nil {
		return err
	}
	stepCtxBytes, err := json.Marshal(persist.StepCtx)
	if err != nil {
		return err
	}
	outputsBytes, err := json.Marshal(persist.Outputs)
	if err != nil {
		return err
	}
	if v, ok := persist.StepCtx["run_id"]; ok {
		if s, ok := v.(string); ok {
			persist.RunID = s
		}
	}
	_, err = s.db.Exec(`
	INSERT INTO paused_runs (token, flow, step_idx, step_ctx, outputs)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(token) DO UPDATE SET flow=excluded.flow, step_idx=excluded.step_idx, step_ctx=excluded.step_ctx, outputs=excluded.outputs
	`, token, flowBytes, persist.StepIdx, stepCtxBytes, outputsBytes)
	return err
}

// Update LoadPausedRuns to return map[string]PausedRunPersist
func (s *SqliteStorage) LoadPausedRuns() (map[string]PausedRunPersist, error) {
	rows, err := s.db.Query(`SELECT token, flow, step_idx, step_ctx, outputs FROM paused_runs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]PausedRunPersist)
	for rows.Next() {
		var token string
		var flowBytes, stepCtxBytes, outputsBytes []byte
		var stepIdx int
		if err := rows.Scan(&token, &flowBytes, &stepIdx, &stepCtxBytes, &outputsBytes); err != nil {
			continue
		}
		var flow model.Flow
		var stepCtx map[string]any
		var outputs map[string]any
		if err := json.Unmarshal(flowBytes, &flow); err != nil {
			continue
		}
		if err := json.Unmarshal(stepCtxBytes, &stepCtx); err != nil {
			continue
		}
		if err := json.Unmarshal(outputsBytes, &outputs); err != nil {
			continue
		}
		result[token] = PausedRunPersist{
			Flow:    &flow,
			StepIdx: stepIdx,
			StepCtx: stepCtx,
			Outputs: outputs,
			Token:   token,
			RunID:   runIDFromStepCtx(stepCtx),
		}
	}
	return result, nil
}

// DeletePausedRun removes a paused run from the database
func (s *SqliteStorage) DeletePausedRun(token string) error {
	_, err := s.db.Exec(`DELETE FROM paused_runs WHERE token=?`, token)
	return err
}

// GetLatestRunByFlowName returns the most recent run for a given flow name
func (s *SqliteStorage) GetLatestRunByFlowName(ctx context.Context, flowName string) (*model.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, flow_name, event, vars, status, started_at, ended_at FROM runs WHERE flow_name = ? ORDER BY started_at DESC LIMIT 1`, flowName)
	var run model.Run
	var event, vars []byte
	var startedAt, endedAtInt int64
	var endedAtPtr *time.Time
	var endedAt sql.NullInt64
	if err := row.Scan(&run.ID, &run.FlowName, &event, &vars, &run.Status, &startedAt, &endedAt); err != nil {
		return nil, err
	}
	json.Unmarshal(event, &run.Event)
	json.Unmarshal(vars, &run.Vars)
	run.StartedAt = time.Unix(startedAt, 0)
	if endedAt.Valid {
		endedAtInt = endedAt.Int64
		t := time.Unix(endedAtInt, 0)
		endedAtPtr = &t
	}
	run.EndedAt = endedAtPtr
	return &run, nil
}

func runIDFromStepCtx(ctx map[string]any) string {
	if v, ok := ctx["run_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
