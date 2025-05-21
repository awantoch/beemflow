package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils/testutil"
)

func TestMain(m *testing.M) {
	testutil.WithCleanDir(m, config.DefaultConfigDir)
}

func TestNewEngine(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	if e == nil {
		t.Error("expected NewEngine not nil")
	}
}

func TestExecuteNoop(t *testing.T) {
	e := NewDefaultEngine(context.Background())
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
	e := NewDefaultEngine(context.Background())
	_, err := e.Execute(context.Background(), nil, map[string]any{})
	if err != nil {
		t.Errorf("expected nil error for nil flow, got %v", err)
	}
}

func TestExecute_NilEvent(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	f := &model.Flow{Name: "test", Steps: []model.Step{}}
	_, err := e.Execute(context.Background(), f, nil)
	if err != nil {
		t.Errorf("expected nil error for nil event, got %v", err)
	}
}

func TestExecute_MinimalValidFlow(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	f := &model.Flow{Name: "test", Steps: []model.Step{{ID: "s1", Use: "core.echo"}}}
	_, err := e.Execute(context.Background(), f, map[string]any{"foo": "bar"})
	if err != nil {
		t.Errorf("expected nil error for minimal valid flow, got %v", err)
	}
}

func TestExecute_AllStepTypes(t *testing.T) {
	e := NewDefaultEngine(context.Background())
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
	e := NewDefaultEngine(context.Background())
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
	f, err := os.ReadFile("../" + config.DefaultFlowsDir + "/echo_await_resume.flow.yaml")
	if err != nil {
		t.Fatalf("failed to read flow: %v", err)
	}
	var flow model.Flow
	if err := yaml.Unmarshal(f, &flow); err != nil {
		t.Fatalf("failed to unmarshal flow: %v", err)
	}
	engine := NewDefaultEngine(context.Background())
	// Start the flow with input and token
	startEvent := map[string]any{"input": "hello world", "token": "abc123"}
	outputs, err := engine.Execute(context.Background(), &flow, startEvent)
	if err == nil || !strings.Contains(err.Error(), "await_event pause") {
		t.Fatalf("expected pause on await_event, got: %v, outputs: %v", err, outputs)
	}
	// Wait to ensure subscription is registered
	time.Sleep(50 * time.Millisecond)
	// Simulate a real-world delay before resume (short for test)
	time.Sleep(50 * time.Millisecond)
	// Simulate resume event
	resumeEvent := map[string]any{"resume_value": "it worked!", "token": "abc123"}
	if err := engine.EventBus.Publish("resume:abc123", resumeEvent); err != nil {
		t.Errorf("Publish failed: %v", err)
	}
	// Wait briefly to allow resume goroutine to complete
	var resumedOutputs map[string]any
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		resumedOutputs = engine.GetCompletedOutputs("abc123")
		if resumedOutputs != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
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

func TestExecute_CatchBlock(t *testing.T) {
	flow := &model.Flow{
		Name:  "catch_test",
		Steps: []model.Step{{ID: "fail", Use: "nonexistent.adapter"}},
		Catch: []model.Step{
			{ID: "catch1", Use: "core.echo", With: map[string]interface{}{"text": "caught!"}},
			{ID: "catch2", Use: "core.echo", With: map[string]interface{}{"text": "second!"}},
		},
	}
	eng := NewDefaultEngine(context.Background())
	outputs, err := eng.Execute(context.Background(), flow, nil)
	if err == nil {
		t.Errorf("expected error from fail step")
	}
	if out, ok := outputs["catch1"].(map[string]any); !ok || out["text"] != "caught!" {
		t.Errorf("expected catch1 to run and output map with text, got outputs: %v", outputs)
	}
	if out, ok := outputs["catch2"].(map[string]any); !ok || out["text"] != "second!" {
		t.Errorf("expected catch2 to run and output map with text, got outputs: %v", outputs)
	}
}

func TestExecute_AdapterErrorPropagation(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	f := &model.Flow{
		Name:  "adapter_error",
		Steps: []model.Step{{ID: "s1", Use: "core.echo"}},
	}
	outputs, err := e.Execute(context.Background(), f, map[string]any{})
	if err != nil {
		t.Errorf("unexpected error from adapter, got %v", err)
	}
	// Expect outputs to be a map with an empty map for s1
	if out, ok := outputs["s1"].(map[string]any); !ok || len(out) != 0 {
		t.Errorf("expected outputs to be map with empty map for s1, got: %v", outputs)
	}
}

func TestExecute_ParallelForeachEdgeCases(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	// Parallel with empty list
	f := &model.Flow{
		Name: "parallel_empty",
		Steps: []model.Step{{
			ID:       "s1",
			Use:      "core.echo",
			Foreach:  "{{list}}",
			As:       "item",
			Parallel: true,
			Do:       []model.Step{{ID: "d1", Use: "core.echo", With: map[string]interface{}{"text": "{{item}}"}}},
		}},
	}
	outputs, err := e.Execute(context.Background(), f, map[string]any{"list": []any{}})
	if err != nil {
		t.Errorf("expected no error for empty foreach, got %v", err)
	}
	// Expect outputs to be a map with an empty map for s1
	if out, ok := outputs["s1"].(map[string]any); !ok || len(out) != 0 {
		t.Errorf("expected outputs to be map with empty map for s1, got %v", outputs)
	}
	// Parallel with error in one branch
	f2 := &model.Flow{
		Name: "parallel_error",
		Steps: []model.Step{{
			ID:       "s1",
			Use:      "core.echo",
			Foreach:  "{{list}}",
			As:       "item",
			Parallel: true,
			Do:       []model.Step{{ID: "d1", Use: "nonexistent.adapter"}},
		}},
	}
	_, err = e.Execute(context.Background(), f2, map[string]any{"list": []any{"a", "b"}})
	if err == nil {
		t.Errorf("expected error for parallel branch failure, got nil")
	}
}

func TestExecute_SecretsInjection(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	f := &model.Flow{
		Name:  "secrets_injection",
		Steps: []model.Step{{ID: "s1", Use: "core.echo", With: map[string]interface{}{"text": "{{ secrets.MY_SECRET }}"}}},
	}
	outputs, err := e.Execute(context.Background(), f, map[string]any{"secrets": map[string]any{"MY_SECRET": "shhh"}})
	if err != nil {
		t.Errorf("expected no error for secrets injection, got %v", err)
	}
	// Expect outputs["s1"] to be a map with key "text" and value "shhh"
	if out, ok := outputs["s1"].(map[string]any); !ok || out["text"] != "shhh" {
		t.Errorf("expected secret injected as map output, got %v", outputs["s1"])
	}
}

func TestExecute_SecretsDotAccess(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	f := &model.Flow{
		Name:  "secrets_dot_access",
		Steps: []model.Step{{ID: "s1", Use: "core.echo", With: map[string]interface{}{"text": "{{ secrets.MY_SECRET }}"}}},
	}
	outputs, err := e.Execute(context.Background(), f, map[string]any{"secrets": map[string]any{"MY_SECRET": "shhh"}})
	if err != nil {
		t.Errorf("expected no error for secrets dot access, got %v", err)
	}
	if out, ok := outputs["s1"].(map[string]any); !ok || out["text"] != "shhh" {
		t.Errorf("expected secret injected as map output, got %v", outputs["s1"])
	}
}

func TestExecute_ArrayAccessInTemplate(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	f := &model.Flow{
		Name:  "array_access",
		Steps: []model.Step{{ID: "s1", Use: "core.echo", With: map[string]interface{}{"text": "First: {{ event.arr.0.val }}, Second: {{ event.arr.1.val }}"}}},
	}
	arr := []map[string]any{{"val": "a"}, {"val": "b"}}
	outputs, err := e.Execute(context.Background(), f, map[string]any{"arr": arr})
	if err != nil {
		t.Errorf("expected no error for array access, got %v", err)
	}
	if out, ok := outputs["s1"].(map[string]any); !ok || out["text"] != "First: a, Second: b" {
		t.Errorf("expected array access output, got %v", outputs["s1"])
	}
}

func TestSqlitePersistenceAndResume_FullFlow(t *testing.T) {
	// Use a temp SQLite file
	tmpDir := t.TempDir()
	// Cleanup temp dir (and any SQLite files) before automatic TempDir removal
	defer func() { os.RemoveAll(tmpDir) }()
	dbPath := filepath.Join(tmpDir, t.Name()+"-resume_fullflow.db")

	// Load the echo_await_resume flow
	f, err := os.ReadFile("../" + config.DefaultFlowsDir + "/echo_await_resume.flow.yaml")
	if err != nil {
		t.Fatalf("failed to read flow: %v", err)
	}
	var flow model.Flow
	if err := yaml.Unmarshal(f, &flow); err != nil {
		t.Fatalf("failed to unmarshal flow: %v", err)
	}

	// Create storage and engine
	s, err := storage.NewSqliteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	engine := NewEngineWithStorage(context.Background(), s)

	// Start the flow, should pause at await_event
	startEvent := map[string]any{"input": "hello world", "token": "abc123"}
	outputs, err := engine.Execute(context.Background(), &flow, startEvent)
	if err == nil || !strings.Contains(err.Error(), "await_event pause") {
		t.Fatalf("expected pause on await_event, got: %v, outputs: %v", err, outputs)
	}

	// Check that only echo_start step is present in DB
	run, err := s.GetLatestRunByFlowName(context.Background(), flow.Name)
	if err != nil {
		t.Fatalf("GetLatestRunByFlowName failed: %v", err)
	}
	steps, err := s.GetSteps(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetSteps failed: %v", err)
	}
	var foundStart bool
	for _, step := range steps {
		if step.StepName == "echo_start" {
			foundStart = true
		}
	}
	if !foundStart {
		t.Fatalf("expected echo_start step after pause")
	}

	// Simulate a restart (new storage/engine instance)
	s2, err := storage.NewSqliteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen sqlite storage: %v", err)
	}
	defer func() {
		_ = s2.Close()
	}()
	engine2 := NewEngineWithStorage(context.Background(), s2)

	// Simulate resume event
	resumeEvent := map[string]any{"resume_value": "it worked!", "token": "abc123"}
	if err := engine2.EventBus.Publish("resume:abc123", resumeEvent); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Wait for both echo_start and echo_resumed steps to appear (polling, up to 2s)
	var steps2 []*model.StepRun
	var run2 *model.Run
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run2, err = s2.GetLatestRunByFlowName(context.Background(), flow.Name)
		if err == nil && run2 != nil {
			steps2, err = s2.GetSteps(context.Background(), run2.ID)
			if err == nil {
				foundStart = false
				for _, step := range steps2 {
					if step.StepName == "echo_start" {
						foundStart = true
					}
				}
				if foundStart {
					break
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !foundStart {
		t.Fatalf("expected both echo_start and echo_resumed steps after resume")
	}
}

func TestSqliteQueryCompletedRunAfterRestart(t *testing.T) {
	// Use a temp SQLite file
	dbPath := filepath.Join(t.TempDir(), t.Name()+"-query_completed_run.db")

	// Load the echo_await_resume flow and remove the await_event step for this test
	f, err := os.ReadFile("../" + config.DefaultFlowsDir + "/echo_await_resume.flow.yaml")
	if err != nil {
		t.Fatalf("failed to read flow: %v", err)
	}
	var flow model.Flow
	if err := yaml.Unmarshal(f, &flow); err != nil {
		t.Fatalf("failed to unmarshal flow: %v", err)
	}
	// Remove the await_event and echo_resumed steps so the flow completes immediately and does not reference .event.resume_value
	var newSteps []model.Step
	for _, s := range flow.Steps {
		if s.AwaitEvent == nil && s.ID != "echo_resumed" {
			newSteps = append(newSteps, s)
		}
	}
	flow.Steps = newSteps

	// Create storage and engine
	s, err := storage.NewSqliteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	engine := NewEngineWithStorage(context.Background(), s)

	startEvent := map[string]any{"input": "hello world", "token": "abc123"}
	outputs, err := engine.Execute(context.Background(), &flow, startEvent)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if outputs["echo_start"] == nil {
		t.Fatalf("expected echo_start output, got: %v", outputs)
	}

	// Simulate a restart (new storage instance)
	s2, err := storage.NewSqliteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen sqlite storage: %v", err)
	}
	defer func() { _ = s2.Close() }()

	// Query the run and steps
	run, err := s2.GetLatestRunByFlowName(context.Background(), flow.Name)
	if err != nil {
		t.Fatalf("GetLatestRunByFlowName failed: %v", err)
	}
	if run == nil {
		t.Fatalf("expected run to be present after restart")
	}
	steps, err := s2.GetSteps(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetSteps failed: %v", err)
	}
	if len(steps) == 0 {
		t.Fatalf("expected steps to be present after restart")
	}
	var foundStart bool
	for _, step := range steps {
		if step.StepName == "echo_start" {
			foundStart = true
		}
	}
	if !foundStart {
		t.Fatalf("expected echo_start step after restart")
	}
}

func TestInMemoryFallback_ListAndGetRun(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow := &model.Flow{Name: "inmem", Steps: []model.Step{{ID: "s1", Use: "core.echo", With: map[string]interface{}{"text": "hi"}}}}
	outputs, err := e.Execute(context.Background(), flow, map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	runs, err := e.ListRuns(context.Background())
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected at least one run in memory")
	}
	run := runs[0]
	got, err := e.GetRunByID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRunByID error: %v", err)
	}
	if got == nil || got.ID != run.ID {
		t.Fatalf("expected to get run by ID, got: %v", got)
	}
	if outputs["s1"] == nil {
		t.Fatalf("expected outputs for s1, got: %v", outputs)
	}
	// Simulate restart (new engine, no persistence)
	e2 := NewDefaultEngine(context.Background())
	runs2, err := e2.ListRuns(context.Background())
	if err != nil {
		t.Fatalf("ListRuns error after restart: %v", err)
	}
	if len(runs2) != 0 {
		t.Fatalf("expected no runs after restart in in-memory mode, got: %d", len(runs2))
	}
}
