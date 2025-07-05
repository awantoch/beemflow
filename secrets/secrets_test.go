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
		
		expectedMsg := "unsupported secrets driver: unsupported"
		if err.Error() != expectedMsg {
			t.Fatalf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})
}

// TestSecretsProviderInterface ensures all implementations satisfy the interface
func TestSecretsProviderInterface(t *testing.T) {
	ctx := context.Background()
	
	t.Run("EnvSecretsProvider", func(t *testing.T) {
		var provider SecretsProvider = NewEnvSecretsProvider("")
		
		// Test interface methods exist
		_, err := provider.GetSecret(ctx, "TEST_KEY")
		if err == nil {
			t.Fatal("Expected error for non-existent key")
		}
		
		err = provider.Close()
		if err != nil {
			t.Fatalf("Expected no error on close, got %v", err)
		}
	})
}

// TestEnvSecretsProviderEdgeCases tests edge cases and error conditions
func TestEnvSecretsProviderEdgeCases(t *testing.T) {
	ctx := context.Background()
	
	t.Run("EmptyKey", func(t *testing.T) {
		provider := NewEnvSecretsProvider("")
		
		_, err := provider.GetSecret(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty key")
		}
		
		if !strings.Contains(err.Error(), "secret not found in environment variables") {
			t.Fatalf("Expected descriptive error message, got: %v", err)
		}
	})
	
	t.Run("WhitespaceKey", func(t *testing.T) {
		provider := NewEnvSecretsProvider("")
		
		_, err := provider.GetSecret(ctx, "   ")
		if err == nil {
			t.Fatal("Expected error for whitespace key")
		}
	})
	
	t.Run("EmptyValue", func(t *testing.T) {
		os.Setenv("EMPTY_SECRET", "")
		defer os.Unsetenv("EMPTY_SECRET")
		
		provider := NewEnvSecretsProvider("")
		
		_, err := provider.GetSecret(ctx, "EMPTY_SECRET")
		if err == nil {
			t.Fatal("Expected error for empty environment variable")
		}
	})
	
	t.Run("SpecialCharacters", func(t *testing.T) {
		os.Setenv("SPECIAL_SECRET_123", "special_value")
		defer os.Unsetenv("SPECIAL_SECRET_123")
		
		provider := NewEnvSecretsProvider("")
		
		value, err := provider.GetSecret(ctx, "SPECIAL_SECRET_123")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if value != "special_value" {
			t.Fatalf("Expected 'special_value', got '%s'", value)
		}
	})
	
	t.Run("CaseSensitivity", func(t *testing.T) {
		os.Setenv("CaseSensitive", "case_value")
		defer os.Unsetenv("CaseSensitive")
		
		provider := NewEnvSecretsProvider("")
		
		// Should not find lowercase version
		_, err := provider.GetSecret(ctx, "casesensitive")
		if err == nil {
			t.Fatal("Expected error for case mismatch")
		}
		
		// Should find exact case
		value, err := provider.GetSecret(ctx, "CaseSensitive")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if value != "case_value" {
			t.Fatalf("Expected 'case_value', got '%s'", value)
		}
	})
	
	t.Run("PrefixEdgeCases", func(t *testing.T) {
		os.Setenv("PREFIX_TEST", "prefixed_value")
		os.Setenv("TEST", "unprefixed_value")
		defer func() {
			os.Unsetenv("PREFIX_TEST")
			os.Unsetenv("TEST")
		}()
		
		provider := NewEnvSecretsProvider("PREFIX_")
		
		// Should find prefixed version first
		value, err := provider.GetSecret(ctx, "TEST")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if value != "prefixed_value" {
			t.Fatalf("Expected 'prefixed_value', got '%s'", value)
		}
	})
	
	t.Run("FallbackBehavior", func(t *testing.T) {
		os.Setenv("FALLBACK_TEST", "fallback_value")
		defer os.Unsetenv("FALLBACK_TEST")
		
		provider := NewEnvSecretsProvider("NONEXISTENT_")
		
		// Should fall back to unprefixed version
		value, err := provider.GetSecret(ctx, "FALLBACK_TEST")
		if err != nil {
			t.Fatalf("Expected no error with fallback, got %v", err)
		}
		
		if value != "fallback_value" {
			t.Fatalf("Expected 'fallback_value', got '%s'", value)
		}
	})
}

