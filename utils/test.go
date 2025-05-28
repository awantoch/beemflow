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
