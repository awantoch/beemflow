package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/utils"
)

func captureOutput(f func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	utils.SetUserOutput(w)
	f()
	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		log.Printf("buf.ReadFrom failed: %v", err)
	}
	os.Stdout = orig
	utils.SetUserOutput(orig)
	return buf.String()
}

func captureStderrExit(f func()) (string, int) {
	origStderr := os.Stderr
	origExit := exit
	r, w, _ := os.Pipe()
	os.Stderr = w
	utils.SetInternalOutput(w)
	var buf bytes.Buffer
	var out string
	exitCode := 0
	exit = func(code int) {
		exitCode = code
		w.Close()
		panic("exit")
	}
	func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic occurred: %v", err)
			}
		}()
		f()
	}()
	w.Close()
	if _, err := io.Copy(&buf, r); err != nil {
		log.Printf("io.Copy failed: %v", err)
	}
	os.Stderr = origStderr
	utils.SetInternalOutput(origStderr)
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
		out := captureOutput(func() {
			if err := NewRootCmd().Execute(); err != nil {
				log.Printf("Execute failed: %v", err)
			}
		})
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
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, t.Name()+"-valid.flow.yaml")
	tmp, err := os.Create(tmpPath)
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmpPath)
	if _, err := tmp.Write([]byte(valid)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	os.Args = []string{"flow", "lint", tmpPath}
	out := captureOutput(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if !strings.Contains(out, "Lint OK") {
		t.Errorf("expected Lint OK, got %q", out)
	}

	os.Args = []string{"flow", "validate", tmpPath}
	out = captureOutput(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if !strings.Contains(out, "Validation OK") {
		t.Errorf("expected Validation OK, got %q", out)
	}

	// Missing file
	os.Args = []string{"flow", "lint", "/nonexistent/file.yaml"}
	stderr, code := captureStderrExit(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if code != 1 || !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("expected exit 1 and YAML parse error, got code=%d, stderr=%q", code, stderr)
	}

	// Invalid YAML (schema error, but valid YAML)
	bad := "name: bad\nsteps: [this is: not valid yaml]"
	tmpDir = t.TempDir()
	tmp2Path := filepath.Join(tmpDir, t.Name()+"-bad.flow.yaml")
	tmp2, err := os.Create(tmp2Path)
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp2Path)
	if _, err := tmp2.Write([]byte(bad)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp2.Close()
	os.Args = []string{"flow", "lint", tmp2Path}
	stderr, code = captureStderrExit(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if code != 2 || !strings.Contains(stderr, "Schema validation error") {
		t.Errorf("expected exit 2 and schema error, got code=%d, stderr=%q", code, stderr)
	}

	// Truly invalid YAML (parse error)
	badYAML := "name: bad\nsteps: [this is: not valid yaml"
	tmpDir = t.TempDir()
	tmp3Path := filepath.Join(tmpDir, t.Name()+"-bad2.flow.yaml")
	tmp3, err := os.Create(tmp3Path)
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp3Path)
	if _, err := tmp3.Write([]byte(badYAML)); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp3.Close()
	os.Args = []string{"flow", "lint", tmp3Path}
	stderr, code = captureStderrExit(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if code != 1 || !strings.Contains(stderr, "YAML parse error") {
		t.Errorf("expected exit 1 and YAML parse error, got code=%d, stderr=%q", code, stderr)
	}
}

func TestMain_ToolStub(t *testing.T) {
	os.Args = []string{"flow", "tool"}
	out := captureOutput(func() {
		if err := NewRootCmd().Execute(); err != nil {
			log.Printf("Execute failed: %v", err)
		}
	})
	if !strings.Contains(out, "flow tool (stub)") {
		t.Errorf("expected tool stub output, got %q", out)
	}
}

func TestMain(m *testing.M) {
	utils.WithCleanDir(m, config.DefaultConfigDir)
}