// TestSecretsProviderFactory tests the factory function comprehensively
func TestSecretsProviderFactory(t *testing.T) {
	ctx := context.Background()
	
	t.Run("NilConfig", func(t *testing.T) {
		provider, err := NewSecretsProvider(ctx, nil)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if provider == nil {
			t.Fatal("Expected provider, got nil")
		}
		
		// Should default to env provider
		if _, ok := provider.(*EnvSecretsProvider); !ok {
			t.Fatal("Expected EnvSecretsProvider for nil config")
		}
	})
	
	t.Run("EmptyConfig", func(t *testing.T) {
		cfg := &config.SecretsConfig{}
		
		provider, err := NewSecretsProvider(ctx, cfg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if _, ok := provider.(*EnvSecretsProvider); !ok {
			t.Fatal("Expected EnvSecretsProvider for empty config")
		}
	})
	
	t.Run("ExplicitEnvDriver", func(t *testing.T) {
		cfg := &config.SecretsConfig{
			Driver: "env",
		}
		
		provider, err := NewSecretsProvider(ctx, cfg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if _, ok := provider.(*EnvSecretsProvider); !ok {
			t.Fatal("Expected EnvSecretsProvider for env driver")
		}
	})
	
	t.Run("CaseInsensitiveDrivers", func(t *testing.T) {
		testCases := []string{"ENV", "Env", "eNv"}
		
		for _, driver := range testCases {
			cfg := &config.SecretsConfig{
				Driver: driver,
			}
			
			provider, err := NewSecretsProvider(ctx, cfg)
			if err != nil {
				t.Fatalf("Expected no error for driver '%s', got %v", driver, err)
			}
			
			if _, ok := provider.(*EnvSecretsProvider); !ok {
				t.Fatalf("Expected EnvSecretsProvider for driver '%s'", driver)
			}
		}
	})
	
	t.Run("AWSDriverVariations", func(t *testing.T) {
		testCases := []string{"aws-sm", "aws", "AWS", "AWS-SM"}
		
		for _, driver := range testCases {
			cfg := &config.SecretsConfig{
				Driver: driver,
			}
			
			_, err := NewSecretsProvider(ctx, cfg)
			if err == nil {
				t.Fatalf("Expected error for driver '%s' without region", driver)
			}
			
			expectedMsg := "region is required for AWS Secrets Manager"
			if err.Error() != expectedMsg {
				t.Fatalf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
			}
		}
	})
}

// TestSecretsProviderConcurrency tests concurrent access to secrets providers
func TestSecretsProviderConcurrency(t *testing.T) {
	ctx := context.Background()
	
	// Set up test environment
	os.Setenv("CONCURRENT_TEST", "concurrent_value")
	defer os.Unsetenv("CONCURRENT_TEST")
	
	provider := NewEnvSecretsProvider("")
	
	// Test concurrent reads
	const numGoroutines = 10
	const numReads = 100
	
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*numReads)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numReads; j++ {
				value, err := provider.GetSecret(ctx, "CONCURRENT_TEST")
				if err != nil {
					errors <- err
					return
				}
				if value != "concurrent_value" {
					errors <- err
					return
				}
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Fatalf("Concurrent access failed: %v", err)
		}
	}
	
	// Check for any remaining errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent access failed: %v", err)
	default:
		// No errors, test passed
	}
}

// TestSecretsProviderContextCancellation tests context cancellation handling
func TestSecretsProviderContextCancellation(t *testing.T) {
	provider := NewEnvSecretsProvider("")
	
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	// The env provider doesn't actually use context for cancellation,
	// but we test that it doesn't panic with cancelled context
	os.Setenv("CANCEL_TEST", "cancel_value")
	defer os.Unsetenv("CANCEL_TEST")
	
	value, err := provider.GetSecret(ctx, "CANCEL_TEST")
	if err != nil {
		t.Fatalf("Expected no error with cancelled context, got %v", err)
	}
	
	if value != "cancel_value" {
		t.Fatalf("Expected 'cancel_value', got '%s'", value)
	}
}