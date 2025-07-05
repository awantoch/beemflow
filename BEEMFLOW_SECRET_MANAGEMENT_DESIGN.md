# BeemFlow Production-Ready Secret Management Design

## Executive Summary

This document outlines a comprehensive production-ready secret management system for BeemFlow that maintains the existing clean interface while adding support for industry-standard secret storage backends including AWS Secrets Manager, HashiCorp Vault, Azure Key Vault, and more.

## Current State Analysis

### Existing Architecture
- **Interface**: Simple `SecretsProvider` interface with `GetSecret(key string) (string, error)`
- **Implementation**: Environment variables only via `.env` file loading
- **Usage**: `{{ secrets.SECRET_NAME }}` template syntax
- **Configuration**: Placeholder `SecretsConfig` struct with `Driver`, `Region`, `Prefix` fields

### Strengths to Preserve
- Clean adapter pattern throughout codebase
- Configuration-driven design with JSON schema validation
- Template-based secret access
- Pluggable architecture

### Gaps to Address
- No production secret backends
- Limited secret metadata support
- No secret rotation capabilities
- No caching or performance optimization
- No multi-environment support

## Design Principles

1. **Backward Compatibility**: Existing `{{ secrets.KEY }}` syntax must continue working
2. **Zero Configuration**: Default to environment variables for development
3. **Production Ready**: Support enterprise-grade secret stores out of the box
4. **Performance**: Intelligent caching and batching
5. **Security**: Encryption at rest, in transit, and secure credential handling
6. **Observability**: Comprehensive logging and metrics
7. **Multi-Environment**: Support for dev/staging/prod configurations

## Enhanced Interface Design

### Core Interfaces

```go
// Enhanced SecretsProvider with advanced capabilities
type SecretsProvider interface {
    // Backward compatible single secret retrieval
    GetSecret(ctx context.Context, key string) (string, error)
    
    // Batch operations for performance
    GetSecrets(ctx context.Context, keys []string) (map[string]string, error)
    
    // Advanced operations
    GetSecretWithMetadata(ctx context.Context, key string) (*SecretValue, error)
    ListSecrets(ctx context.Context, prefix string) ([]string, error)
    
    // Health and lifecycle
    HealthCheck(ctx context.Context) error
    Close() error
}

// Rich secret value with metadata
type SecretValue struct {
    Value     string    `json:"value"`
    Version   string    `json:"version,omitempty"`
    CreatedAt time.Time `json:"created_at,omitempty"`
    ExpiresAt *time.Time `json:"expires_at,omitempty"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}

// Configuration structure
type SecretsConfig struct {
    Driver  string `json:"driver"`  // env, aws-sm, vault, azure-kv, gcp-sm, 1password, etc.
    Region  string `json:"region,omitempty"`
    Prefix  string `json:"prefix,omitempty"`
    
    // Driver-specific configuration
    AWS    *AWSSecretsConfig    `json:"aws,omitempty"`
    Vault  *VaultConfig        `json:"vault,omitempty"`
    Azure  *AzureKeyVaultConfig `json:"azure,omitempty"`
    GCP    *GCPSecretsConfig   `json:"gcp,omitempty"`
    
    // Performance and behavior
    CacheConfig *CacheConfig `json:"cache,omitempty"`
    BatchSize   int         `json:"batch_size,omitempty"`
    Timeout     time.Duration `json:"timeout,omitempty"`
}
```

## Backend Implementations

### 1. Environment Variables (Default)
```go
type EnvSecretsProvider struct {
    prefix string
    cache  map[string]string
    mu     sync.RWMutex
}

// Configuration
{
  "secrets": {
    "driver": "env",
    "prefix": "BEEMFLOW_"
  }
}
```

### 2. AWS Secrets Manager
```go
type AWSSecretsProvider struct {
    client *secretsmanager.Client
    region string
    prefix string
    cache  *SecretCache
}

type AWSSecretsConfig struct {
    Region          string `json:"region"`
    AccessKeyID     string `json:"access_key_id,omitempty"`
    SecretAccessKey string `json:"secret_access_key,omitempty"`
    SessionToken    string `json:"session_token,omitempty"`
    RoleARN         string `json:"role_arn,omitempty"`
    Profile         string `json:"profile,omitempty"`
}

