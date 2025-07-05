package secrets

import (
	"context"
)

// SecretsProvider resolves secrets for flows
type SecretsProvider interface {
	GetSecret(ctx context.Context, key string) (string, error)
	Close() error
}