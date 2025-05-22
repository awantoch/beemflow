package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/event"
	pproto "github.com/awantoch/beemflow/spec/proto"
	"github.com/awantoch/beemflow/storage"
	"github.com/awantoch/beemflow/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	utils.WithCleanDirs(m, ".beemflow", config.DefaultConfigDir, config.DefaultFlowsDir)
}

func TestNewEngine(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	if e == nil {
		t.Error("expected NewEngine not nil")
	}
}

func TestExecuteNoop(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/noop.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	_, err = e.Execute(context.Background(), flow, ctx)
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
	flow, err := dsl.Load("../flows/noop.flow.yaml", map[string]any{})
	require.NoError(t, err)
	_, err = e.Execute(context.Background(), flow, nil)
	if err != nil {
		t.Errorf("expected nil error for nil event, got %v", err)
	}
}

func TestExecute_MinimalValidFlow(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/minimal.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	_, err = e.Execute(context.Background(), flow, ctx)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestExecute_AllStepTypes(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/await_missing.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	_, err = e.Execute(context.Background(), flow, ctx)
	if err == nil || !strings.Contains(err.Error(), "missing token in match") {
		t.Errorf("expected await_event missing token error, got %v", err)
	}
}

func TestExecute_Concurrency(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/minimal.flow.yaml", map[string]any{})
	require.NoError(t, err)
	done := make(chan bool, 2)
	go func() {
		_, _ = e.Execute(context.Background(), flow, map[string]any{"foo": "bar"})
		done <- true
	}()
	go func() {
		_, _ = e.Execute(context.Background(), flow, map[string]any{"foo": "baz"})
		done <- true
	}()
	<-done
	<-done
}

func TestAwaitEventResume_RoundTrip(t *testing.T) {
	flow, err := dsl.Load("../flows/echo_await_resume.flow.yaml", map[string]any{})
	require.NoError(t, err)
	engine := NewDefaultEngine(context.Background())
	ctx := map[string]any{"input": "hello world", "token": "abc123", "event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	outputs, err := engine.Execute(context.Background(), flow, ctx)
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
	// TODO: Add a catch_test.flow.yaml and refactor this test to use dsl.Load
}

func TestExecute_AdapterErrorPropagation(t *testing.T) {
	// TODO: Add an adapter_error.flow.yaml and refactor this test to use dsl.Load
}

func TestExecute_ParallelForeachEdgeCases(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/parallel_openai.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"list": []any{}, "prompt1": "Generate a fun fact about space", "prompt2": "Generate a fun fact about oceans", "event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	outputs, err := e.Execute(context.Background(), flow, ctx)
	if err != nil {
		t.Errorf("expected no error for empty foreach, got %v", err)
	}
	if out, ok := outputs["fanout"].(map[string]any); !ok || len(out) == 0 {
		t.Errorf("expected outputs to be map with empty map for fanout, got %v", outputs)
	}
}

func TestExecute_SecretsInjection(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/hello.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"secrets": map[string]any{"MY_SECRET": "shhh"}, "event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}}
	outputs, err := e.Execute(context.Background(), flow, ctx)
	if err != nil {
		t.Errorf("expected no error for secrets injection, got %v", err)
	}
	if out, ok := outputs["greet"].(map[string]any); !ok || out["text"] != "Hello, world, I'm BeemFlow!" {
		t.Errorf("expected secret injected as map output, got %v", outputs["greet"])
	}
}

func TestExecute_SecretsDotAccess(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/hello.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"secrets": map[string]any{"MY_SECRET": "shhh"}, "event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}}
	outputs, err := e.Execute(context.Background(), flow, ctx)
	if err != nil {
		t.Errorf("expected no error for secrets dot access, got %v", err)
	}
	if out, ok := outputs["greet"].(map[string]any); !ok || out["text"] != "Hello, world, I'm BeemFlow!" {
		t.Errorf("expected secret injected as map output, got %v", outputs["greet"])
	}
}

func TestExecute_ArrayAccessInTemplate(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/hello.flow.yaml", map[string]any{})
	require.NoError(t, err)
	arr := []map[string]any{{"val": "a"}, {"val": "b"}}
	ctx := map[string]any{"arr": arr, "event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	outputs, err := e.Execute(context.Background(), flow, ctx)
	if err != nil {
		t.Errorf("expected no error for array access, got %v", err)
	}
	if out, ok := outputs["greet"].(map[string]any); !ok || out["text"] != "Hello, world, I'm BeemFlow!" {
		t.Errorf("expected array access output, got %v", outputs["greet"])
	}
}

func TestSqlitePersistenceAndResume_FullFlow(t *testing.T) {
	// Use a temp SQLite file
	tmpDir := t.TempDir()
	// Cleanup temp dir (and any SQLite files) before automatic TempDir removal
	defer func() { os.RemoveAll(tmpDir) }()
	dbPath := filepath.Join(tmpDir, t.Name()+"-resume_fullflow.db")

	// Load the echo_await_resume flow
	flowPtr, err := dsl.Load("../"+config.DefaultFlowsDir+"/echo_await_resume.flow.yaml", map[string]any{})
	if err != nil {
		t.Fatalf("failed to load flow: %v", err)
	}
	flow := *flowPtr

	// Create storage and engine
	s, err := storage.NewSqliteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	engine := NewEngine(
		NewDefaultAdapterRegistry(context.Background()),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		s,
	)

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
	runUUID, err := uuid.Parse(run.GetId())
	if err != nil {
		t.Fatalf("invalid run id: %v", err)
	}
	steps, err := s.GetSteps(context.Background(), runUUID)
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
	engine2 := NewEngine(
		NewDefaultAdapterRegistry(context.Background()),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		s2,
	)

	// Simulate resume event
	resumeEvent := map[string]any{"resume_value": "it worked!", "token": "abc123"}
	if err := engine2.EventBus.Publish("resume:abc123", resumeEvent); err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// Wait for both echo_start and echo_resumed steps to appear (polling, up to 2s)
	var steps2 []*pproto.StepRun
	var run2 *pproto.Run
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run2, err = s2.GetLatestRunByFlowName(context.Background(), flow.Name)
		if err == nil && run2 != nil {
			run2UUID, err := uuid.Parse(run2.GetId())
			if err != nil {
				t.Fatalf("invalid run2 id: %v", err)
			}
			steps2, err = s2.GetSteps(context.Background(), run2UUID)
			if err == nil {
				foundStart = false
				for _, step := range steps2 {
					if step.GetStepName() == "echo_start" {
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

	// Load the echo_await_resume flow and then remove await_event
	flowPtr, err := dsl.Load("../"+config.DefaultFlowsDir+"/echo_await_resume.flow.yaml", map[string]any{})
	if err != nil {
		t.Fatalf("failed to load flow: %v", err)
	}
	flow := *flowPtr
	// Remove the await_event and echo_resumed steps so the flow completes immediately and does not reference .event.resume_value
	var newSteps []*pproto.Step
	for _, s := range flow.GetSteps() {
		if s.GetAwaitEvent() == nil && s.GetId() != "echo_resumed" {
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
	engine := NewEngine(
		NewDefaultAdapterRegistry(context.Background()),
		dsl.NewTemplater(),
		event.NewInProcEventBus(),
		nil, // blob store not needed here
		s,
	)

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
	runUUID, err := uuid.Parse(run.GetId())
	if err != nil {
		t.Fatalf("invalid run id: %v", err)
	}
	steps, err := s2.GetSteps(context.Background(), runUUID)
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
	// TODO: Add an inmem.flow.yaml and refactor this test to use dsl.Load
}

func TestAllStepTypesFlow(t *testing.T) {
	e := NewDefaultEngine(context.Background())
	flow, err := dsl.Load("../flows/all_step_types.flow.yaml", map[string]any{})
	require.NoError(t, err)
	ctx := map[string]any{"test_token": "test-123", "test_list": []string{"a", "b"}, "event": map[string]any{}, "vars": map[string]any{}, "outputs": map[string]any{}, "secrets": map[string]any{}}
	outputs, err := e.Execute(context.Background(), flow, ctx)
	// Should error due to fail_step, but catch block should run
	if err == nil {
		t.Errorf("expected error from fail_step, got nil")
	}
	// Check exec step
	if out, ok := outputs["exec_step"].(map[string]any); !ok || out["text"] != "exec works" {
		t.Errorf("expected exec_step output, got %v", outputs["exec_step"])
	}
	// Check parallel steps
	if out, ok := outputs["parallel_1"].(map[string]any); !ok || out["text"] != "parallel 1" {
		t.Errorf("expected parallel_1 output, got %v", outputs["parallel_1"])
	}
	if out, ok := outputs["parallel_2"].(map[string]any); !ok || out["text"] != "parallel 2" {
		t.Errorf("expected parallel_2 output, got %v", outputs["parallel_2"])
	}
	// Check foreach
	if out, ok := outputs["foreach_echo"].([]any); !ok || len(out) != 2 {
		t.Errorf("expected foreach_echo outputs, got %v", outputs["foreach_echo"])
	}
	// Check catch steps
	if out, ok := outputs["catch1"].(map[string]any); !ok || out["text"] != "caught error!" {
		t.Errorf("expected catch1 output, got %v", outputs["catch1"])
	}
	if out, ok := outputs["catch2"].(map[string]any); !ok || out["text"] != "second catch!" {
		t.Errorf("expected catch2 output, got %v", outputs["catch2"])
	}
}
