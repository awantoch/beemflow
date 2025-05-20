package blob

import (
	"context"
	"os"
	"testing"
)

func newTestS3BlobStore(t *testing.T) *S3BlobStore {
	bucket := os.Getenv("S3_TEST_BUCKET")
	region := os.Getenv("S3_TEST_REGION")
	if bucket == "" || region == "" {
		t.Skip("S3_TEST_BUCKET or S3_TEST_REGION not set")
	}
	store, err := NewS3BlobStore(context.Background(), bucket, region)
	if err != nil {
		t.Fatalf("NewS3BlobStore failed: %v", err)
	}
	return store
}

func TestNewS3BlobStore(t *testing.T) {
	_ = newTestS3BlobStore(t)
}

func TestBlobStore_RoundTrip(t *testing.T) {
	store := newTestS3BlobStore(t)
	value := []byte("test-data")
	mime := "text/plain"
	filename := "test.txt"
	url, err := store.Put(context.Background(), value, mime, filename)
	if err != nil {
		t.Errorf("Put failed: %v", err)
	}
	_, err = store.Get(context.Background(), url)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
}

func TestBlobStore_EmptyData(t *testing.T) {
	store := newTestS3BlobStore(t)
	url, err := store.Put(context.Background(), []byte{}, "text/plain", "empty.txt")
	if err != nil {
		t.Errorf("Put failed for empty data: %v", err)
	}
	_, err = store.Get(context.Background(), url)
	if err != nil {
		t.Errorf("Get failed for empty data: %v", err)
	}
}

func TestBlobStore_GetUnknownURL(t *testing.T) {
	store := newTestS3BlobStore(t)
	_, err := store.Get(context.Background(), "s3://dummy-url/unknown.txt")
	if err != nil {
		t.Errorf("expected no error for unknown url (stub), got %v", err)
	}
}

func TestBlobStore_Concurrency(t *testing.T) {
	store := newTestS3BlobStore(t)
	done := make(chan bool, 2)
	go func() {
		if _, err := store.Put(context.Background(), []byte("a"), "text/plain", "a.txt"); err != nil {
			t.Errorf("Put failed: %v", err)
		}
		if _, err := store.Get(context.Background(), "s3://dummy-url/a.txt"); err != nil {
			t.Errorf("Get failed: %v", err)
		}
		done <- true
	}()
	go func() {
		if _, err := store.Put(context.Background(), []byte("b"), "text/plain", "b.txt"); err != nil {
			t.Errorf("Put failed: %v", err)
		}
		if _, err := store.Get(context.Background(), "s3://dummy-url/b.txt"); err != nil {
			t.Errorf("Get failed: %v", err)
		}
		done <- true
	}()
	<-done
	<-done
}

func TestBlobStore_DoubleStore(t *testing.T) {
	store := newTestS3BlobStore(t)
	_, err1 := store.Put(context.Background(), []byte("first"), "text/plain", "dup.txt")
	_, err2 := store.Put(context.Background(), []byte("second"), "text/plain", "dup.txt")
	if err1 != nil || err2 != nil {
		t.Errorf("expected no error for double store, got %v, %v", err1, err2)
	}
}

func TestBlobStore_ErrorCase(t *testing.T) {
	store := newTestS3BlobStore(t)
	_, err := store.Put(context.Background(), nil, "", "")
	if err != nil {
		t.Errorf("expected no error for nil/empty input (stub), got %v", err)
	}
}

func TestS3BlobStore_InvalidURL(t *testing.T) {
	bucket := os.Getenv("S3_TEST_BUCKET")
	region := os.Getenv("S3_TEST_REGION")
	if bucket == "" || region == "" {
		t.Skip("S3_TEST_BUCKET or S3_TEST_REGION not set")
	}
	store, err := NewS3BlobStore(context.Background(), bucket, region)
	if err != nil {
		t.Fatalf("NewS3BlobStore failed: %v", err)
	}
	_, err = store.Get(context.Background(), "not-an-s3-url")
	if err == nil {
		t.Errorf("expected error for invalid s3 URL, got nil")
	}
}

func TestS3BlobStore_InvalidConfig(t *testing.T) {
	_, err := NewS3BlobStore(context.Background(), "", "")
	if err == nil {
		t.Errorf("expected error for invalid S3 config, got nil")
	}
}
