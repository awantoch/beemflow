package engine

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/constants"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/secrets"
	"github.com/awantoch/beemflow/utils"
)

// SecretsManager manages secret providers and integrates with the BeemFlow engine
type SecretsManager struct {
	provider        secrets.SecretsProvider
	templateRegex   *regexp.Regexp
	cache           map[string]string
	cacheMu         sync.RWMutex
	preflightCache  map[string][]string // Cache of secret references per flow
	preflightCacheMu sync.RWMutex
}

// NewSecretsManager creates a new secrets manager with the given provider
func NewSecretsManager(provider secrets.SecretsProvider) *SecretsManager {
	// Regex to match {{ secrets.KEY }} and {{ secrets.KEY.property }} patterns
	templateRegex := regexp.MustCompile(`\{\{\s*secrets\.([^}\s.]+)(?:\.([^}\s]+))?\s*\}\}`)

	return &SecretsManager{
		provider:       provider,
		templateRegex:  templateRegex,
		cache:          make(map[string]string),
		preflightCache: make(map[string][]string),
	}
}

// UpdateEngineWithSecrets updates the BeemFlow engine to use production-ready secret management
func UpdateEngineWithSecrets(ctx context.Context, engine *Engine, config *config.Config) error {
	// Create secrets provider factory
	factory := secrets.NewProviderFactory()

	// Create the secrets provider from configuration
	provider, err := factory.CreateSecretsProvider(ctx, config.Secrets)
	if err != nil {
		return utils.Errorf("failed to create secrets provider: %w", err)
	}

	// Perform health check
	if err := provider.HealthCheck(ctx); err != nil {
		return utils.Errorf("secrets provider health check failed: %w", err)
	}

	// Create secrets manager
	secretsManager := NewSecretsManager(provider)

	// Replace the engine's secret collection method
	engine.secretsManager = secretsManager

	utils.Info("Successfully initialized secrets provider: %s", provider.Type())
	return nil
}

// collectSecretsAdvanced replaces the original collectSecrets method with advanced functionality
func (e *Engine) collectSecretsAdvanced(ctx context.Context, flow *model.Flow, event map[string]any) (SecretsData, error) {
	if e.secretsManager == nil {
		// Fallback to original behavior if secrets manager not initialized
		return e.collectSecrets(event), nil
	}

	// Extract secret references from the flow
	secretRefs := e.secretsManager.ExtractSecretReferences(flow)
	if len(secretRefs) == 0 {
		return make(SecretsData), nil
	}

	// Check for any secrets passed in the event (for backward compatibility)
	secretsFromEvent := make(SecretsData)
	if eventSecrets, ok := utils.SafeMapAssert(event[constants.SecretsKey]); ok {
		for k, v := range eventSecrets {
			if str, ok := v.(string); ok {
				secretsFromEvent[k] = str
			}
		}
	}

	// Get secrets from the provider (batch operation for efficiency)
	secretsFromProvider, err := e.secretsManager.GetSecrets(ctx, secretRefs)
	if err != nil {
		utils.WarnCtx(ctx, "Failed to retrieve some secrets from provider: %v", "error", err)
		// Continue with partial results rather than failing completely
	}

	// Merge secrets (event secrets take precedence for backward compatibility)
	result := make(SecretsData)
	for _, ref := range secretRefs {
		if value, ok := secretsFromEvent[ref]; ok {
			result[ref] = value
		} else if value, ok := secretsFromProvider[ref]; ok {
			result[ref] = value
		} else {
			utils.WarnCtx(ctx, "Secret not found: %s", "secret", ref)
		}
	}

	// Add environment variables with $env prefix (backward compatibility)
	for k, v := range event {
		if strings.HasPrefix(k, constants.EnvVarPrefix) {
			envVar := strings.TrimPrefix(k, constants.EnvVarPrefix)
			if str, ok := v.(string); ok {
				result[envVar] = str
			}
		}
	}

	return result, nil
}

// ExtractSecretReferences extracts all secret references from a flow
func (sm *SecretsManager) ExtractSecretReferences(flow *model.Flow) []string {
	if flow == nil {
		return nil
	}

	// Check cache first
	flowKey := flow.Name + ":" + flow.Version
	sm.preflightCacheMu.RLock()
	if refs, exists := sm.preflightCache[flowKey]; exists {
		sm.preflightCacheMu.RUnlock()
		return refs
	}
	sm.preflightCacheMu.RUnlock()

	// Extract references from the flow
	refs := sm.extractFromFlow(flow)

	// Cache the results
	sm.preflightCacheMu.Lock()
	sm.preflightCache[flowKey] = refs
	sm.preflightCacheMu.Unlock()

	return refs
}

