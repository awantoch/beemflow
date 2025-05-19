package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSqliteStorage_FileCreation(t *testing.T) {
	t.Run("WithSubdir", func(t *testing.T) {
		// Create a temporary base directory
		tmp := t.TempDir()
		// Define a nested subdirectory that does not exist
		nested := filepath.Join(tmp, "nested", "subdir")
		dsn := filepath.Join(nested, t.Name()+"-test.db")
		// Call NewSqliteStorage, which should create the nested directories and file
		s, err := NewSqliteStorage(dsn)
		if err != nil {
			t.Fatalf("NewSqliteStorage failed: %v", err)
		}
		if s == nil {
			t.Fatalf("expected non-nil SqliteStorage for DSN %q", dsn)
		}
		// Check that the directory was created
		if info, err := os.Stat(nested); err != nil {
			t.Errorf("expected directory %q to exist, got error: %v", nested, err)
		} else if !info.IsDir() {
			t.Errorf("expected %q to be a directory", nested)
		}
		// Check that the database file was created
		if _, err := os.Stat(dsn); err != nil {
			t.Errorf("expected database file %q to exist, got error: %v", dsn, err)
		}
	})

	t.Run("WithoutSubdir", func(t *testing.T) {
		// Create a temporary base directory
		tmp := t.TempDir()
		// Define a DSN directly under the base (no subdirectory)
		dsn := filepath.Join(tmp, t.Name()+"-plain.db")
		// Call NewSqliteStorage, which should create the file in the existing directory
		s, err := NewSqliteStorage(dsn)
		if err != nil {
			t.Fatalf("NewSqliteStorage failed: %v", err)
		}
		if s == nil {
			t.Fatalf("expected non-nil SqliteStorage for DSN %q", dsn)
		}
		// The base directory should already exist; just check the file
		if _, err := os.Stat(dsn); err != nil {
			t.Errorf("expected database file %q to exist, got error: %v", dsn, err)
		}
	})
}
