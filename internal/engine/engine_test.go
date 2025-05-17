package engine

import (
	"context"
	"testing"

	"github.com/awantoch/beemflow/internal/model"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Error("expected NewEngine not nil")
	}
}

func TestExecuteNoop(t *testing.T) {
	e := NewEngine()
	_, err := e.Execute(context.Background(), &model.Flow{}, map[string]any{})
	if err != nil {
		t.Errorf("Execute returned error: %v", err)
	}
}

func TestNewCronScheduler(t *testing.T) {
	s := NewCronScheduler()
	if s == nil {
		t.Error("expected NewCronScheduler not nil")
	}
}

func TestExecute_NilFlow(t *testing.T) {
	e := NewEngine()
	_, err := e.Execute(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Errorf("expected nil error for nil flow, got %v", err)
	}
}

func TestExecute_NilEvent(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "test", Steps: map[string]model.Step{}}
	_, err := e.Execute(context.Background(), f, nil)
	if err != nil {
		t.Errorf("expected nil error for nil event, got %v", err)
	}
}

func TestExecute_MinimalValidFlow(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "test", Steps: map[string]model.Step{"s1": {Use: "core.echo"}}}
	_, err := e.Execute(context.Background(), f, map[string]any{"foo": "bar"})
	if err != nil {
		t.Errorf("expected nil error for minimal valid flow, got %v", err)
	}
}

func TestExecute_AllStepTypes(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "all_types", Steps: map[string]model.Step{
		"s1": {
			Use:        "core.echo",
			With:       map[string]interface{}{"text": "hi"},
			If:         "x > 0",
			Foreach:    "{{list}}",
			As:         "item",
			Do:         []model.Step{{Use: "core.echo", With: map[string]interface{}{"text": "{{item}}"}}},
			Parallel:   []string{"s2"},
			Retry:      &model.RetrySpec{Attempts: 2, DelaySec: 1},
			AwaitEvent: &model.AwaitEventSpec{Source: "bus", Match: map[string]interface{}{"key": "value"}, Timeout: "10s"},
			Wait:       &model.WaitSpec{Seconds: 5, Until: "2025-01-01"},
		},
		"s2": {Use: "core.echo", With: map[string]interface{}{"text": "hi"}},
	}}
	_, err := e.Execute(context.Background(), f, map[string]any{"foo": "bar"})
	if err != nil {
		t.Errorf("expected nil error for all step types, got %v", err)
	}
}

func TestExecute_Concurrency(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "concurrent", Steps: map[string]model.Step{"s1": {Use: "core.echo"}}}
	done := make(chan bool, 2)
	go func() {
		_, _ = e.Execute(context.Background(), f, map[string]any{"foo": "bar"})
		done <- true
	}()
	go func() {
		_, _ = e.Execute(context.Background(), f, map[string]any{"foo": "baz"})
		done <- true
	}()
	<-done
	<-done
}
