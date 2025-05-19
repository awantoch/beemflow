package blob

import (
	"github.com/awantoch/beemflow/logger"
)

// BlobStore is the interface for pluggable blob storage backends.
type BlobStore interface {
	Put(data []byte, mime, filename string) (url string, err error)
	Get(url string) ([]byte, error)
}

// See filesystem.go and s3.go for driver implementations.

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
		dir := "./.beemflow/files"
		if cfg != nil && cfg.Directory != "" {
			dir = cfg.Directory
		}
		return NewFilesystemBlobStore(dir)
	}
	if cfg.Driver == "s3" {
		if cfg.Bucket == "" || cfg.Region == "" {
			return nil, logger.Errorf("s3 driver requires bucket and region")
		}
		return NewS3BlobStore(cfg.Bucket, cfg.Region)
	}
	return nil, logger.Errorf("unsupported blob driver: %s", cfg.Driver)
}
