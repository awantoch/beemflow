package engine

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

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
	f := &model.Flow{Name: "test", Steps: []model.Step{}}
	_, err := e.Execute(context.Background(), f, nil)
	if err != nil {
		t.Errorf("expected nil error for nil event, got %v", err)
	}
}

func TestExecute_MinimalValidFlow(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "test", Steps: []model.Step{{ID: "s1", Use: "core.echo"}}}
	_, err := e.Execute(context.Background(), f, map[string]any{"foo": "bar"})
	if err != nil {
		t.Errorf("expected nil error for minimal valid flow, got %v", err)
	}
}

func TestExecute_AllStepTypes(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "all_types", Steps: []model.Step{
		{
			ID:         "s1",
			Use:        "core.echo",
			With:       map[string]interface{}{"text": "hi"},
			If:         "x > 0",
			Foreach:    "{{list}}",
			As:         "item",
			Do:         []model.Step{{ID: "d1", Use: "core.echo", With: map[string]interface{}{"text": "{{item}}"}}},
			Parallel:   true,
			Retry:      &model.RetrySpec{Attempts: 2, DelaySec: 1},
			AwaitEvent: &model.AwaitEventSpec{Source: "bus", Match: map[string]interface{}{"key": "value"}, Timeout: "10s"},
			Wait:       &model.WaitSpec{Seconds: 5, Until: "2025-01-01"},
		},
		{ID: "s2", Use: "core.echo", With: map[string]interface{}{"text": "hi"}},
	}}
	_, err := e.Execute(context.Background(), f, map[string]any{"foo": "bar"})
	if err == nil || !strings.Contains(err.Error(), "missing token in match") {
		t.Errorf("expected await_event missing token error, got %v", err)
	}
}

func TestExecute_Concurrency(t *testing.T) {
	e := NewEngine()
	f := &model.Flow{Name: "concurrent", Steps: []model.Step{{ID: "s1", Use: "core.echo"}}}
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

func TestAwaitEventResume_RoundTrip(t *testing.T) {
	// Load the test flow
	f, err := os.ReadFile("../../flows/echo_await_resume.flow.yaml")
	if err != nil {
		t.Fatalf("failed to read flow: %v", err)
	}
	var flow model.Flow
	if err := yaml.Unmarshal(f, &flow); err != nil {
		t.Fatalf("failed to unmarshal flow: %v", err)
	}
	engine := NewEngine()
	// Start the flow with input and token
	startEvent := map[string]any{"input": "hello world", "token": "abc123"}
	outputs, err := engine.Execute(context.Background(), &flow, startEvent)
	if err == nil || !strings.Contains(err.Error(), "await_event pause") {
		t.Fatalf("expected pause on await_event, got: %v, outputs: %v", err, outputs)
	}
	// Wait to ensure subscription is registered
	time.Sleep(50 * time.Millisecond)
	// Simulate a real-world delay before resume
	time.Sleep(7 * time.Second)
	// Simulate resume event
	resumeEvent := map[string]any{"resume_value": "it worked!", "token": "abc123"}
	engine.EventBus.Publish("resume:abc123", resumeEvent)
	// Wait briefly to allow resume goroutine to complete
	time.Sleep(100 * time.Millisecond)
	// After resume, the outputs should include both echo steps
	resumedOutputs := engine.GetCompletedOutputs("abc123")
	t.Logf("resumed outputs: %+v", resumedOutputs)
	if resumedOutputs == nil {
		t.Fatalf("expected outputs after resume, got nil")
	}
	if resumedOutputs["echo_start"] == nil {
		t.Errorf("expected echo_start output, got: %v", resumedOutputs)
	}
	if resumedOutputs["echo_resumed"] == nil {
		t.Errorf("expected echo_resumed output, got: %v", resumedOutputs)
	}
}