// Configuration
{
  "secrets": {
    "driver": "aws-sm",
    "region": "us-west-2",
    "prefix": "beemflow/",
    "aws": {
      "role_arn": "arn:aws:iam::123456789012:role/BeemFlowSecretsRole"
    }
  }
}
```

### 3. HashiCorp Vault
```go
type VaultSecretsProvider struct {
    client *vault.Client
    path   string
    cache  *SecretCache
}

type VaultConfig struct {
    Address    string `json:"address"`
    Token      string `json:"token,omitempty"`
    RoleID     string `json:"role_id,omitempty"`
    SecretID   string `json:"secret_id,omitempty"`
    Namespace  string `json:"namespace,omitempty"`
    AuthMethod string `json:"auth_method"` // token, approle, kubernetes, etc.
    Path       string `json:"path"`        // secret mount path
}

// Configuration
{
  "secrets": {
    "driver": "vault",
    "vault": {
      "address": "https://vault.company.com:8200",
      "auth_method": "approle",
      "role_id": "${VAULT_ROLE_ID}",
      "secret_id": "${VAULT_SECRET_ID}",
      "path": "secret/beemflow"
    }
  }
}
```

### 4. Azure Key Vault
```go
type AzureKeyVaultProvider struct {
    client *azkeyvault.Client
    vaultURL string
    cache    *SecretCache
}

type AzureKeyVaultConfig struct {
    VaultURL     string `json:"vault_url"`
    TenantID     string `json:"tenant_id,omitempty"`
    ClientID     string `json:"client_id,omitempty"`
    ClientSecret string `json:"client_secret,omitempty"`
    AuthMethod   string `json:"auth_method"` // msi, service_principal, cli
}

// Configuration
{
  "secrets": {
    "driver": "azure-kv",
    "azure": {
      "vault_url": "https://beemflow-secrets.vault.azure.net/",
      "auth_method": "msi"
    }
  }
}
```

### 5. Google Cloud Secret Manager
```go
type GCPSecretsProvider struct {
    client *secretmanager.Client
    projectID string
    cache     *SecretCache
}

type GCPSecretsConfig struct {
    ProjectID              string `json:"project_id"`
    ServiceAccountKey      string `json:"service_account_key,omitempty"`
    ServiceAccountKeyFile  string `json:"service_account_key_file,omitempty"`
}

// Configuration
{
  "secrets": {
    "driver": "gcp-sm",
    "gcp": {
      "project_id": "my-project-123"
    }
  }
}
```

### 6. 1Password Connect
```go
type OnePasswordProvider struct {
    client *onepassword.Client
    vault  string
    cache  *SecretCache
}

// Configuration
{
  "secrets": {
    "driver": "1password",
    "onepassword": {
      "connect_host": "https://1password.company.com",
      "connect_token": "${OP_CONNECT_TOKEN}",
      "vault": "BeemFlow Secrets"
    }
  }
}
```

### 7. Multi-Provider (Fallback Chain)
```go
type MultiSecretsProvider struct {
    providers []SecretsProvider
    strategy  string // "failover", "merge", "priority"
}

// Configuration
{
  "secrets": {
    "driver": "multi",
    "multi": {
      "strategy": "failover",
      "providers": [
        {
          "driver": "vault",
          "vault": { /* vault config */ }
        },
        {
          "driver": "env",
          "prefix": "FALLBACK_"
        }
      ]
    }
  }
}
```

## Caching and Performance

### Intelligent Caching System
```go
type SecretCache struct {
    store    map[string]*CachedSecret
    mu       sync.RWMutex
    ttl      time.Duration
    maxSize  int
    metrics  *CacheMetrics
}

type CachedSecret struct {
    Value     string
    ExpiresAt time.Time
    Version   string
    Hits      int64
}

