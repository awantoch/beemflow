package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// AWSSecretsProvider implements SecretsProvider using AWS Secrets Manager
type AWSSecretsProvider struct {
	client *secretsmanager.Client
	prefix string
}

var _ SecretsProvider = (*AWSSecretsProvider)(nil)

// NewAWSSecretsProvider creates a new AWS Secrets Manager provider
func NewAWSSecretsProvider(ctx context.Context, region, prefix string) (*AWSSecretsProvider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)
	
	return &AWSSecretsProvider{
		client: client,
		prefix: prefix,
	}, nil
}

// GetSecret retrieves a secret from AWS Secrets Manager
func (a *AWSSecretsProvider) GetSecret(ctx context.Context, key string) (string, error) {
	secretName := key
	if a.prefix != "" {
		secretName = a.prefix + key
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := a.client.GetSecretValue(ctx, input)
	if err != nil {
		// Try without prefix as fallback
		if a.prefix != "" && !strings.Contains(err.Error(), "ResourceNotFoundException") {
			return a.getSecretWithoutPrefix(ctx, key)
		}
		return "", fmt.Errorf("failed to get secret %s: %w", key, err)
	}

	if result.SecretString == nil {
		return "", fmt.Errorf("secret %s has no string value", key)
	}

	return *result.SecretString, nil
}

// getSecretWithoutPrefix tries to get the secret without the prefix
func (a *AWSSecretsProvider) getSecretWithoutPrefix(ctx context.Context, key string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(key),
	}

	result, err := a.client.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", key, err)
	}

	if result.SecretString == nil {
		return "", fmt.Errorf("secret %s has no string value", key)
	}

	return *result.SecretString, nil
}

// Close cleans up resources (no-op for AWS provider)
func (a *AWSSecretsProvider) Close() error {
	return nil
}