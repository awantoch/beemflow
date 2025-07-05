package api

import (
	"context"
	"io"

	"github.com/awantoch/beemflow/blob"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	beemengine "github.com/awantoch/beemflow/engine"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/secrets"
	"github.com/awantoch/beemflow/utils"
)

// InitializeDependencies sets up all the heavy dependencies (engine, storage, etc.)
// Returns a cleanup function that should be called when shutting down
func InitializeDependencies(cfg *config.Config) (func(), error) {
	// Initialize storage
	store, err := GetStoreFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize event bus
	var bus event.EventBus
	if cfg.Event != nil {
		bus, err = event.NewEventBusFromConfig(cfg.Event)
		if err != nil {
			utils.WarnCtx(context.Background(), "Failed to create event bus: %v, using in-memory fallback", "error", err)
			bus = event.NewInProcEventBus()
		}
	} else {
		bus = event.NewInProcEventBus()
	}

	// Initialize blob store
	var blobStore blob.BlobStore
	blobConfig := (*blob.BlobConfig)(nil)
	if cfg.Blob != nil {
		blobConfig = &blob.BlobConfig{
			Driver: cfg.Blob.Driver,
			Bucket: cfg.Blob.Bucket,
		}
	}
	blobStore, err = blob.NewDefaultBlobStore(context.Background(), blobConfig)
	if err != nil {
		utils.WarnCtx(context.Background(), "Failed to create blob store: %v, using nil fallback", "error", err)
		blobStore = nil
	}

	// Create engine
	adapters := beemengine.NewDefaultAdapterRegistry(context.Background())
	templ := dsl.NewTemplater()
	engine := beemengine.NewEngine(adapters, templ, bus, blobStore, store)
	
	// Initialize secrets provider
	if secretsProvider, err := secrets.NewSecretsProvider(context.Background(), cfg.Secrets); err == nil {
		engine.SetSecretsProvider(secretsProvider)
	} else {
		utils.WarnCtx(context.Background(), "Failed to create secrets provider: %v, using default", "error", err)
	}

	// Return cleanup function
	cleanup := func() {
		if err := engine.Close(); err != nil {
			utils.Error("Failed to close engine: %v", err)
		}
		if store != nil {
			if closer, ok := store.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					utils.Error("Failed to close storage: %v", err)
				}
			}
		}
		if blobStore != nil {
			if closer, ok := blobStore.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					utils.Error("Failed to close blob store: %v", err)
				}
			}
		}
	}

	return cleanup, nil
}
