package secrets

import (
	"context"
	"fmt"
	"os"
)

// EnvSecretsProvider implements SecretsProvider using environment variables
type EnvSecretsProvider struct {
	prefix string
}

var _ SecretsProvider = (*EnvSecretsProvider)(nil)

// NewEnvSecretsProvider creates a new environment variable secrets provider
func NewEnvSecretsProvider(prefix string) *EnvSecretsProvider {
	return &EnvSecretsProvider{
		prefix: prefix,
	}
}

// GetSecret retrieves a secret from environment variables
func (e *EnvSecretsProvider) GetSecret(ctx context.Context, key string) (string, error) {
	envKey := key
	if e.prefix != "" {
		envKey = e.prefix + key
	}
	
	value := os.Getenv(envKey)
	if value == "" {
		// Try without prefix as fallback
		if e.prefix != "" {
			value = os.Getenv(key)
		}
		if value == "" {
			return "", fmt.Errorf("secret not found in environment variables: %s", key)
		}
	}
	
	return value, nil
}

// Close cleans up resources (no-op for environment provider)
func (e *EnvSecretsProvider) Close() error {
	return nil
}