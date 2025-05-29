package utils

import (
	"os"
	"testing"
)

// WithCleanDir removes the specified directory before and after running tests.
// Returns the exit code instead of calling os.Exit() for safer test execution.
func WithCleanDir(m *testing.M, dir string) int {
	os.RemoveAll(dir)
	code := m.Run()
	os.RemoveAll(dir)
	return code
}

// WithCleanDirs removes all specified directories before and after running tests.
// Returns the exit code instead of calling os.Exit() for safer test execution.
func WithCleanDirs(m *testing.M, dirs ...string) int {
	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
	code := m.Run()
	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
	return code
}

// CleanupDir removes the specified directory.
func CleanupDir(dir string) {
	os.RemoveAll(dir)
}

func CleanupTempDir(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		// Non-critical cleanup error, log but don't fail
		Error("Failed to cleanup temp directory %s: %v", dir, err)
	}
}

func CleanupTempDirT(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: failed to cleanup temp directory %s: %v", dir, err)
	}
}

func WithCleanup[T any](t *testing.T, setup func() (T, string), test func(T)) {
	resource, dir := setup()
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("Warning: failed to cleanup temp directory %s: %v", dir, err)
		}
	}()
	test(resource)
}
