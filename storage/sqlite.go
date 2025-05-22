package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SqliteStorage implements Storage using SQLite as the backend.
type SqliteStorage struct {
	db *sql.DB
}

var _ Storage = (*SqliteStorage)(nil)

type PausedRunPersist struct {
	Flow    *pproto.Flow   `json:"flow"`
	StepIdx int            `json:"step_idx"`
	StepCtx map[string]any `json:"step_ctx"`
	Outputs map[string]any `json:"outputs"`
	Token   string         `json:"token"`
	RunID   string         `json:"run_id"`
}

func runIDFromStepCtx(ctx map[string]any) string {
	if v, ok := ctx["run_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func NewSqliteStorage(dsn string) (*SqliteStorage, error) {
	// Only create parent directories if not using in-memory SQLite (":memory:").
	if dsn != ":memory:" && dsn != "" {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, utils.Errorf("failed to create db directory %q: %w", dir, err)
		}
	}
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

func (s *SqliteStorage) SaveRun(ctx context.Context, run *pproto.Run) error {
	event, _ := json.Marshal(run.Event)
	vars, _ := json.Marshal(run.Vars)
	startUnix := run.StartedAt.AsTime().Unix()
	var endUnix interface{}
	if run.EndedAt != nil {
		endUnix = run.EndedAt.AsTime().Unix()
	} else {
		endUnix = nil
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO runs (id, flow_name, event, vars, status, started_at, ended_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET flow_name=excluded.flow_name, event=excluded.event, vars=excluded.vars, status=excluded.status, started_at=excluded.started_at, ended_at=excluded.ended_at
`, run.Id, run.FlowName, event, vars, run.Status, startUnix, endUnix)
	return err
}

func (s *SqliteStorage) GetRun(ctx context.Context, id uuid.UUID) (*pproto.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, flow_name, event, vars, status, started_at, ended_at FROM runs WHERE id=?`, id.String())
	var run pproto.Run
	var eventBytes, varsBytes []byte
	var startedAtInt int64
	var endedAt sql.NullInt64
	if err := row.Scan(&run.Id, &run.FlowName, &eventBytes, &varsBytes, &run.Status, &startedAtInt, &endedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(eventBytes, &run.Event); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(varsBytes, &run.Vars); err != nil {
		return nil, err
	}
	run.StartedAt = timestamppb.New(time.Unix(startedAtInt, 0))
	if endedAt.Valid {
		run.EndedAt = timestamppb.New(time.Unix(endedAt.Int64, 0))
	}
	return &run, nil
}

func (s *SqliteStorage) SaveStep(ctx context.Context, step *pproto.StepRun) error {
	outputs, _ := json.Marshal(step.Outputs)
	startUnix := step.StartedAt.AsTime().Unix()
	var endUnix interface{}
	if step.EndedAt != nil {
		endUnix = step.EndedAt.AsTime().Unix()
	} else {
		endUnix = nil
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO steps (id, run_id, step_name, status, started_at, ended_at, outputs, error)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id, step_name=excluded.step_name, status=excluded.status, started_at=excluded.started_at, ended_at=excluded.ended_at, outputs=excluded.outputs, error=excluded.error
`, step.Id, step.RunId, step.StepName, step.Status, startUnix, endUnix, outputs, step.Error)
	return err
}

func (s *SqliteStorage) GetSteps(ctx context.Context, runID uuid.UUID) ([]*pproto.StepRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, step_name, status, started_at, ended_at, outputs, error FROM steps WHERE run_id=?`, runID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []*pproto.StepRun
	for rows.Next() {
		var srun pproto.StepRun
		var runIDStr string
		var outputsBytes []byte
		var startedAtInt int64
		var endedAt sql.NullInt64
		if err := rows.Scan(&srun.Id, &runIDStr, &srun.StepName, &srun.Status, &startedAtInt, &endedAt, &outputsBytes, &srun.Error); err != nil {
			continue
		}
		srun.RunId = runIDStr
		if err := json.Unmarshal(outputsBytes, &srun.Outputs); err != nil {
			return nil, err
		}
		srun.StartedAt = timestamppb.New(time.Unix(startedAtInt, 0))
		if endedAt.Valid {
			srun.EndedAt = timestamppb.New(time.Unix(endedAt.Int64, 0))
		}
		steps = append(steps, &srun)
	}
	return steps, nil
}

func (s *SqliteStorage) RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO waits (token, wake_at) VALUES (?, ?) ON CONFLICT(token) DO UPDATE SET wake_at=excluded.wake_at`, token.String(), wakeAt)
	return err
}

func (s *SqliteStorage) ResolveWait(ctx context.Context, token uuid.UUID) (*pproto.Run, error) {
	_, _ = s.db.ExecContext(ctx, `DELETE FROM waits WHERE token=?`, token.String())
	return nil, nil
}

// PausedRunPersist and helpers

func (s *SqliteStorage) SavePausedRun(token string, paused any) error {
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

func (s *SqliteStorage) LoadPausedRuns() (map[string]any, error) {
	rows, err := s.db.Query(`SELECT token, flow, step_idx, step_ctx, outputs FROM paused_runs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]any)
	for rows.Next() {
		var token string
		var flowBytes, stepCtxBytes, outputsBytes []byte
		var stepIdx int
		if err := rows.Scan(&token, &flowBytes, &stepIdx, &stepCtxBytes, &outputsBytes); err != nil {
			continue
		}
		var flow pproto.Flow
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

func (s *SqliteStorage) DeletePausedRun(token string) error {
	_, err := s.db.Exec(`DELETE FROM paused_runs WHERE token=?`, token)
	return err
}

func (s *SqliteStorage) GetLatestRunByFlowName(ctx context.Context, flowName string) (*pproto.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, flow_name, event, vars, status, started_at, ended_at FROM runs WHERE flow_name = ? ORDER BY started_at DESC LIMIT 1`, flowName)
	var run pproto.Run
	var eventBytes, varsBytes []byte
	var startedAtInt int64
	var endedAt sql.NullInt64
	if err := row.Scan(&run.Id, &run.FlowName, &eventBytes, &varsBytes, &run.Status, &startedAtInt, &endedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(eventBytes, &run.Event); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(varsBytes, &run.Vars); err != nil {
		return nil, err
	}
	run.StartedAt = timestamppb.New(time.Unix(startedAtInt, 0))
	if endedAt.Valid {
		run.EndedAt = timestamppb.New(time.Unix(endedAt.Int64, 0))
	}
	return &run, nil
}

func (s *SqliteStorage) ListRuns(ctx context.Context) ([]*pproto.Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, flow_name, event, vars, status, started_at, ended_at FROM runs ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []*pproto.Run
	for rows.Next() {
		var run pproto.Run
		var eventBytes, varsBytes []byte
		var startedAtInt int64
		var endedAt sql.NullInt64
		if err := rows.Scan(&run.Id, &run.FlowName, &eventBytes, &varsBytes, &run.Status, &startedAtInt, &endedAt); err != nil {
			continue
		}
		if err := json.Unmarshal(eventBytes, &run.Event); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(varsBytes, &run.Vars); err != nil {
			return nil, err
		}
		run.StartedAt = timestamppb.New(time.Unix(startedAtInt, 0))
		if endedAt.Valid {
			run.EndedAt = timestamppb.New(time.Unix(endedAt.Int64, 0))
		}
		runs = append(runs, &run)
	}
	return runs, nil
}

func (s *SqliteStorage) DeleteRun(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM steps WHERE run_id=?`, id.String())
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM runs WHERE id=?`, id.String())
	return err
}

// Close closes the underlying SQL database connection.
func (s *SqliteStorage) Close() error {
	return s.db.Close()
}