// extractFromFlow recursively extracts secret references from all parts of a flow
func (sm *SecretsManager) extractFromFlow(flow *model.Flow) []string {
	refSet := make(map[string]bool)

	// Extract from vars
	for _, v := range flow.Vars {
		sm.extractFromValue(v, refSet)
	}

	// Extract from steps
	for _, step := range flow.Steps {
		sm.extractFromStep(&step, refSet)
	}

	// Extract from catch blocks
	for _, step := range flow.Catch {
		sm.extractFromStep(&step, refSet)
	}

	// Convert set to slice
	refs := make([]string, 0, len(refSet))
	for ref := range refSet {
		refs = append(refs, ref)
	}

	return refs
}

// extractFromStep extracts secret references from a single step
func (sm *SecretsManager) extractFromStep(step *model.Step, refSet map[string]bool) {
	// Extract from step inputs
	for _, v := range step.With {
		sm.extractFromValue(v, refSet)
	}

	// Extract from conditional expressions
	if step.If != "" {
		sm.extractFromString(step.If, refSet)
	}

	// Extract from foreach expressions
	if step.Foreach != "" {
		sm.extractFromString(step.Foreach, refSet)
	}

	// Extract from nested steps (parallel execution)
	for _, nestedStep := range step.Steps {
		sm.extractFromStep(&nestedStep, refSet)
	}

	// Extract from do blocks (foreach)
	for _, doStep := range step.Do {
		sm.extractFromStep(&doStep, refSet)
	}
}

// extractFromValue recursively extracts secret references from any value
func (sm *SecretsManager) extractFromValue(value any, refSet map[string]bool) {
	switch v := value.(type) {
	case string:
		sm.extractFromString(v, refSet)
	case map[string]any:
		for _, val := range v {
			sm.extractFromValue(val, refSet)
		}
	case []any:
		for _, val := range v {
			sm.extractFromValue(val, refSet)
		}
	}
}

// extractFromString extracts secret references from a template string
func (sm *SecretsManager) extractFromString(text string, refSet map[string]bool) {
	matches := sm.templateRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 {
			secretKey := match[1]
			refSet[secretKey] = true
		}
	}
}

// GetSecrets retrieves multiple secrets using the configured provider
func (sm *SecretsManager) GetSecrets(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	// Check cache first
	cached := make(map[string]string)
	uncachedKeys := make([]string, 0, len(keys))

	sm.cacheMu.RLock()
	for _, key := range keys {
		if value, exists := sm.cache[key]; exists {
			cached[key] = value
		} else {
			uncachedKeys = append(uncachedKeys, key)
		}
	}
	sm.cacheMu.RUnlock()

	// If all keys are cached, return immediately
	if len(uncachedKeys) == 0 {
		return cached, nil
	}

	// Fetch uncached secrets from provider
	providerResults, err := sm.provider.GetSecrets(ctx, uncachedKeys)
	if err != nil {
		return cached, err
	}

	// Update cache and merge results
	sm.cacheMu.Lock()
	for key, value := range providerResults {
		sm.cache[key] = value
		cached[key] = value
	}
	sm.cacheMu.Unlock()

	return cached, nil
}

// GetSecret retrieves a single secret
func (sm *SecretsManager) GetSecret(ctx context.Context, key string) (string, error) {
	// Check cache first
	sm.cacheMu.RLock()
	if value, exists := sm.cache[key]; exists {
		sm.cacheMu.RUnlock()
		return value, nil
	}
	sm.cacheMu.RUnlock()

	// Fetch from provider
	value, err := sm.provider.GetSecret(ctx, key)
	if err != nil {
		return "", err
	}

	// Cache the result
	sm.cacheMu.Lock()
	sm.cache[key] = value
	sm.cacheMu.Unlock()

	return value, nil
}

// ValidateSecrets validates that all required secrets are available before flow execution
func (sm *SecretsManager) ValidateSecrets(ctx context.Context, flow *model.Flow) error {
	refs := sm.ExtractSecretReferences(flow)
	if len(refs) == 0 {
		return nil
	}

	// Attempt to retrieve all secrets
	secrets, err := sm.GetSecrets(ctx, refs)
	if err != nil {
		return utils.Errorf("secret validation failed: %w", err)
	}

	// Check for missing secrets
	var missingSecrets []string
	for _, ref := range refs {
		if _, exists := secrets[ref]; !exists {
			missingSecrets = append(missingSecrets, ref)
		}
	}

	if len(missingSecrets) > 0 {
		return utils.Errorf("required secrets not found: %v", missingSecrets)
	}

	return nil
}

// ClearCache clears the secrets cache
func (sm *SecretsManager) ClearCache() {
	sm.cacheMu.Lock()
	defer sm.cacheMu.Unlock()
	sm.cache = make(map[string]string)
}

// ClearPreflightCache clears the preflight cache
func (sm *SecretsManager) ClearPreflightCache() {
	sm.preflightCacheMu.Lock()
	defer sm.preflightCacheMu.Unlock()
	sm.preflightCache = make(map[string][]string)
}

