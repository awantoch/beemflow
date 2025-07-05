package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AWSSecretsProvider implements SecretsProvider for AWS Secrets Manager
type AWSSecretsProvider struct {
	client    *secretsmanager.Client
	region    string
	prefix    string
	cache     SecretCache
	timeout   time.Duration
	batchSize int
	mu        sync.RWMutex
	metrics   *AWSMetrics
}

// AWSSecretsConfig contains AWS-specific configuration
type AWSSecretsConfig struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken    string `json:"session_token,omitempty"`
	RoleARN         string `json:"role_arn,omitempty"`
	Profile         string `json:"profile,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"` // For testing with LocalStack
}

// AWSMetrics tracks performance metrics for AWS provider
type AWSMetrics struct {
	RequestsTotal   int64
	ErrorsTotal     int64
	CacheHits       int64
	CacheMisses     int64
	AvgResponseTime time.Duration
	mu              sync.RWMutex
}

// NewAWSSecretsProvider creates a new AWS Secrets Manager provider
func NewAWSSecretsProvider(ctx context.Context, awsConfig *AWSSecretsConfig, providerConfig *ProviderConfig) (*AWSSecretsProvider, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(awsConfig.Region),
	)
	if err != nil {
		return nil, NewSecretError("CONFIG_ERROR", "failed to load AWS config", "aws-sm", "", err)
	}

	// Handle custom credentials
	if awsConfig.AccessKeyID != "" && awsConfig.SecretAccessKey != "" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider(
			awsConfig.AccessKeyID,
			awsConfig.SecretAccessKey,
			awsConfig.SessionToken,
		)
	}

	// Handle role assumption
	if awsConfig.RoleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClient, awsConfig.RoleARN)
	}

	// Create the Secrets Manager client
	smClient := secretsmanager.NewFromConfig(cfg)

	// Override endpoint for testing
	if awsConfig.Endpoint != "" {
		smClient = secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
			o.BaseEndpoint = aws.String(awsConfig.Endpoint)
		})
	}

	// Set up caching if enabled
	var cache SecretCache
	if providerConfig.CacheConfig != nil && providerConfig.CacheConfig.Enabled {
		cache = NewLRUCache(providerConfig.CacheConfig.MaxSize, providerConfig.CacheConfig.TTL)
	}

	// Set defaults
	timeout := 30 * time.Second
	if providerConfig.Timeout > 0 {
		timeout = providerConfig.Timeout
	}

	batchSize := 50
	if providerConfig.BatchSize > 0 {
		batchSize = providerConfig.BatchSize
	}

	return &AWSSecretsProvider{
		client:    smClient,
		region:    awsConfig.Region,
		prefix:    providerConfig.Prefix,
		cache:     cache,
		timeout:   timeout,
		batchSize: batchSize,
		metrics:   &AWSMetrics{},
	}, nil
}

// Type returns the provider type identifier
func (p *AWSSecretsProvider) Type() string {
	return "aws-sm"
}

// GetSecret retrieves a single secret from AWS Secrets Manager
func (p *AWSSecretsProvider) GetSecret(ctx context.Context, key string) (string, error) {
	start := time.Now()
	defer func() {
		p.updateMetrics(time.Since(start), 1, 0)
	}()

	// Check cache first
	if p.cache != nil {
		if cached, found := p.cache.Get(key); found {
			p.metrics.mu.Lock()
			p.metrics.CacheHits++
			p.metrics.mu.Unlock()
			return cached.Value, nil
		}
		p.metrics.mu.Lock()
		p.metrics.CacheMisses++
		p.metrics.mu.Unlock()
	}

	// Prepare the secret name with prefix
	secretName := p.buildSecretName(key)

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Get the secret from AWS
	result, err := p.client.GetSecretValue(timeoutCtx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		p.updateMetrics(0, 0, 1)
		return "", p.handleAWSError(err, key)
	}

	if result.SecretString == nil {
		return "", NewSecretError("INVALID_SECRET", "secret value is not a string", "aws-sm", key, nil)
	}

	value := *result.SecretString

	// Cache the result
	if p.cache != nil {
		secretValue := &SecretValue{
			Value:     value,
			Version:   aws.ToString(result.VersionId),
			CreatedAt: aws.ToTime(result.CreatedDate),
		}
		if result.Name != nil {
			secretValue.Metadata = map[string]string{
				"arn":  aws.ToString(result.ARN),
				"name": aws.ToString(result.Name),
			}
		}
		p.cache.Set(key, secretValue, p.timeout)
	}

	return value, nil
}

