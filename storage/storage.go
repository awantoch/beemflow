package storage

import (
	"context"

	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Storage interface {
	SaveRun(ctx context.Context, run *pproto.Run) error
	GetRun(ctx context.Context, id uuid.UUID) (*pproto.Run, error)
	SaveStep(ctx context.Context, step *pproto.StepRun) error
	GetSteps(ctx context.Context, runID uuid.UUID) ([]*pproto.StepRun, error)
	RegisterWait(ctx context.Context, token uuid.UUID, wakeAt *int64) error
	ResolveWait(ctx context.Context, token uuid.UUID) (*pproto.Run, error)
	ListRuns(ctx context.Context) ([]*pproto.Run, error)
	SavePausedRun(token string, paused any) error
	LoadPausedRuns() (map[string]any, error)
	DeletePausedRun(token string) error
	DeleteRun(ctx context.Context, id uuid.UUID) error
}
