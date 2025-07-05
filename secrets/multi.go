package secrets

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MultiSecretsProvider implements SecretsProvider by combining multiple providers
// It supports different strategies for handling multiple providers:
// - Failover: Try providers in order until one succeeds
// - Merge: Combine results from all providers (first provider wins on conflicts)
// - Priority: Use first provider, others as backup for missing secrets
type MultiSecretsProvider struct {
	providers []SecretsProvider
	strategy  MultiProviderStrategy
	timeout   time.Duration
	mu        sync.RWMutex
	metrics   *MultiProviderMetrics
}

// MultiProviderConfig defines configuration for multi-provider setup
type MultiProviderConfig struct {
	Strategy  MultiProviderStrategy `json:"strategy"`
	Providers []ProviderConfig      `json:"providers"`
	Timeout   time.Duration         `json:"timeout,omitempty"`
}

// MultiProviderMetrics tracks performance across multiple providers
type MultiProviderMetrics struct {
	RequestsTotal       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	ProviderFailures    map[string]int64
	ProviderSuccesses   map[string]int64
	StrategyDistribution map[string]int64
	AvgResponseTime     time.Duration
	mu                  sync.RWMutex
}

// NewMultiSecretsProvider creates a new multi-provider with the given strategy
func NewMultiSecretsProvider(providers []SecretsProvider, strategy MultiProviderStrategy, timeout time.Duration) *MultiSecretsProvider {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	metrics := &MultiProviderMetrics{
		ProviderFailures:     make(map[string]int64),
		ProviderSuccesses:    make(map[string]int64),
		StrategyDistribution: make(map[string]int64),
	}

	return &MultiSecretsProvider{
		providers: providers,
		strategy:  strategy,
		timeout:   timeout,
		metrics:   metrics,
	}
}

// Type returns the provider type identifier
func (m *MultiSecretsProvider) Type() string {
	return "multi"
}

// GetSecret retrieves a secret using the configured strategy
func (m *MultiSecretsProvider) GetSecret(ctx context.Context, key string) (string, error) {
	start := time.Now()
	defer func() {
		m.updateMetrics(time.Since(start), 1)
	}()

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	switch m.strategy {
	case StrategyFailover:
		return m.getSecretFailover(timeoutCtx, key)
	case StrategyMerge:
		return m.getSecretMerge(timeoutCtx, key)
	case StrategyPriority:
		return m.getSecretPriority(timeoutCtx, key)
	default:
		return "", NewSecretError("INVALID_STRATEGY", "unsupported multi-provider strategy", "multi", key, nil)
	}
}

// getSecretFailover tries providers in order until one succeeds
func (m *MultiSecretsProvider) getSecretFailover(ctx context.Context, key string) (string, error) {
	var lastErr error

	for i, provider := range m.providers {
		value, err := provider.GetSecret(ctx, key)
		if err == nil {
			m.updateProviderMetrics(provider.Type(), true)
			m.updateStrategyMetrics("failover_success", i)
			return value, nil
		}

		m.updateProviderMetrics(provider.Type(), false)
		lastErr = err

		// Don't try other providers for certain errors
		if isNonRetryableError(err) {
			break
		}
	}

	m.updateStrategyMetrics("failover_failed", len(m.providers))
	if lastErr != nil {
		return "", lastErr
	}
	return "", NewSecretError("SECRET_NOT_FOUND", "secret not found in any provider", "multi", key, nil)
}

// getSecretMerge gets secret from first available provider (used for merge strategy)
func (m *MultiSecretsProvider) getSecretMerge(ctx context.Context, key string) (string, error) {
	// For single secret retrieval, merge strategy acts like failover
	// The real benefit of merge is in batch operations
	return m.getSecretFailover(ctx, key)
}