// GetSecrets retrieves multiple secrets efficiently using batch operations
func (p *AWSSecretsProvider) GetSecrets(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	start := time.Now()
	defer func() {
		p.updateMetrics(time.Since(start), int64(len(keys)), 0)
	}()

	results := make(map[string]string)
	var uncachedKeys []string

	// Check cache for all keys first
	if p.cache != nil {
		for _, key := range keys {
			if cached, found := p.cache.Get(key); found {
				results[key] = cached.Value
				p.metrics.mu.Lock()
				p.metrics.CacheHits++
				p.metrics.mu.Unlock()
			} else {
				uncachedKeys = append(uncachedKeys, key)
				p.metrics.mu.Lock()
				p.metrics.CacheMisses++
				p.metrics.mu.Unlock()
			}
		}
	} else {
		uncachedKeys = keys
	}

	if len(uncachedKeys) == 0 {
		return results, nil
	}

	// Process uncached keys in batches
	for i := 0; i < len(uncachedKeys); i += p.batchSize {
		end := i + p.batchSize
		if end > len(uncachedKeys) {
			end = len(uncachedKeys)
		}

		batch := uncachedKeys[i:end]
		batchResults, err := p.getBatchSecrets(ctx, batch)
		if err != nil {
			// If batch fails, try individual requests
			for _, key := range batch {
				if value, err := p.GetSecret(ctx, key); err == nil {
					results[key] = value
				}
			}
		} else {
			for k, v := range batchResults {
				results[k] = v
			}
		}
	}

	return results, nil
}

// getBatchSecrets handles batch retrieval of secrets
func (p *AWSSecretsProvider) getBatchSecrets(ctx context.Context, keys []string) (map[string]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	results := make(map[string]string)

	// AWS Secrets Manager doesn't have native batch operations, so we use concurrent requests
	type secretResult struct {
		key   string
		value string
		err   error
	}

	resultChan := make(chan secretResult, len(keys))
	semaphore := make(chan struct{}, 10) // Limit concurrent requests

	for _, key := range keys {
		go func(k string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			secretName := p.buildSecretName(k)
			result, err := p.client.GetSecretValue(timeoutCtx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String(secretName),
			})

			if err != nil {
				resultChan <- secretResult{key: k, err: err}
				return
			}

			if result.SecretString != nil {
				value := *result.SecretString
				
				// Cache the result
				if p.cache != nil {
					secretValue := &SecretValue{
						Value:     value,
						Version:   aws.ToString(result.VersionId),
						CreatedAt: aws.ToTime(result.CreatedDate),
					}
					if result.Name != nil {
						secretValue.Metadata = map[string]string{
							"arn":  aws.ToString(result.ARN),
							"name": aws.ToString(result.Name),
						}
					}
					p.cache.Set(k, secretValue, p.timeout)
				}

				resultChan <- secretResult{key: k, value: value}
			} else {
				resultChan <- secretResult{key: k, err: fmt.Errorf("secret value is not a string")}
			}
		}(key)
	}

	// Collect results
	for i := 0; i < len(keys); i++ {
		result := <-resultChan
		if result.err == nil {
			results[result.key] = result.value
		}
		// Note: We don't fail the entire batch if one secret fails
	}

	return results, nil
}

// GetSecretWithMetadata retrieves a secret with full metadata
func (p *AWSSecretsProvider) GetSecretWithMetadata(ctx context.Context, key string) (*SecretValue, error) {
	start := time.Now()
	defer func() {
		p.updateMetrics(time.Since(start), 1, 0)
	}()

	// Check cache first
	if p.cache != nil {
		if cached, found := p.cache.Get(key); found {
			p.metrics.mu.Lock()
			p.metrics.CacheHits++
			p.metrics.mu.Unlock()
			return cached, nil
		}
		p.metrics.mu.Lock()
		p.metrics.CacheMisses++
		p.metrics.mu.Unlock()
	}

	secretName := p.buildSecretName(key)
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	result, err := p.client.GetSecretValue(timeoutCtx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		p.updateMetrics(0, 0, 1)
		return nil, p.handleAWSError(err, key)
	}

	if result.SecretString == nil {
		return nil, NewSecretError("INVALID_SECRET", "secret value is not a string", "aws-sm", key, nil)
	}

	secretValue := &SecretValue{
		Value:     *result.SecretString,
		Version:   aws.ToString(result.VersionId),
		CreatedAt: aws.ToTime(result.CreatedDate),
		Metadata: map[string]string{
			"arn":    aws.ToString(result.ARN),
			"name":   aws.ToString(result.Name),
			"region": p.region,
		},
	}

	// Cache the result
	if p.cache != nil {
		p.cache.Set(key, secretValue, p.timeout)
	}

	return secretValue, nil
}