type CacheConfig struct {
    Enabled    bool          `json:"enabled"`
    TTL        time.Duration `json:"ttl"`
    MaxSize    int           `json:"max_size"`
    Strategy   string        `json:"strategy"` // lru, ttl, write-through
}
```

### Batch Operations
```go
// Automatically batch secret requests for performance
func (e *Engine) collectSecretsWithBatching(event map[string]any, secretRefs []string) SecretsData {
    if len(secretRefs) > 1 {
        // Use batch API for multiple secrets
        secrets, err := e.secretsProvider.GetSecrets(ctx, secretRefs)
        if err != nil {
            // Fallback to individual requests
            return e.collectSecretsIndividually(event, secretRefs)
        }
        return secrets
    }
    return e.collectSecretsIndividually(event, secretRefs)
}
```

## Security Enhancements

### Encryption and Transport Security
- All secret providers use TLS 1.3+ for transport
- Local cache encryption using AES-256-GCM
- Memory protection for secret values
- Automatic credential rotation support

### Access Control Integration
```go
type SecretAccessPolicy struct {
    FlowPatterns    []string `json:"flow_patterns"`
    SecretPatterns  []string `json:"secret_patterns"`
    AllowedActions  []string `json:"allowed_actions"`
    RequiredRoles   []string `json:"required_roles"`
}

// Configuration
{
  "secrets": {
    "driver": "vault",
    "access_policies": [
      {
        "flow_patterns": ["finance/*", "accounting/*"],
        "secret_patterns": ["stripe/*", "quickbooks/*"],
        "required_roles": ["finance-admin"]
      }
    ]
  }
}
```

## Template Engine Integration

### Enhanced Template Functions
```yaml
# Existing syntax (backward compatible)
token: "{{ secrets.SLACK_TOKEN }}"

# Enhanced syntax with metadata
created_at: "{{ secrets.SLACK_TOKEN.created_at }}"
version: "{{ secrets.SLACK_TOKEN.version }}"

# Conditional secret access
api_url: "{{ secrets.API_URL | default 'https://api.example.com' }}"

# Secret transformation
password_hash: "{{ secrets.DB_PASSWORD | hash 'sha256' }}"

# Multi-provider fallback in templates
token: "{{ secrets.PROD_TOKEN | fallback secrets.DEV_TOKEN }}"
```

### Secret Discovery and Validation
```go
// Automatically discover secret references in flow templates
func (p *Parser) ExtractSecretReferences(flow *model.Flow) []string {
    refs := []string{}
    // Parse all template strings and extract {{ secrets.* }} references
    // This enables pre-flight validation and batching
    return refs
}

// Validate all secrets exist before flow execution
func (e *Engine) ValidateSecrets(ctx context.Context, flow *model.Flow) error {
    refs := e.parser.ExtractSecretReferences(flow)
    for _, ref := range refs {
        if _, err := e.secretsProvider.GetSecret(ctx, ref); err != nil {
            return fmt.Errorf("secret validation failed for %s: %w", ref, err)
        }
    }
    return nil
}
```

## Deployment Configurations

### Development Environment
```json
{
  "secrets": {
    "driver": "env",
    "cache": {
      "enabled": false
    }
  }
}
```

### Staging Environment
```json
{
  "secrets": {
    "driver": "multi",
    "multi": {
      "strategy": "failover",
      "providers": [
        {
          "driver": "vault",
          "vault": {
            "address": "https://vault-staging.company.com:8200",
            "auth_method": "kubernetes",
            "path": "secret/beemflow-staging"
          }
        },
        {
          "driver": "env",
          "prefix": "STAGING_"
        }
      ]
    },
    "cache": {
      "enabled": true,
      "ttl": "5m",
      "max_size": 1000
    }
  }
}
```

### Production Environment
```json
{
  "secrets": {
    "driver": "aws-sm",
    "region": "us-west-2",
    "prefix": "beemflow/prod/",
    "aws": {
      "role_arn": "arn:aws:iam::123456789012:role/BeemFlowProdSecretsRole"
    },
    "cache": {
      "enabled": true,
      "ttl": "1h",
      "max_size": 10000,
      "strategy": "write-through"
    },
    "timeout": "30s",
    "batch_size": 50
  }
}
```

### Kubernetes Deployment
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: beemflow-config
data:
  flow.config.json: |
    {
      "secrets": {
        "driver": "multi",
        "multi": {
          "strategy": "failover",
          "providers": [
            {
              "driver": "vault",
              "vault": {
                "address": "https://vault.vault.svc.cluster.local:8200",
                "auth_method": "kubernetes",
                "path": "secret/beemflow"
              }
            },
            {
              "driver": "kubernetes",
              "kubernetes": {
                "namespace": "beemflow"
              }
            }
          ]
        }
      }
    }
```

