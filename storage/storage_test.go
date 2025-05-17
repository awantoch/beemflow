package storage

import (
	"context"
	"os"
	"testing"

	"github.com/awantoch/beemflow/model"
	"github.com/google/uuid"
)

func TestNewPostgresStorage(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set")
	}
	s, err := NewPostgresStorage(dsn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatalf("expected non-nil PostgresStorage")
	}
}

func TestStorage_RoundTrip(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set")
	}
	s, _ := NewPostgresStorage(dsn)
	ctx := context.Background()
	run := &model.Run{ID: uuid.New(), FlowName: "test-flow"}
	if err := s.SaveRun(ctx, run); err != nil {
		t.Errorf("SaveRun failed: %v", err)
	}
	gotRun, err := s.GetRun(ctx, run.ID)
	if err != nil {
		t.Errorf("GetRun failed: %v", err)
	}
	if gotRun.ID != run.ID {
		t.Errorf("expected run ID %v, got %v", run.ID, gotRun.ID)
	}
	step := &model.StepRun{ID: uuid.New(), StepName: "step1"}
	if err := s.SaveStep(ctx, step); err != nil {
		t.Errorf("SaveStep failed: %v", err)
	}
	steps, err := s.GetSteps(ctx, run.ID)
	if err != nil {
		t.Errorf("GetSteps failed: %v", err)
	}
	if len(steps) == 0 {
		t.Errorf("expected at least one step")
	}
	token := uuid.New()
	if err := s.RegisterWait(ctx, token, nil); err != nil {
		t.Errorf("RegisterWait failed: %v", err)
	}
	_, err = s.ResolveWait(ctx, token)
	if err != nil {
		t.Errorf("ResolveWait failed: %v", err)
	}
}