// GetProvider returns the underlying secrets provider
func (sm *SecretsManager) GetProvider() secrets.SecretsProvider {
	return sm.provider
}

// HealthCheck performs a health check on the secrets provider
func (sm *SecretsManager) HealthCheck(ctx context.Context) error {
	if sm.provider == nil {
		return utils.Errorf("secrets provider not initialized")
	}
	return sm.provider.HealthCheck(ctx)
}

// Close cleans up the secrets manager
func (sm *SecretsManager) Close() error {
	if sm.provider != nil {
		return sm.provider.Close()
	}
	return nil
}

// Enhanced Engine methods

// Add secretsManager field to Engine struct (this would be added to the existing Engine struct)
// Note: This is shown for illustration - in practice you'd modify the existing Engine struct
type EnhancedEngine struct {
	*Engine // Embed the existing engine
	secretsManager *SecretsManager
}

// executeStepWithSecretsValidation executes a step with pre-flight secret validation
func (e *EnhancedEngine) executeStepWithSecretsValidation(ctx context.Context, step *model.Step, stepCtx *StepContext, stepID string) error {
	// Pre-validate secrets for this step if secrets manager is available
	if e.secretsManager != nil {
		// Create a mini-flow for just this step to validate its secrets
		miniFlow := &model.Flow{
			Name:  "step-" + stepID,
			Steps: []model.Step{*step},
		}

		if err := e.secretsManager.ValidateSecrets(ctx, miniFlow); err != nil {
			return utils.Errorf("secret validation failed for step %s: %w", stepID, err)
		}
	}

	// Execute the step using the original method
	return e.Engine.executeStep(ctx, step, stepCtx, stepID)
}

// Enhanced template data preparation that includes metadata
func (e *EnhancedEngine) prepareEnhancedTemplateData(stepCtx *StepContext) map[string]any {
	data := e.prepareTemplateDataAsMap(stepCtx)

	// If we have a secrets manager, enhance the secrets data with metadata access
	if e.secretsManager != nil {
		// Create a proxy object that supports both value and metadata access
		secretsProxy := &SecretsTemplateProxy{
			manager: e.secretsManager,
			values:  stepCtx.Secrets,
		}
		data[constants.TemplateFieldSecrets] = secretsProxy
	}

	return data
}

// SecretsTemplateProxy provides template access to secrets with metadata support
type SecretsTemplateProxy struct {
	manager *SecretsManager
	values  SecretsData
}

// This would be used by the template engine to provide access to secret metadata
// Implementation would depend on the specific template engine used by BeemFlow

// Example configuration for different environments

// Development configuration (environment variables)
func GetDevelopmentSecretsConfig() *secrets.ExtendedSecretsConfig {
	return &secrets.ExtendedSecretsConfig{
		Driver: "env",
		Prefix: "BEEMFLOW_",
		CacheConfig: &secrets.CacheConfig{
			Enabled: false, // No caching in development
		},
	}
}

// Production configuration (AWS Secrets Manager with fallback)
func GetProductionSecretsConfig() *secrets.ExtendedSecretsConfig {
	return &secrets.ExtendedSecretsConfig{
		Driver: "multi",
		Multi: &secrets.MultiProviderConfig{
			Strategy: secrets.StrategyFailover,
			Providers: []secrets.ProviderConfig{
				{
					Driver: "aws-sm",
					Region: "us-west-2",
					Prefix: "beemflow/prod/",
					CacheConfig: &secrets.CacheConfig{
						Enabled: true,
						TTL:     "1h",
						MaxSize: 10000,
					},
				},
				{
					Driver: "env",
					Prefix: "BEEMFLOW_",
				},
			},
		},
		AuditConfig: &secrets.AuditConfig{
			Enabled: true,
			Driver:  "file",
			Path:    "/var/log/beemflow/secrets.audit",
		},
		AccessPolicies: []secrets.SecretAccessPolicy{
			{
				FlowPatterns:   []string{"finance/*", "accounting/*"},
				SecretPatterns: []string{"stripe/*", "quickbooks/*"},
				RequiredRoles:  []string{"finance-admin"},
			},
		},
	}
}

// Kubernetes configuration (Vault with Kubernetes secrets fallback)
func GetKubernetesSecretsConfig() *secrets.ExtendedSecretsConfig {
	return &secrets.ExtendedSecretsConfig{
		Driver: "multi",
		Multi: &secrets.MultiProviderConfig{
			Strategy: secrets.StrategyPriority,
			Providers: []secrets.ProviderConfig{
				{
					Driver: "vault",
					// Vault config would be here
				},
				{
					Driver: "kubernetes",
					// K8s config would be here
				},
			},
		},
		CacheConfig: &secrets.CacheConfig{
			Enabled: true,
			TTL:     "30m",
			MaxSize: 1000,
		},
	}
}