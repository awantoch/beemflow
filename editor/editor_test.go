package editor

import (
	"os"
	"testing"
)

// TestEditorFiles verifies that the editor build files exist
func TestEditorFiles(t *testing.T) {
	// Check WASM file exists
	if _, err := os.Stat("wasm/main.wasm"); err != nil {
		t.Errorf("WASM file not found: %v", err)
	}

	// Check WASM runtime exists
	if _, err := os.Stat("wasm/wasm_exec.js"); err != nil {
		t.Errorf("WASM runtime file not found: %v", err)
	}

	// Check web build exists
	if _, err := os.Stat("web/dist/index.html"); err != nil {
		t.Errorf("Web build not found: %v", err)
	}

	// Check web assets directory exists
	if _, err := os.Stat("web/dist/assets"); err != nil {
		t.Errorf("Web assets directory not found: %v", err)
	}
}

// TestWASMSize verifies the WASM file is reasonable size
func TestWASMSize(t *testing.T) {
	info, err := os.Stat("wasm/main.wasm")
	if err != nil {
		t.Skipf("WASM file not found, skipping size test: %v", err)
		return
	}

	size := info.Size()
	if size < 1024*1024 {
		t.Errorf("WASM file seems too small: %d bytes", size)
	}
	if size > 50*1024*1024 {
		t.Errorf("WASM file seems too large: %d bytes", size)
	}

	t.Logf("WASM file size: %.2f MB", float64(size)/(1024*1024))
}