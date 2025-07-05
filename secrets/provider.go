package secrets

import (
	"context"
	"time"
)

// SecretsProvider defines the interface for all secret storage backends
// This interface maintains backward compatibility while adding advanced features
type SecretsProvider interface {
	// Backward compatible single secret retrieval
	GetSecret(ctx context.Context, key string) (string, error)
	
	// Batch operations for performance optimization
	GetSecrets(ctx context.Context, keys []string) (map[string]string, error)
	
	// Advanced operations with metadata support
	GetSecretWithMetadata(ctx context.Context, key string) (*SecretValue, error)
	ListSecrets(ctx context.Context, prefix string) ([]string, error)
	
	// Health and lifecycle management
	HealthCheck(ctx context.Context) error
	Close() error
	
	// Provider identification
	Type() string
}

// SecretValue represents a secret with rich metadata
type SecretValue struct {
	Value     string            `json:"value"`
	Version   string            `json:"version,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SecretReference represents a parsed secret reference from templates
type SecretReference struct {
	Key      string `json:"key"`
	Property string `json:"property,omitempty"` // For accessing metadata like .created_at
}

// ProviderConfig contains base configuration for all providers
type ProviderConfig struct {
	Driver      string        `json:"driver"`
	Region      string        `json:"region,omitempty"`
	Prefix      string        `json:"prefix,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	BatchSize   int           `json:"batch_size,omitempty"`
	CacheConfig *CacheConfig  `json:"cache,omitempty"`
}

// CacheConfig defines caching behavior for secret providers
type CacheConfig struct {
	Enabled  bool          `json:"enabled"`
	TTL      time.Duration `json:"ttl"`
	MaxSize  int           `json:"max_size"`
	Strategy string        `json:"strategy"` // lru, ttl, write-through
}

// ProviderFactory creates secret providers based on configuration
type ProviderFactory interface {
	CreateProvider(config *ProviderConfig) (SecretsProvider, error)
	SupportedDrivers() []string
}

// SecretCache provides caching capabilities for secret providers
type SecretCache interface {
	Get(key string) (*SecretValue, bool)
	Set(key string, value *SecretValue, ttl time.Duration)
	Delete(key string)
	Clear()
	Stats() CacheStats
}

// CacheStats provides metrics about cache performance
type CacheStats struct {
	Hits        int64 `json:"hits"`
	Misses      int64 `json:"misses"`
	Size        int   `json:"size"`
	MaxSize     int   `json:"max_size"`
	HitRate     float64 `json:"hit_rate"`
	Evictions   int64 `json:"evictions"`
}

// SecretAuditLog represents an audit entry for secret access
type SecretAuditLog struct {
	FlowID    string    `json:"flow_id"`
	RunID     string    `json:"run_id,omitempty"`
	SecretKey string    `json:"secret_key"`
	Action    string    `json:"action"` // read, list, batch_read
	UserID    string    `json:"user_id,omitempty"`
	Source    string    `json:"source"` // provider type
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	Log(entry SecretAuditLog) error
	Query(filters AuditFilters) ([]SecretAuditLog, error)
}

// AuditFilters defines filters for audit log queries
type AuditFilters struct {
	FlowID     string    `json:"flow_id,omitempty"`
	SecretKey  string    `json:"secret_key,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	Actions    []string  `json:"actions,omitempty"`
	Success    *bool     `json:"success,omitempty"`
}

// SecretAccessPolicy defines access control rules for secrets
type SecretAccessPolicy struct {
	FlowPatterns   []string `json:"flow_patterns"`
	SecretPatterns []string `json:"secret_patterns"`
	AllowedActions []string `json:"allowed_actions"`
	RequiredRoles  []string `json:"required_roles"`
	DenyPatterns   []string `json:"deny_patterns,omitempty"`
}

// AccessController evaluates secret access policies
type AccessController interface {
	CheckAccess(ctx context.Context, request AccessRequest) error
}

// AccessRequest represents a request to access secrets
type AccessRequest struct {
	FlowName   string   `json:"flow_name"`
	SecretKeys []string `json:"secret_keys"`
	Action     string   `json:"action"`
	UserID     string   `json:"user_id,omitempty"`
	UserRoles  []string `json:"user_roles,omitempty"`
}

// ProviderHealthStatus represents the health status of a secret provider
type ProviderHealthStatus struct {
	Provider    string        `json:"provider"`
	Healthy     bool          `json:"healthy"`
	Error       string        `json:"error,omitempty"`
	LastCheck   time.Time     `json:"last_check"`
	ResponseTime time.Duration `json:"response_time"`
	Version     string        `json:"version,omitempty"`
}

// MultiProviderStrategy defines how multiple providers should be used
type MultiProviderStrategy string

const (
	StrategyFailover MultiProviderStrategy = "failover" // Try providers in order until one succeeds
	StrategyMerge    MultiProviderStrategy = "merge"    // Merge results from all providers
	StrategyPriority MultiProviderStrategy = "priority" // Use first provider, others as backup
)

// Common errors that providers should return
var (
	ErrSecretNotFound    = &SecretError{Code: "SECRET_NOT_FOUND", Message: "secret not found"}
	ErrAccessDenied     = &SecretError{Code: "ACCESS_DENIED", Message: "access denied"}
	ErrProviderDown     = &SecretError{Code: "PROVIDER_DOWN", Message: "secret provider unavailable"}
	ErrInvalidConfig    = &SecretError{Code: "INVALID_CONFIG", Message: "invalid provider configuration"}
	ErrQuotaExceeded    = &SecretError{Code: "QUOTA_EXCEEDED", Message: "secret provider quota exceeded"}
	ErrSecretExpired    = &SecretError{Code: "SECRET_EXPIRED", Message: "secret has expired"}
)

// SecretError provides structured error information
type SecretError struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Provider string `json:"provider,omitempty"`
	Key      string `json:"key,omitempty"`
	Cause    error  `json:"-"`
}

func (e *SecretError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *SecretError) Unwrap() error {
	return e.Cause
}

// NewSecretError creates a new SecretError with the given details
func NewSecretError(code, message, provider, key string, cause error) *SecretError {
	return &SecretError{
		Code:     code,
		Message:  message,
		Provider: provider,
		Key:      key,
		Cause:    cause,
	}
}