// getSecretPriority uses first provider, falls back to others only if not found
func (m *MultiSecretsProvider) getSecretPriority(ctx context.Context, key string) (string, error) {
	if len(m.providers) == 0 {
		return "", NewSecretError("NO_PROVIDERS", "no providers configured", "multi", key, nil)
	}

	// Try primary provider first
	primary := m.providers[0]
	value, err := primary.GetSecret(ctx, key)
	if err == nil {
		m.updateProviderMetrics(primary.Type(), true)
		m.updateStrategyMetrics("priority_primary", 0)
		return value, nil
	}

	m.updateProviderMetrics(primary.Type(), false)

	// Only try backup providers if secret not found in primary
	if isSecretNotFoundError(err) {
		for i := 1; i < len(m.providers); i++ {
			provider := m.providers[i]
			value, err := provider.GetSecret(ctx, key)
			if err == nil {
				m.updateProviderMetrics(provider.Type(), true)
				m.updateStrategyMetrics("priority_backup", i)
				return value, nil
			}
			m.updateProviderMetrics(provider.Type(), false)
		}
	}

	m.updateStrategyMetrics("priority_failed", len(m.providers))
	return "", err
}

// GetSecrets retrieves multiple secrets using the configured strategy
func (m *MultiSecretsProvider) GetSecrets(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	start := time.Now()
	defer func() {
		m.updateMetrics(time.Since(start), int64(len(keys)))
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	switch m.strategy {
	case StrategyFailover:
		return m.getSecretsFailover(timeoutCtx, keys)
	case StrategyMerge:
		return m.getSecretsMerge(timeoutCtx, keys)
	case StrategyPriority:
		return m.getSecretsPriority(timeoutCtx, keys)
	default:
		return nil, NewSecretError("INVALID_STRATEGY", "unsupported multi-provider strategy", "multi", "", nil)
	}
}

// getSecretsFailover tries providers in order for all keys
func (m *MultiSecretsProvider) getSecretsFailover(ctx context.Context, keys []string) (map[string]string, error) {
	for _, provider := range m.providers {
		results, err := provider.GetSecrets(ctx, keys)
		if err == nil && len(results) > 0 {
			m.updateProviderMetrics(provider.Type(), true)
			return results, nil
		}
		m.updateProviderMetrics(provider.Type(), false)
	}

	return map[string]string{}, NewSecretError("BATCH_FAILED", "batch secret retrieval failed in all providers", "multi", "", nil)
}

// getSecretsMerge combines results from all providers
func (m *MultiSecretsProvider) getSecretsMerge(ctx context.Context, keys []string) (map[string]string, error) {
	results := make(map[string]string)
	remainingKeys := make([]string, len(keys))
	copy(remainingKeys, keys)

	for _, provider := range m.providers {
		if len(remainingKeys) == 0 {
			break
		}

		providerResults, err := provider.GetSecrets(ctx, remainingKeys)
		if err != nil {
			m.updateProviderMetrics(provider.Type(), false)
			continue
		}

		m.updateProviderMetrics(provider.Type(), true)

		// Add new results and update remaining keys
		var newRemainingKeys []string
		for _, key := range remainingKeys {
			if value, found := providerResults[key]; found {
				results[key] = value
			} else {
				newRemainingKeys = append(newRemainingKeys, key)
			}
		}
		remainingKeys = newRemainingKeys
	}

	return results, nil
}

// getSecretsPriority uses primary provider, falls back for missing secrets
func (m *MultiSecretsProvider) getSecretsPriority(ctx context.Context, keys []string) (map[string]string, error) {
	if len(m.providers) == 0 {
		return map[string]string{}, NewSecretError("NO_PROVIDERS", "no providers configured", "multi", "", nil)
	}

	// Try primary provider first
	primary := m.providers[0]
	results, err := primary.GetSecrets(ctx, keys)
	if err != nil {
		m.updateProviderMetrics(primary.Type(), false)
		results = make(map[string]string)
	} else {
		m.updateProviderMetrics(primary.Type(), true)
	}

	// Find missing keys
	var missingKeys []string
	for _, key := range keys {
		if _, found := results[key]; !found {
			missingKeys = append(missingKeys, key)
		}
	}

	// Try backup providers for missing keys
	if len(missingKeys) > 0 && len(m.providers) > 1 {
		for i := 1; i < len(m.providers); i++ {
			if len(missingKeys) == 0 {
				break
			}

			provider := m.providers[i]
			backupResults, err := provider.GetSecrets(ctx, missingKeys)
			if err != nil {
				m.updateProviderMetrics(provider.Type(), false)
				continue
			}

			m.updateProviderMetrics(provider.Type(), true)

			// Add backup results and update missing keys
			var stillMissingKeys []string
			for _, key := range missingKeys {
				if value, found := backupResults[key]; found {
					results[key] = value
				} else {
					stillMissingKeys = append(stillMissingKeys, key)
				}
			}
			missingKeys = stillMissingKeys
		}
	}

	return results, nil
}

// GetSecretWithMetadata retrieves a secret with metadata from the first available provider
func (m *MultiSecretsProvider) GetSecretWithMetadata(ctx context.Context, key string) (*SecretValue, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	var lastErr error

	for _, provider := range m.providers {
		value, err := provider.GetSecretWithMetadata(timeoutCtx, key)
		if err == nil {
			// Add provider information to metadata
			if value.Metadata == nil {
				value.Metadata = make(map[string]string)
			}
			value.Metadata["provider"] = provider.Type()
			value.Metadata["multi_strategy"] = string(m.strategy)
			return value, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, NewSecretError("SECRET_NOT_FOUND", "secret not found in any provider", "multi", key, nil)
}

// ListSecrets returns secrets from all providers based on strategy
func (m *MultiSecretsProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	switch m.strategy {
	case StrategyMerge:
		return m.listSecretsMerge(timeoutCtx, prefix)
	default:
		// For failover and priority, use first available provider
		return m.listSecretsFirst(timeoutCtx, prefix)
	}
}

// listSecretsMerge combines secret lists from all providers
func (m *MultiSecretsProvider) listSecretsMerge(ctx context.Context, prefix string) ([]string, error) {
	secretSet := make(map[string]bool)
	var allSecrets []string

	for _, provider := range m.providers {
		secrets, err := provider.ListSecrets(ctx, prefix)
		if err != nil {
			continue // Skip providers that fail
		}

		for _, secret := range secrets {
			if !secretSet[secret] {
				secretSet[secret] = true
				allSecrets = append(allSecrets, secret)
			}
		}
	}

	return allSecrets, nil
}

// listSecretsFirst returns secrets from first available provider
func (m *MultiSecretsProvider) listSecretsFirst(ctx context.Context, prefix string) ([]string, error) {
	var lastErr error

	for _, provider := range m.providers {
		secrets, err := provider.ListSecrets(ctx, prefix)
		if err == nil {
			return secrets, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return []string{}, nil
}

// HealthCheck checks the health of all providers
func (m *MultiSecretsProvider) HealthCheck(ctx context.Context) error {
	var errors []string
	healthyCount := 0

	for _, provider := range m.providers {
		if err := provider.HealthCheck(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", provider.Type(), err))
		} else {
			healthyCount++
		}
	}

	// Require at least one healthy provider for most strategies
	requiredHealthy := 1
	if m.strategy == StrategyMerge {
		requiredHealthy = len(m.providers) / 2 + 1 // Majority for merge strategy
	}

	if healthyCount < requiredHealthy {
		return NewSecretError("HEALTH_CHECK_FAILED",
			fmt.Sprintf("insufficient healthy providers: %d/%d healthy, errors: %s",
				healthyCount, len(m.providers), strings.Join(errors, "; ")),
			"multi", "", nil)
	}

	return nil
}

// Close closes all providers
func (m *MultiSecretsProvider) Close() error {
	var errors []string

	for _, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", provider.Type(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing providers: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GetProviders returns the list of configured providers
func (m *MultiSecretsProvider) GetProviders() []SecretsProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	providers := make([]SecretsProvider, len(m.providers))
	copy(providers, m.providers)
	return providers
}

// GetStrategy returns the current strategy
func (m *MultiSecretsProvider) GetStrategy() MultiProviderStrategy {
	return m.strategy
}

// GetMetrics returns current performance metrics
func (m *MultiSecretsProvider) GetMetrics() MultiProviderMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()
	
	// Create a deep copy of the metrics
	metrics := MultiProviderMetrics{
		RequestsTotal:       m.metrics.RequestsTotal,
		SuccessfulRequests:  m.metrics.SuccessfulRequests,
		FailedRequests:      m.metrics.FailedRequests,
		AvgResponseTime:     m.metrics.AvgResponseTime,
		ProviderFailures:    make(map[string]int64),
		ProviderSuccesses:   make(map[string]int64),
		StrategyDistribution: make(map[string]int64),
	}
	
	for k, v := range m.metrics.ProviderFailures {
		metrics.ProviderFailures[k] = v
	}
	for k, v := range m.metrics.ProviderSuccesses {
		metrics.ProviderSuccesses[k] = v
	}
	for k, v := range m.metrics.StrategyDistribution {
		metrics.StrategyDistribution[k] = v
	}
	
	return metrics
}

// updateMetrics updates performance metrics
func (m *MultiSecretsProvider) updateMetrics(duration time.Duration, requests int64) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	m.metrics.RequestsTotal += requests

	if duration > 0 {
		if m.metrics.AvgResponseTime == 0 {
			m.metrics.AvgResponseTime = duration
		} else {
			m.metrics.AvgResponseTime = time.Duration(
				0.9*float64(m.metrics.AvgResponseTime) + 0.1*float64(duration),
			)
		}
	}
}

// updateProviderMetrics updates per-provider metrics
func (m *MultiSecretsProvider) updateProviderMetrics(providerType string, success bool) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	if success {
		m.metrics.ProviderSuccesses[providerType]++
		m.metrics.SuccessfulRequests++
	} else {
		m.metrics.ProviderFailures[providerType]++
		m.metrics.FailedRequests++
	}
}

// updateStrategyMetrics updates strategy-specific metrics
func (m *MultiSecretsProvider) updateStrategyMetrics(strategyEvent string, providerIndex int) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	key := fmt.Sprintf("%s_provider_%d", strategyEvent, providerIndex)
	m.metrics.StrategyDistribution[key]++
}

// isNonRetryableError checks if an error should stop failover attempts
func isNonRetryableError(err error) bool {
	if secretErr, ok := err.(*SecretError); ok {
		switch secretErr.Code {
		case "ACCESS_DENIED", "INVALID_CONFIG":
			return true
		}
	}
	return false
}

// isSecretNotFoundError checks if an error indicates the secret doesn't exist
func isSecretNotFoundError(err error) bool {
	if secretErr, ok := err.(*SecretError); ok {
		return secretErr.Code == "SECRET_NOT_FOUND"
	}
	return false
}

// AddProvider adds a new provider to the multi-provider
func (m *MultiSecretsProvider) AddProvider(provider SecretsProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, provider)
}

// RemoveProvider removes a provider by type
func (m *MultiSecretsProvider) RemoveProvider(providerType string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, provider := range m.providers {
		if provider.Type() == providerType {
			// Close the provider before removing it
			provider.Close()
			
			// Remove from slice
			m.providers = append(m.providers[:i], m.providers[i+1:]...)
			return true
		}
	}
	return false
}

// SetStrategy changes the multi-provider strategy
func (m *MultiSecretsProvider) SetStrategy(strategy MultiProviderStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strategy = strategy
}