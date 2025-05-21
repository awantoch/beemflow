package utils

import (
	"os"
	"testing"
)

// WithCleanDir removes the specified directory before and after running tests.
func WithCleanDir(m *testing.M, dir string) {
	os.RemoveAll(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// WithCleanDirs removes all specified directories before and after running tests.
func WithCleanDirs(m *testing.M, dirs ...string) {
	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
	code := m.Run()
	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
	os.Exit(code)
}

// CleanupDir removes the specified directory.
func CleanupDir(dir string) {
	os.RemoveAll(dir)
}
