package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/parser"
	"github.com/awantoch/beemflow/pkg/logger"
)

func captureOutput(f func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = orig
	return buf.String()
}

func captureStderrExit(f func()) (string, int) {
	origStderr := os.Stderr
	origExit := exit
	r, w, _ := os.Pipe()
	os.Stderr = w
	logger.Logger.SetOutput(w)
	exitCode := 0
	exit = func(code int) {
		exitCode = code
		w.Close()
		panic("exit")
	}
	var buf bytes.Buffer
	var out string
	func() {
		defer func() {
			recover()
		}()
		f()
	}()
	w.Close()
	io.Copy(&buf, r)
	os.Stderr = origStderr
	logger.Logger.SetOutput(origStderr)
	exit = origExit
	out = buf.String()
	return out, exitCode
}

func TestMainCommands(t *testing.T) {
	cases := []struct {
		args        []string
		wantsOutput bool
	}{
		{[]string{"flow", "serve"}, true},
		{[]string{"flow", "run"}, true},
		{[]string{"flow", "test"}, true},
		{[]string{"flow", "tool", "scaffold"}, true},
	}
	for _, c := range cases {
		os.Args = c.args
		out := captureOutput(func() { NewRootCmd().Execute() })
		if c.wantsOutput && out == "" {
			t.Errorf("expected output for %v, got empty", c.args)
		}
	}
}

func TestMain_LintValidateCommands(t *testing.T) {
	// Valid flow file
	valid := `name: test
on: cli.manual
steps:
  - id: s1
    use: core.echo
    with: {text: hi}`
	tmp, err := os.CreateTemp("", "valid.flow.yaml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write([]byte(valid)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	os.Args = []string{"flow", "lint", tmp.Name()}
	out := captureOutput(func() { NewRootCmd().Execute() })
	if !strings.Contains(out, "Lint OK") {
		t.Errorf("expected Lint OK, got %q", out)
	}

	os.Args = []string{"flow", "validate", tmp.Name()}
	out = captureOutput(func() { NewRootCmd().Execute() })
	if !strings.Contains(out, "Validation OK") {
		t.Errorf("expected Validation OK, got %q", out)
	}

	// Missing file
	os.Args = []string{"flow", "lint", "/nonexistent/file.yaml"}
	stderr, code := captureStderrExit(func() { NewRootCmd().Execute() })
	if code != 1 || !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("expected exit 1 and YAML parse error, got code=%d, stderr=%q", code, stderr)
	}

	// Invalid YAML (schema error, but valid YAML)
	bad := "name: bad\nsteps: [this is: not valid yaml]"
	tmp2, err := os.CreateTemp("", "bad.flow.yaml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp2.Name())
	if _, err := tmp2.Write([]byte(bad)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp2.Close()
	os.Args = []string{"flow", "lint", tmp2.Name()}
	stderr, code = captureStderrExit(func() { NewRootCmd().Execute() })
	if code != 2 || !strings.Contains(stderr, "Schema validation error") {
		t.Errorf("expected exit 2 and schema error, got code=%d, stderr=%q", code, stderr)
	}

	// Truly invalid YAML (parse error)
	badYAML := "name: bad\nsteps: [this is: not valid yaml"
	tmp3, err := os.CreateTemp("", "bad2.flow.yaml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp3.Name())
	if _, err := tmp3.Write([]byte(badYAML)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp3.Close()
	os.Args = []string{"flow", "lint", tmp3.Name()}
	stderr, code = captureStderrExit(func() { NewRootCmd().Execute() })
	if code != 1 || !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("expected exit 1 and YAML parse error, got code=%d, stderr=%q", code, stderr)
	}

	// Schema error (simulate by patching parser.ValidateFlow)
	os.Args = []string{"flow", "lint", tmp.Name()}
	origValidate := parser.ValidateFlow
	parser.ValidateFlow = func(flow *model.Flow, schemaPath string) error { return fmt.Errorf("schema fail") }
	stderr, code = captureStderrExit(func() { NewRootCmd().Execute() })
	parser.ValidateFlow = origValidate
	if code != 2 || !strings.Contains(stderr, "Schema validation error") {
		t.Errorf("expected exit 2 and schema error, got code=%d, stderr=%q", code, stderr)
	}
}

func TestMain_ToolStub(t *testing.T) {
	os.Args = []string{"flow", "tool"}
	out := captureOutput(func() { NewRootCmd().Execute() })
	if !strings.Contains(out, "flow tool (stub)") {
		t.Errorf("expected tool stub output, got %q", out)
	}
}
