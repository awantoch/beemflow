package blob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// BlobStore is the interface for pluggable blob storage backends.
type BlobStore interface {
	Put(data []byte, mime, filename string) (url string, err error)
	Get(url string) ([]byte, error)
}

// FilesystemBlobStore implements BlobStore using the local filesystem.
// This is the default and recommended blob store for local/dev/prod.
type FilesystemBlobStore struct {
	dir string
}

// NewFilesystemBlobStore creates a new FilesystemBlobStore with the given directory.
// The directory will be created if it does not exist.
func NewFilesystemBlobStore(dir string) (*FilesystemBlobStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FilesystemBlobStore{dir: dir}, nil
}

// Put stores the blob as a file in the directory. Returns a file:// URL.
func (f *FilesystemBlobStore) Put(data []byte, mime, filename string) (string, error) {
	if filename == "" {
		filename = fmt.Sprintf("blob-%d", time.Now().UnixNano())
	}
	path := filepath.Join(f.dir, filename)
	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return "file://" + path, nil
}

// Get retrieves the blob from the file:// URL.
func (f *FilesystemBlobStore) Get(url string) ([]byte, error) {
	const prefix = "file://"
	if !strings.HasPrefix(url, prefix) {
		return nil, fmt.Errorf("invalid file URL: %s", url)
	}
	path := url[len(prefix):]
	return os.ReadFile(path)
}

// S3BlobStore implements BlobStore using AWS S3.
// This is NOT the default. Use only if configured explicitly.
type S3BlobStore struct {
	client *s3.Client
	bucket string
	region string
}

func NewS3BlobStore(bucket, region string) (*S3BlobStore, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg)
	return &S3BlobStore{client: client, bucket: bucket, region: region}, nil
}

func (s *S3BlobStore) Put(data []byte, mime, filename string) (string, error) {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(filename),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(mime),
		ACL:         types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("s3://%s/%s", s.bucket, filename), nil
}

func (s *S3BlobStore) Get(url string) ([]byte, error) {
	// Expect url format: s3://bucket/key
	var key string
	_, err := fmt.Sscanf(url, "s3://%s/%s", &s.bucket, &key)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// BlobConfig is a minimal struct for blob store configuration.
type BlobConfig struct {
	Driver    string
	Directory string
	Bucket    string
	Region    string
}

// NewDefaultBlobStore returns a BlobStore based on config, or FilesystemBlobStore in ./beemflow-files if config is nil or empty.
func NewDefaultBlobStore(cfg *BlobConfig) (BlobStore, error) {
	if cfg == nil || cfg.Driver == "" || cfg.Driver == "filesystem" {
		dir := "./beemflow-files"
		if cfg != nil && cfg.Directory != "" {
			dir = cfg.Directory
		}
		return NewFilesystemBlobStore(dir)
	}
	if cfg.Driver == "s3" {
		if cfg.Bucket == "" || cfg.Region == "" {
			return nil, fmt.Errorf("s3 driver requires bucket and region")
		}
		return NewS3BlobStore(cfg.Bucket, cfg.Region)
	}
	return nil, fmt.Errorf("unsupported blob driver: %s", cfg.Driver)
}
