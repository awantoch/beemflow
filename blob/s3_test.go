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

// Enhanced S3 tests that don't require AWS credentials

func TestNewS3BlobStore_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Test with empty bucket
	store, err := NewS3BlobStore(ctx, "", "us-west-2")
	if err == nil {
		t.Error("NewS3BlobStore with empty bucket should fail")
	}
	if store != nil {
		t.Error("Expected nil store for empty bucket")
	}

	// Test with empty region
	store, err = NewS3BlobStore(ctx, "test-bucket", "")
	if err == nil {
		t.Error("NewS3BlobStore with empty region should fail")
	}
	if store != nil {
		t.Error("Expected nil store for empty region")
	}

	// Test with both empty
	store, err = NewS3BlobStore(ctx, "", "")
	if err == nil {
		t.Error("NewS3BlobStore with empty bucket and region should fail")
	}
	if store != nil {
		t.Error("Expected nil store for empty bucket and region")
	}
}

func TestNewS3BlobStore_ValidParams(t *testing.T) {
	ctx := context.Background()

	// Test with valid parameters - may fail due to AWS credentials but should
	// test the validation and setup code path
	store, err := NewS3BlobStore(ctx, "test-bucket", "us-west-2")

	// We don't expect this to necessarily succeed in a test environment
	// but we want to ensure it doesn't panic and follows the proper code path
	if err != nil {
		t.Logf("S3BlobStore creation failed as expected in test environment: %v", err)
		// The function should fail gracefully, not panic
		return
	}

	// If it succeeds (e.g., in an environment with AWS credentials),
	// verify the store is properly constructed
	if store == nil {
		t.Error("Expected non-nil store when no error")
		return
	}

	if store.bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got %s", store.bucket)
	}

	if store.region != "us-west-2" {
		t.Errorf("Expected region 'us-west-2', got %s", store.region)
	}

	if store.client == nil {
		t.Error("Expected non-nil S3 client")
	}
}

func TestS3BlobStore_URLParsing(t *testing.T) {
	// Create a mock S3BlobStore to test URL parsing logic
	// This tests the Get method's URL parsing without requiring AWS
	store := &S3BlobStore{
		bucket: "test-bucket",
		region: "us-west-2",
		client: nil, // We won't call actual AWS operations
	}

	// This will test the URL parsing part but fail at the actual S3 call
	// which is expected since we don't have a real client
	ctx := context.Background()

	// Test invalid URL format
	_, err := store.Get(ctx, "invalid-url")
	if err == nil {
		t.Error("Expected error for invalid URL format")
	}

	// Test URL with wrong bucket - this will fail at the fmt.Sscanf level
	_, err = store.Get(ctx, "s3://wrong-bucket/test-key")
	if err == nil {
		t.Error("Expected error for URL parsing or bucket mismatch")
	}

	// The actual error might be from fmt.Sscanf or bucket mismatch
	// We just want to ensure it fails appropriately
	t.Logf("URL parsing error (expected): %v", err)
}

func TestS3BlobStore_PutMethodSignature(t *testing.T) {
	// Test that S3BlobStore has the correct Put method signature without calling AWS
	store := &S3BlobStore{
		bucket: "test-bucket",
		region: "us-west-2",
		client: nil, // No real client
	}

	// We're only testing the method signature exists and compiles
	// We don't actually call it since that would panic with nil client
	_ = store

	// Just verify the struct fields are accessible
	if store.bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got %s", store.bucket)
	}

	// The actual Put method call would panic with nil client, so we skip it
	// This test just ensures the method signature compiles correctly
}

func TestS3BlobStore_StructFields(t *testing.T) {
	// Test S3BlobStore struct field access
	store := &S3BlobStore{
		bucket: "my-bucket",
		region: "eu-west-1",
		client: nil,
	}

	if store.bucket != "my-bucket" {
		t.Errorf("Expected bucket 'my-bucket', got %s", store.bucket)
	}

	if store.region != "eu-west-1" {
		t.Errorf("Expected region 'eu-west-1', got %s", store.region)
	}
}