### Serverless Deployment (Vercel/Lambda)
```json
{
  "secrets": {
    "driver": "aws-sm",
    "region": "us-east-1",
    "cache": {
      "enabled": true,
      "ttl": "15m",
      "max_size": 100
    },
    "timeout": "5s"
  }
}
```

## Migration Strategy

### Phase 1: Foundation (Week 1-2)
1. Implement enhanced `SecretsProvider` interface
2. Refactor existing environment variable provider
3. Add caching infrastructure
4. Update template engine integration

### Phase 2: Cloud Providers (Week 3-4)
1. Implement AWS Secrets Manager provider
2. Implement HashiCorp Vault provider
3. Add configuration validation and health checks
4. Comprehensive testing with real backends

### Phase 3: Enterprise Features (Week 5-6)
1. Implement Azure Key Vault and GCP Secret Manager
2. Add multi-provider support with fallback chains
3. Implement access control and audit logging
4. Performance optimization and metrics

### Phase 4: Advanced Features (Week 7-8)
1. Add 1Password and other password manager integrations
2. Implement secret rotation notifications
3. Add template function enhancements
4. Complete documentation and examples

## Security Considerations

### Credential Management
- Never log secret values in plaintext
- Automatic credential rotation support
- Secure credential injection for CI/CD
- Memory-safe secret handling

### Audit and Compliance
```go
type SecretAuditLog struct {
    FlowID    string    `json:"flow_id"`
    SecretKey string    `json:"secret_key"`
    Action    string    `json:"action"`
    UserID    string    `json:"user_id,omitempty"`
    Timestamp time.Time `json:"timestamp"`
    Source    string    `json:"source"`
}

// Audit all secret access
func (p *AuditingSecretsProvider) GetSecret(ctx context.Context, key string) (string, error) {
    value, err := p.provider.GetSecret(ctx, key)
    p.auditLogger.Log(SecretAuditLog{
        FlowID:    getFlowIDFromContext(ctx),
        SecretKey: key,
        Action:    "read",
        Timestamp: time.Now(),
        Source:    p.provider.Type(),
    })
    return value, err
}
```

### Network Security
- TLS 1.3+ for all external communications
- Certificate pinning for critical services
- Network policy enforcement in Kubernetes
- VPC/private network deployment support

## Monitoring and Observability

### Metrics
```go
type SecretsMetrics struct {
    RequestsTotal      prometheus.Counter
    RequestDuration    prometheus.Histogram
    CacheHitRate       prometheus.Gauge
    ActiveConnections  prometheus.Gauge
    ErrorsTotal        prometheus.Counter
    SecretsCount       prometheus.Gauge
}
```

### Health Checks
```go
func (p *SecretsProvider) HealthCheck(ctx context.Context) error {
    // Test connectivity to secret backend
    // Validate authentication/authorization
    // Check cache status
    // Verify critical secrets are accessible
    return nil
}
```

### Logging
```go
// Structured logging for all secret operations
utils.InfoCtx(ctx, "Secret retrieved successfully",
    "provider", provider.Type(),
    "key", secretKey,
    "cache_hit", cacheHit,
    "duration_ms", duration.Milliseconds(),
)
```

## Example Configurations

### Small Team (GitHub Actions + 1Password)
```json
{
  "secrets": {
    "driver": "1password",
    "onepassword": {
      "connect_host": "${OP_CONNECT_HOST}",
      "connect_token": "${OP_CONNECT_TOKEN}",
      "vault": "Engineering Secrets"
    },
    "cache": {
      "enabled": true,
      "ttl": "10m"
    }
  }
}
```

