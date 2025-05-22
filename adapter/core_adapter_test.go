package adapter

import (
	"context"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/awantoch/beemflow/utils"
)

// TestCoreAdapter tests that CoreAdapter prints text and returns inputs.
func TestCoreAdapter(t *testing.T) {
	a := &CoreAdapter{}
	// Set debug mode
	os.Setenv("BEEMFLOW_DEBUG", "1")
	defer os.Unsetenv("BEEMFLOW_DEBUG")
	// capture logger output
	r, w, _ := os.Pipe()
	orig := os.Stderr
	utils.SetInternalOutput(w)

	in := map[string]any{"text": "echoed"}
	out, err := a.Execute(context.Background(), in)
	w.Close()
	utils.SetInternalOutput(orig)

	buf, _ := io.ReadAll(r)
	if len(buf) == 0 || string(buf) == "\n" {
		t.Errorf("expected echoed in logger output, got %q", buf)
	}
	if !reflect.DeepEqual(out, in) || err != nil {
		t.Errorf("expected inputs returned, got %v, %v", out, err)
	}
}
