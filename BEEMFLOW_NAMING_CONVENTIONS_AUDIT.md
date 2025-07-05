# BeemFlow Naming Conventions & Architecture Audit

## Executive Summary

This document provides a comprehensive audit of naming conventions and architectural patterns across the BeemFlow codebase to ensure consistency and intellectual honesty. The analysis reveals several distinct patterns that should be codified and consistently applied.

## Core Architectural Patterns

### 1. Interface-Based Architecture

BeemFlow follows a clean interface-based architecture with pluggable implementations:

#### **Interface Naming Convention: `{Domain}` (noun)**
- `Adapter` - Tool execution interface
- `EventBus` - Event publishing/subscription interface  
- `BlobStore` - Binary data storage interface
- `Storage` - Persistent state storage interface
- `SecretsProvider` - Secret retrieval interface
- `MCPRegistry` - MCP server registry interface
- `Renderer` - Graph rendering interface

#### **Implementation Naming Convention: `{Technology/Type}{Domain}` (noun)**
- `CoreAdapter`, `HTTPAdapter`, `MCPAdapter`
- `WatermillEventBus` (technology-specific)
- `S3BlobStore`, `FilesystemBlobStore`
- `SqliteStorage`, `MemoryStorage`
- `EnvSecretsProvider`, `AWSSecretsProvider`
- `LocalRegistry`, `RemoteRegistry`, `SmitheryRegistry`, `DefaultRegistry`

### 2. Factory Pattern Usage

#### **Factory Function Naming: `New{Type}` or `New{Type}FromConfig`**
- `NewRegistry()` - Simple constructor
- `NewTemplater()` - Simple constructor
- `NewEventBusFromConfig()` - Configuration-driven constructor
- `NewSecretsProvider()` - Configuration-driven constructor
- `NewDefaultBlobStore()` - Configuration-driven with defaults
- `NewDefaultAdapterRegistry()` - Complex initialization with defaults

#### **Factory Type Naming: `{Domain}Factory`**
- `RegistryFactory` - Creates registry managers with consistent configuration

### 3. Registry vs Provider vs Adapter vs Store

**Current Usage Analysis:**

#### **Registry Pattern**
- **Purpose**: Collection/lookup of similar items
- **Examples**: `adapter.Registry`, `RegistryManager`, `LocalRegistry`
- **Characteristics**: 
  - Contains multiple items of the same type
  - Provides lookup/search functionality
  - Often hierarchical (local → remote → default)

#### **Provider Pattern**  
- **Purpose**: Abstraction for external services/resources
- **Examples**: `SecretsProvider`, `EnvSecretsProvider`, `AWSSecretsProvider`
- **Characteristics**:
  - Abstracts external dependencies
  - Often configuration-driven
  - Pluggable implementations

#### **Adapter Pattern**
- **Purpose**: Unified interface for diverse tool integrations
- **Examples**: `Adapter`, `CoreAdapter`, `HTTPAdapter`, `MCPAdapter`
- **Characteristics**:
  - Converts between different interfaces
  - Execution-focused
  - Tool/service specific

#### **Store Pattern**
- **Purpose**: Data persistence/retrieval
- **Examples**: `BlobStore`, `S3BlobStore`, `FilesystemBlobStore`
- **Characteristics**:
  - CRUD operations
  - Data-focused
  - Storage technology specific

### 4. Configuration Patterns

#### **Config Struct Naming: `{Domain}Config`**
- `Config` - Root configuration
- `StorageConfig` - Storage-specific configuration
- `EventConfig` - Event bus configuration
- `SecretsConfig` - Secrets provider configuration
- `BlobConfig` - Blob store configuration
- `RegistryConfig` - Registry configuration

#### **Default Values Naming: `Default{Property}`**
- `DefaultConfigDir` - Default configuration directory
- `DefaultFlowsDir` - Default flows directory
- `DefaultSQLiteDSN` - Default SQLite data source name
- `DefaultLocalRegistryPath` - Default local registry path

## Consistency Issues Identified

### 1. Storage vs Store Inconsistency

**Issue**: Mixed usage of "Storage" and "Store" suffixes
- `Storage` interface vs `BlobStore` interface
- `SqliteStorage` vs `S3BlobStore`

**Recommendation**: 
- Use `Storage` for persistent state (flows, runs, metadata)
- Use `Store` for binary/blob data
- Current usage is actually correct - keep as-is

### 2. Registry Terminology Overload

**Issue**: "Registry" used in multiple contexts
- `adapter.Registry` (collection of adapters)
- `RegistryManager` (manages multiple registries)
- `LocalRegistry` (implements MCPRegistry interface)
- `RegistryConfig` (configuration for registries)

**Analysis**: This is actually consistent - all relate to collections/catalogs of items

### 3. Factory vs Creator Functions

**Issue**: Mixed patterns for object creation
- `NewSecretsProvider()` - Factory function
- `NewFactory()` - Factory object constructor
- `NewDefaultAdapterRegistry()` - Complex initialization

