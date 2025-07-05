package secrets

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/awantoch/beemflow/config"
)

func TestEnvSecretsProvider(t *testing.T) {
	ctx := context.Background()
	
	// Set up test environment variables
	os.Setenv("TEST_SECRET", "test_value")
	os.Setenv("BEEMFLOW_API_KEY", "api_key_value")
	defer func() {
		os.Unsetenv("TEST_SECRET")
		os.Unsetenv("BEEMFLOW_API_KEY")
	}()

	t.Run("BasicSecretResolution", func(t *testing.T) {
		provider := NewEnvSecretsProvider("")
		
		value, err := provider.GetSecret(ctx, "TEST_SECRET")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if value != "test_value" {
			t.Fatalf("Expected 'test_value', got '%s'", value)
		}
		
		// Test non-existent secret
		_, err = provider.GetSecret(ctx, "NON_EXISTENT")
		if err == nil {
			t.Fatal("Expected error for non-existent secret")
		}
	})

	t.Run("PrefixSupport", func(t *testing.T) {
		provider := NewEnvSecretsProvider("BEEMFLOW_")
		
		value, err := provider.GetSecret(ctx, "API_KEY")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if value != "api_key_value" {
			t.Fatalf("Expected 'api_key_value', got '%s'", value)
		}
	})

	t.Run("FallbackBehavior", func(t *testing.T) {
		provider := NewEnvSecretsProvider("MISSING_")
		
		// Should fall back to TEST_SECRET without prefix
		value, err := provider.GetSecret(ctx, "TEST_SECRET")
		if err != nil {
			t.Fatalf("Expected no error with fallback, got %v", err)
		}
		if value != "test_value" {
			t.Fatalf("Expected 'test_value', got '%s'", value)
		}
	})

	t.Run("EmptyValues", func(t *testing.T) {
		os.Setenv("EMPTY_SECRET", "")
		defer os.Unsetenv("EMPTY_SECRET")
		
		provider := NewEnvSecretsProvider("")
		
		_, err := provider.GetSecret(ctx, "EMPTY_SECRET")
		if err == nil {
			t.Fatal("Expected error for empty environment variable")
		}
	})
}

func TestSecretsProviderFactory(t *testing.T) {
	ctx := context.Background()

	t.Run("DefaultProvider", func(t *testing.T) {
		provider, err := NewSecretsProvider(ctx, nil)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Should be an EnvSecretsProvider
		if _, ok := provider.(*EnvSecretsProvider); !ok {
			t.Fatal("Expected EnvSecretsProvider for nil config")
		}
	})

	t.Run("EnvProvider", func(t *testing.T) {
		cfg := &config.SecretsConfig{
			Driver: "env",
			Prefix: "TEST_",
		}
		
		provider, err := NewSecretsProvider(ctx, cfg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		envProvider, ok := provider.(*EnvSecretsProvider)
		if !ok {
			t.Fatal("Expected EnvSecretsProvider for env driver")
		}
		
		if envProvider.prefix != "TEST_" {
			t.Fatalf("Expected prefix 'TEST_', got '%s'", envProvider.prefix)
		}
	})

	t.Run("AWSProviderValidation", func(t *testing.T) {
		cfg := &config.SecretsConfig{
			Driver: "aws-sm",
		}
		
		_, err := NewSecretsProvider(ctx, cfg)
		if err == nil {
			t.Fatal("Expected error for AWS driver without region")
		}
		
		if !strings.Contains(err.Error(), "region is required") {
			t.Fatalf("Expected region error, got: %v", err)
		}
	})

	t.Run("UnsupportedDriver", func(t *testing.T) {
		cfg := &config.SecretsConfig{
			Driver: "unsupported",
		}
		
		_, err := NewSecretsProvider(ctx, cfg)
		if err == nil {
			t.Fatal("Expected error for unsupported driver")
		}
		
		expectedMsg := "unsupported secrets driver: unsupported"
		if err.Error() != expectedMsg {
			t.Fatalf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})
}

func TestSecretsProviderInterface(t *testing.T) {
	ctx := context.Background()
	
	// Ensure EnvSecretsProvider implements SecretsProvider interface
	var provider SecretsProvider = NewEnvSecretsProvider("")
	
	// Test interface methods
	_, err := provider.GetSecret(ctx, "NON_EXISTENT")
	if err == nil {
		t.Fatal("Expected error for non-existent key")
	}
	
	err = provider.Close()
	if err != nil {
		t.Fatalf("Expected no error on close, got %v", err)
	}
}