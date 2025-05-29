package blob

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/utils"
)

func TestMain(m *testing.M) {
	os.Exit(utils.WithCleanDirs(m, ".beemflow", config.DefaultConfigDir))
}

func newTestFilesystemBlobStore(t *testing.T) *FilesystemBlobStore {
	dir := filepath.Join(t.TempDir(), t.Name()+"-blobstore")
	store, err := NewFilesystemBlobStore(dir)
	if err != nil {
		t.Fatalf("NewFilesystemBlobStore failed: %v", err)
	}
	return store
}

func TestFilesystemBlobStore_RoundTrip(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	value := []byte("test-data")
	mime := "text/plain"
	filename := "test.txt"
	url, err := store.Put(context.Background(), value, mime, filename)
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}
	got, err := store.Get(context.Background(), url)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("expected %q, got %q", value, got)
	}
}

func TestFilesystemBlobStore_EmptyData(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	url, err := store.Put(context.Background(), []byte{}, "text/plain", "empty.txt")
	if err != nil {
		t.Errorf("Put failed for empty data: %v", err)
	}
	got, err := store.Get(context.Background(), url)
	if err != nil {
		t.Errorf("Get failed for empty data: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty data, got %v bytes", len(got))
	}
}

func TestFilesystemBlobStore_GetUnknownURL(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	_, err := store.Get(context.Background(), "file:///does/not/exist.txt")
	if err == nil {
		t.Errorf("expected error for unknown file, got nil")
	}
}

func TestFilesystemBlobStore_Concurrency(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	done := make(chan bool, 2)
	go func() {
		if _, err := store.Put(context.Background(), []byte("a"), "text/plain", "a.txt"); err != nil {
			t.Errorf("Put failed: %v", err)
		}
		if _, err := store.Get(context.Background(), "file://"+store.dir+"/a.txt"); err != nil {
			t.Errorf("Get failed: %v", err)
		}
		done <- true
	}()
	go func() {
		if _, err := store.Put(context.Background(), []byte("b"), "text/plain", "b.txt"); err != nil {
			t.Errorf("Put failed: %v", err)
		}
		if _, err := store.Get(context.Background(), "file://"+store.dir+"/b.txt"); err != nil {
			t.Errorf("Get failed: %v", err)
		}
		done <- true
	}()
	<-done
	<-done
}

func TestFilesystemBlobStore_DoubleStore(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	_, err1 := store.Put(context.Background(), []byte("first"), "text/plain", "dup.txt")
	_, err2 := store.Put(context.Background(), []byte("second"), "text/plain", "dup.txt")
	if err1 != nil || err2 != nil {
		t.Errorf("expected no error for double store, got %v, %v", err1, err2)
	}
}

func TestFilesystemBlobStore_ErrorCase(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	url, err := store.Put(context.Background(), nil, "", "errorcase.txt")
	if err != nil {
		t.Errorf("expected no error for nil input, got %v", err)
	}
	got, err := store.Get(context.Background(), url)
	if err != nil {
		t.Errorf("Get failed for errorcase.txt: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty data, got %v bytes", len(got))
	}
}

func TestFilesystemBlobStore_InvalidURL(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	_, err := store.Get(context.Background(), "not-a-file-url")
	if err == nil {
		t.Errorf("expected error for invalid file URL, got nil")
	}
}

func TestFilesystemBlobStore_EmptyFilename(t *testing.T) {
	store := newTestFilesystemBlobStore(t)

	// Test with empty filename - should generate a unique filename
	url, err := store.Put(context.Background(), []byte("test data"), "text/plain", "")
	if err != nil {
		t.Errorf("Put with empty filename failed: %v", err)
	}

	if url == "" {
		t.Error("Expected non-empty URL for empty filename")
	}

	// Should be able to retrieve the data
	got, err := store.Get(context.Background(), url)
	if err != nil {
		t.Errorf("Get failed for auto-generated filename: %v", err)
	}

	expected := []byte("test data")
	if !bytes.Equal(got, expected) {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

// Tests for NewDefaultBlobStore function

func TestNewDefaultBlobStore(t *testing.T) {
	ctx := context.Background()

	// Test with nil config (should default to filesystem)
	store, err := NewDefaultBlobStore(ctx, nil)
	if err != nil {
		t.Errorf("NewDefaultBlobStore with nil config should not fail, got: %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}

	// Test with empty config (should default to filesystem)
	cfg := &BlobConfig{}
	store, err = NewDefaultBlobStore(ctx, cfg)
	if err != nil {
		t.Errorf("NewDefaultBlobStore with empty config should not fail, got: %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}

	// Test with filesystem driver using temp directory
	cfg = &BlobConfig{
		Driver:    "filesystem",
		Directory: filepath.Join(t.TempDir(), "test-blobs"),
	}
	store, err = NewDefaultBlobStore(ctx, cfg)
	if err != nil {
		t.Errorf("NewDefaultBlobStore with filesystem config should not fail, got: %v", err)
	}
	if store == nil {
		t.Error("Expected non-nil store")
	}

	// Test with s3 driver but missing bucket
	cfg = &BlobConfig{
		Driver: "s3",
		Region: "us-west-2",
	}
	store, err = NewDefaultBlobStore(ctx, cfg)
	if err == nil {
		t.Error("NewDefaultBlobStore with S3 config missing bucket should fail")
	}
	if store != nil {
		t.Error("Expected nil store for invalid S3 config")
	}

	// Test with unsupported driver
	cfg = &BlobConfig{
		Driver: "unsupported",
	}
	store, err = NewDefaultBlobStore(ctx, cfg)
	if err == nil {
		t.Error("NewDefaultBlobStore with unsupported driver should fail")
	}
	if store != nil {
		t.Error("Expected nil store for unsupported driver")
	}
}
