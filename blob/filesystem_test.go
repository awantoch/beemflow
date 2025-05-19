package blob

import "testing"

func newTestFilesystemBlobStore(t *testing.T) *FilesystemBlobStore {
	dir := t.TempDir()
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
	url, err := store.Put(value, mime, filename)
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}
	got, err := store.Get(url)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("expected %q, got %q", value, got)
	}
}

func TestFilesystemBlobStore_EmptyData(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	url, err := store.Put([]byte{}, "text/plain", "empty.txt")
	if err != nil {
		t.Errorf("Put failed for empty data: %v", err)
	}
	got, err := store.Get(url)
	if err != nil {
		t.Errorf("Get failed for empty data: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty data, got %v bytes", len(got))
	}
}

func TestFilesystemBlobStore_GetUnknownURL(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	_, err := store.Get("file:///does/not/exist.txt")
	if err == nil {
		t.Errorf("expected error for unknown file, got nil")
	}
}

func TestFilesystemBlobStore_Concurrency(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	done := make(chan bool, 2)
	go func() {
		if _, err := store.Put([]byte("a"), "text/plain", "a.txt"); err != nil {
			t.Errorf("Put failed: %v", err)
		}
		if _, err := store.Get("file://" + store.dir + "/a.txt"); err != nil {
			t.Errorf("Get failed: %v", err)
		}
		done <- true
	}()
	go func() {
		if _, err := store.Put([]byte("b"), "text/plain", "b.txt"); err != nil {
			t.Errorf("Put failed: %v", err)
		}
		if _, err := store.Get("file://" + store.dir + "/b.txt"); err != nil {
			t.Errorf("Get failed: %v", err)
		}
		done <- true
	}()
	<-done
	<-done
}

func TestFilesystemBlobStore_DoubleStore(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	_, err1 := store.Put([]byte("first"), "text/plain", "dup.txt")
	_, err2 := store.Put([]byte("second"), "text/plain", "dup.txt")
	if err1 != nil || err2 != nil {
		t.Errorf("expected no error for double store, got %v, %v", err1, err2)
	}
}

func TestFilesystemBlobStore_ErrorCase(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	url, err := store.Put(nil, "", "errorcase.txt")
	if err != nil {
		t.Errorf("expected no error for nil input, got %v", err)
	}
	got, err := store.Get(url)
	if err != nil {
		t.Errorf("Get failed for errorcase.txt: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty data, got %v bytes", len(got))
	}
}

func TestFilesystemBlobStore_InvalidURL(t *testing.T) {
	store := newTestFilesystemBlobStore(t)
	_, err := store.Get("not-a-file-url")
	if err == nil {
		t.Errorf("expected error for invalid file URL, got nil")
	}
}
