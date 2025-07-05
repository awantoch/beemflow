package secrets

import (
	"context"
	"os"
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

	t.Run("WithoutPrefix", func(t *testing.T) {
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

	t.Run("WithPrefix", func(t *testing.T) {
		provider := NewEnvSecretsProvider("BEEMFLOW_")
		
		value, err := provider.GetSecret(ctx, "API_KEY")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if value != "api_key_value" {
			t.Fatalf("Expected 'api_key_value', got '%s'", value)
		}
	})

	t.Run("FallbackWithoutPrefix", func(t *testing.T) {
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

	t.Run("Close", func(t *testing.T) {
		provider := NewEnvSecretsProvider("")
		err := provider.Close()
		if err != nil {
			t.Fatalf("Expected no error on close, got %v", err)
		}
	})
}

func TestNewSecretsProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("DefaultToEnv", func(t *testing.T) {
		provider, err := NewSecretsProvider(ctx, nil)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Should be an EnvSecretsProvider
		if _, ok := provider.(*EnvSecretsProvider); !ok {
			t.Fatal("Expected EnvSecretsProvider for nil config")
		}
	})

	t.Run("EnvDriver", func(t *testing.T) {
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

	t.Run("AWSDriverWithoutRegion", func(t *testing.T) {
		cfg := &config.SecretsConfig{
			Driver: "aws-sm",
		}
		
		_, err := NewSecretsProvider(ctx, cfg)
		if err == nil {
			t.Fatal("Expected error for AWS driver without region")
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
	})
}