// ListSecrets returns a list of secret names with the given prefix
func (p *AWSSecretsProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	var secrets []string
	fullPrefix := p.buildSecretName(prefix)

	paginator := secretsmanager.NewListSecretsPaginator(p.client, &secretsmanager.ListSecretsInput{
		MaxResults: aws.Int32(100),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(timeoutCtx)
		if err != nil {
			return nil, p.handleAWSError(err, prefix)
		}

		for _, secret := range output.SecretList {
			if secret.Name != nil {
				name := *secret.Name
				if strings.HasPrefix(name, fullPrefix) {
					// Remove prefix to get the original key
					key := strings.TrimPrefix(name, p.prefix)
					secrets = append(secrets, key)
				}
			}
		}
	}

	return secrets, nil
}

// HealthCheck verifies the provider is working correctly
func (p *AWSSecretsProvider) HealthCheck(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try to list secrets to verify connectivity and permissions
	_, err := p.client.ListSecrets(timeoutCtx, &secretsmanager.ListSecretsInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return NewSecretError("HEALTH_CHECK_FAILED", "AWS Secrets Manager health check failed", "aws-sm", "", err)
	}

	return nil
}

// Close cleans up resources
func (p *AWSSecretsProvider) Close() error {
	if p.cache != nil {
		p.cache.Clear()
	}
	return nil
}

// buildSecretName constructs the full secret name with prefix
func (p *AWSSecretsProvider) buildSecretName(key string) string {
	if p.prefix == "" {
		return key
	}
	return p.prefix + key
}

// handleAWSError converts AWS errors to SecretError types
func (p *AWSSecretsProvider) handleAWSError(err error, key string) error {
	var notFoundErr *types.ResourceNotFoundException
	var accessDeniedErr *types.AccessDeniedException
	var quotaErr *types.LimitExceededException

	switch {
	case aws.ErrorAs(err, &notFoundErr):
		return NewSecretError("SECRET_NOT_FOUND", "secret not found", "aws-sm", key, err)
	case aws.ErrorAs(err, &accessDeniedErr):
		return NewSecretError("ACCESS_DENIED", "access denied", "aws-sm", key, err)
	case aws.ErrorAs(err, &quotaErr):
		return NewSecretError("QUOTA_EXCEEDED", "AWS quota exceeded", "aws-sm", key, err)
	default:
		return NewSecretError("PROVIDER_ERROR", "AWS Secrets Manager error", "aws-sm", key, err)
	}
}

// updateMetrics updates internal performance metrics
func (p *AWSSecretsProvider) updateMetrics(duration time.Duration, requests, errors int64) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.RequestsTotal += requests
	p.metrics.ErrorsTotal += errors
	
	if requests > 0 {
		// Update average response time using exponential moving average
		if p.metrics.AvgResponseTime == 0 {
			p.metrics.AvgResponseTime = duration
		} else {
			p.metrics.AvgResponseTime = time.Duration(
				0.9*float64(p.metrics.AvgResponseTime) + 0.1*float64(duration),
			)
		}
	}
}

// GetMetrics returns current performance metrics
func (p *AWSSecretsProvider) GetMetrics() AWSMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()
	return *p.metrics
}

// parseJSONSecret attempts to parse a JSON secret and extract a specific field
func parseJSONSecret(secretString, field string) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(secretString), &data); err != nil {
		return "", err
	}

	if value, ok := data[field]; ok {
		if str, ok := value.(string); ok {
			return str, nil
		}
		return fmt.Sprintf("%v", value), nil
	}

	return "", fmt.Errorf("field %s not found in secret", field)
}

// AWSSecretsProviderFactory creates AWS Secrets Manager providers
type AWSSecretsProviderFactory struct{}

// CreateProvider creates a new AWS Secrets Manager provider
func (f *AWSSecretsProviderFactory) CreateProvider(config *ProviderConfig) (SecretsProvider, error) {
	// This would be called by the main factory with the full configuration
	// For now, we'll assume the AWS config is embedded in a custom field
	return nil, fmt.Errorf("use NewAWSSecretsProvider directly")
}

// SupportedDrivers returns the drivers supported by this factory
func (f *AWSSecretsProviderFactory) SupportedDrivers() []string {
	return []string{"aws-sm"}
}