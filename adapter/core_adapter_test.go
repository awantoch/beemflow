package adapter

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

// TestCoreAdapter tests that CoreAdapter prints text and returns inputs.
func TestCoreAdapter(t *testing.T) {
	a := &CoreAdapter{}
	// capture stdout
	r, w, _ := os.Pipe()
	origOut := os.Stdout
	os.Stdout = w

	in := map[string]any{"text": "echoed"}
	out, err := a.Execute(context.Background(), in)
	w.Close()
	os.Stdout = origOut

	buf, _ := ioutil.ReadAll(r)
	if string(buf) != "echoed\n" {
		t.Errorf("expected echoed newline, got %q", buf)
	}
	if !reflect.DeepEqual(out, in) || err != nil {
		t.Errorf("expected inputs returned, got %v, %v", out, err)
	}
}