### Enterprise (Multi-Cloud with Vault)
```json
{
  "secrets": {
    "driver": "multi",
    "multi": {
      "strategy": "merge",
      "providers": [
        {
          "driver": "vault",
          "vault": {
            "address": "https://vault.company.com:8200",
            "auth_method": "kubernetes",
            "path": "secret/beemflow"
          }
        },
        {
          "driver": "aws-sm",
          "region": "us-west-2",
          "prefix": "beemflow/aws/",
          "aws": {
            "role_arn": "arn:aws:iam::123456789012:role/BeemFlowSecretsRole"
          }
        },
        {
          "driver": "azure-kv",
          "azure": {
            "vault_url": "https://company-secrets.vault.azure.net/",
            "auth_method": "msi"
          }
        }
      ]
    },
    "cache": {
      "enabled": true,
      "ttl": "1h",
      "max_size": 10000
    }
  }
}
```

## Implementation Files

### Core Implementation
- `secrets/provider.go` - Enhanced interfaces and base types
- `secrets/env.go` - Environment variable provider (enhanced)
- `secrets/cache.go` - Caching infrastructure
- `secrets/multi.go` - Multi-provider implementation
- `secrets/factory.go` - Provider factory and configuration

### Cloud Providers
- `secrets/aws.go` - AWS Secrets Manager provider
- `secrets/vault.go` - HashiCorp Vault provider
- `secrets/azure.go` - Azure Key Vault provider
- `secrets/gcp.go` - Google Cloud Secret Manager provider

### Integration
- `secrets/onepassword.go` - 1Password Connect provider
- `secrets/kubernetes.go` - Kubernetes secrets provider
- `secrets/file.go` - File-based secrets provider

### Infrastructure
- `secrets/audit.go` - Audit logging
- `secrets/metrics.go` - Prometheus metrics
- `secrets/health.go` - Health check endpoints

## Configuration Schema Updates

```json
{
  "secrets": {
    "type": "object",
    "properties": {
      "driver": {
        "type": "string",
        "enum": ["env", "aws-sm", "vault", "azure-kv", "gcp-sm", "1password", "kubernetes", "file", "multi"]
      },
      "region": { "type": "string" },
      "prefix": { "type": "string" },
      "timeout": { "type": "string" },
      "batch_size": { "type": "integer", "minimum": 1, "maximum": 1000 },
      "aws": {
        "type": "object",
        "properties": {
          "region": { "type": "string" },
          "access_key_id": { "type": "string" },
          "secret_access_key": { "type": "string" },
          "session_token": { "type": "string" },
          "role_arn": { "type": "string" },
          "profile": { "type": "string" }
        }
      },
      "vault": {
        "type": "object",
        "properties": {
          "address": { "type": "string", "format": "uri" },
          "token": { "type": "string" },
          "role_id": { "type": "string" },
          "secret_id": { "type": "string" },
          "namespace": { "type": "string" },
          "auth_method": { "type": "string", "enum": ["token", "approle", "kubernetes", "aws", "azure"] },
          "path": { "type": "string" }
        },
        "required": ["address", "path"]
      },
      "cache": {
        "type": "object",
        "properties": {
          "enabled": { "type": "boolean" },
          "ttl": { "type": "string" },
          "max_size": { "type": "integer" },
          "strategy": { "type": "string", "enum": ["lru", "ttl", "write-through"] }
        }
      }
    }
  }
}
```

## Conclusion

This design provides a comprehensive, production-ready secret management system for BeemFlow that:

1. **Maintains Backward Compatibility**: Existing flows continue working unchanged
2. **Supports Enterprise Requirements**: Integration with major cloud secret stores
3. **Optimizes Performance**: Intelligent caching and batching
4. **Ensures Security**: Encryption, audit logging, and access controls  
5. **Enables Scalability**: Multi-provider support and fallback strategies
6. **Provides Observability**: Comprehensive metrics and health checks
7. **Facilitates Deployment**: Configuration examples for all deployment modes

The modular design allows teams to start simple with environment variables and progressively adopt more sophisticated secret management as their needs grow, while the clean interfaces ensure consistent behavior across all deployment scenarios.