**Recommendation**: 
- Simple cases: `New{Type}()`
- Config-driven: `New{Type}FromConfig()`
- Complex initialization: `NewDefault{Type}()`

## Secrets Implementation Analysis

### Current Implementation Consistency

Our secrets implementation follows BeemFlow patterns correctly:

#### ✅ **Interface Design**
```go
type SecretsProvider interface {
    GetSecret(ctx context.Context, key string) (string, error)
    Close() error
}
```
- Follows `{Domain}` naming convention
- Clean, minimal interface design
- Consistent with other provider interfaces

#### ✅ **Implementation Naming**
```go
type EnvSecretsProvider struct { ... }
type AWSSecretsProvider struct { ... }
```
- Follows `{Technology}{Domain}` convention
- Consistent with `SqliteStorage`, `S3BlobStore` patterns

#### ✅ **Factory Function**
```go
func NewSecretsProvider(ctx context.Context, cfg *config.SecretsConfig) (SecretsProvider, error)
```
- Follows `New{Type}FromConfig` pattern
- Consistent with `NewEventBusFromConfig()` pattern

#### ✅ **Configuration**
```go
type SecretsConfig struct {
    Driver string `json:"driver"`
    Region string `json:"region,omitempty"`
    Prefix string `json:"prefix,omitempty"`
}
```
- Follows `{Domain}Config` naming convention
- Consistent with `EventConfig`, `StorageConfig` patterns

## Architectural Consistency Analysis

### 1. Engine Integration Pattern

**Consistent Pattern**:
```go
type Engine struct {
    Adapters        *adapter.Registry
    Templater       *dsl.Templater
    EventBus        event.EventBus
    BlobStore       blob.BlobStore
    Storage         storage.Storage
    SecretsProvider secrets.SecretsProvider
}
```

**Analysis**: All dependencies follow interface-based injection pattern

### 2. Configuration-Driven Initialization

**Consistent Pattern**:
```go
// In core/dependencies.go
func InitializeDependencies(cfg *config.Config) (func(), error) {
    // Initialize each component from config
    store, err := GetStoreFromConfig(cfg)
    bus, err := event.NewEventBusFromConfig(cfg.Event)
    blobStore, err := blob.NewDefaultBlobStore(ctx, blobConfig)
    secretsProvider, err := secrets.NewSecretsProvider(ctx, cfg.Secrets)
}
```

**Analysis**: All components follow the same initialization pattern

### 3. Default Fallback Pattern

**Consistent Pattern**:
```go
// Graceful fallback to defaults with warnings
if secretsProvider, err := secrets.NewSecretsProvider(ctx, cfg.Secrets); err == nil {
    engine.SetSecretsProvider(secretsProvider)
} else {
    utils.WarnCtx(ctx, "Failed to create secrets provider: %v, using default", "error", err)
}
```

**Analysis**: All components follow the same error handling/fallback pattern

## Recommendations

### 1. Maintain Current Patterns

The current naming conventions are intellectually honest and consistent:

- **Interfaces**: Simple domain nouns (`Adapter`, `Storage`, `SecretsProvider`)
- **Implementations**: Technology/type prefix (`SqliteStorage`, `AWSSecretsProvider`)
- **Factories**: `New{Type}` or `New{Type}FromConfig`
- **Configuration**: `{Domain}Config`

### 2. Secrets Implementation Verdict

Our secrets implementation is **architecturally consistent** with BeemFlow patterns:
- ✅ Interface design matches other providers
- ✅ Naming follows established conventions
- ✅ Factory pattern matches existing code
- ✅ Configuration structure is consistent
- ✅ Engine integration follows the same pattern

### 3. Future Extensions

When adding new components, follow these patterns:

#### **For External Service Integrations**:
```go
type {Service}Provider interface { ... }
type {Technology}{Service}Provider struct { ... }
func New{Service}Provider(ctx context.Context, cfg *config.{Service}Config) ({Service}Provider, error)
```

#### **For Data Storage**:
```go
type {Domain}Store interface { ... }  // for blob/binary data
type {Domain}Storage interface { ... } // for structured data
```

#### **For Collections/Catalogs**:
```go
type {Domain}Registry interface { ... }
type {Type}{Domain}Registry struct { ... }
```

## Conclusion

The BeemFlow codebase demonstrates **strong architectural consistency** with well-established patterns:

1. **Interface-based design** with pluggable implementations
2. **Configuration-driven initialization** with graceful fallbacks
3. **Consistent naming conventions** across all components
4. **Clean separation of concerns** between different architectural layers

Our secrets management implementation **correctly follows all established patterns** and maintains the intellectual honesty of the codebase architecture. No changes are needed to the current implementation.

The codebase shows mature architectural thinking with consistent application of design patterns across all components. The naming conventions are not arbitrary but reflect the actual architectural roles and responsibilities of each component.