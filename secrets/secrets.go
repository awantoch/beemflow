package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/awantoch/beemflow/config"
)

// NewSecretsProvider creates a secrets provider from BeemFlow configuration
func NewSecretsProvider(ctx context.Context, cfg *config.SecretsConfig) (SecretsProvider, error) {
	if cfg == nil {
		// Default to environment variables
		return NewEnvSecretsProvider(""), nil
	}

	switch strings.ToLower(cfg.Driver) {
	case "", "env":
		return NewEnvSecretsProvider(cfg.Prefix), nil
	case "aws-sm", "aws":
		if cfg.Region == "" {
			return nil, fmt.Errorf("region is required for AWS Secrets Manager")
		}
		return NewAWSSecretsProvider(ctx, cfg.Region, cfg.Prefix)
	default:
		return nil, fmt.Errorf("unsupported secrets driver: %s", cfg.Driver)
	}
}