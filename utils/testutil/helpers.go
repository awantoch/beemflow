package testutil

import (
	"os"
	"testing"
)

// WithCleanDir removes the specified directory before and after running tests.
func WithCleanDir(m *testing.M, dir string) {
	// Clean up before tests
	os.RemoveAll(dir)
	// Run tests
	code := m.Run()
	// Clean up after tests
	os.RemoveAll(dir)
	os.Exit(code)
}

// CleanupDir removes the specified directory.
func CleanupDir(dir string) {
	os.RemoveAll(dir)
}

// WithCleanDirs removes all specified directories before and after running tests.
func WithCleanDirs(m *testing.M, dirs ...string) {
	// Clean up before tests
	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
	// Run tests
	code := m.Run()
	// Clean up after tests
	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
	os.Exit(code)
}
