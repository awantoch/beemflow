package engine

import (
	"context"
	"os"
	"testing"

	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/dsl"
	"github.com/awantoch/beemflow/event"
	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/secrets"
	"github.com/awantoch/beemflow/storage"
)

// TestSecretsInEngine tests that secrets work properly with the BeemFlow engine
func TestSecretsInEngine(t *testing.T) {
	ctx := context.Background()
	
	// Set up test environment
	os.Setenv("TEST_SECRET", "secret_value")
	os.Setenv("BEEMFLOW_API_KEY", "api_key_value")
	defer func() {
		os.Unsetenv("TEST_SECRET")
		os.Unsetenv("BEEMFLOW_API_KEY")
	}()
	
	t.Run("BasicSecretResolution", func(t *testing.T) {
		// Create engine with secrets provider
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		eng.SetSecretsProvider(secrets.NewEnvSecretsProvider(""))
		
		// Create flow that uses secrets
		flow := &model.Flow{
			Name: "secret_test",
			Steps: []model.Step{
				{
					ID:  "test_step",
					Use: "core.echo",
					With: map[string]any{
						"text": "Value: {{ secrets.TEST_SECRET }}",
					},
				},
			},
		}
		
		// Execute flow
		outputs, err := eng.Execute(ctx, flow, map[string]any{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check that secret was resolved
		stepOutput := outputs["test_step"].(map[string]any)
		text := stepOutput["text"].(string)
		
		expectedText := "Value: secret_value"
		if text != expectedText {
			t.Fatalf("Expected '%s', got '%s'", expectedText, text)
		}
	})
	
	t.Run("PrefixedSecrets", func(t *testing.T) {
		// Create engine with prefixed secrets provider
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		eng.SetSecretsProvider(secrets.NewEnvSecretsProvider("BEEMFLOW_"))
		
		// Create flow that uses prefixed secrets
		flow := &model.Flow{
			Name: "prefixed_test",
			Steps: []model.Step{
				{
					ID:  "test_step",
					Use: "core.echo",
					With: map[string]any{
						"text": "API Key: {{ secrets.API_KEY }}",
					},
				},
			},
		}
		
		// Execute flow
		outputs, err := eng.Execute(ctx, flow, map[string]any{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check that prefixed secret was resolved
		stepOutput := outputs["test_step"].(map[string]any)
		text := stepOutput["text"].(string)
		
		expectedText := "API Key: api_key_value"
		if text != expectedText {
			t.Fatalf("Expected '%s', got '%s'", expectedText, text)
		}
	})
	
	t.Run("MissingSecretHandling", func(t *testing.T) {
		// Create engine
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		eng.SetSecretsProvider(secrets.NewEnvSecretsProvider(""))
		
		// Create flow with non-existent secret
		flow := &model.Flow{
			Name: "missing_secret_test",
			Steps: []model.Step{
				{
					ID:  "test_step",
					Use: "core.echo",
					With: map[string]any{
						"text": "Missing: {{ secrets.NON_EXISTENT }}",
					},
				},
			},
		}
		
		// Execute flow - should succeed with empty secret value
		outputs, err := eng.Execute(ctx, flow, map[string]any{})
		if err != nil {
			t.Fatalf("Expected no error for missing secret, got %v", err)
		}
		
		// Check that the missing secret rendered as empty string
		stepOutput := outputs["test_step"].(map[string]any)
		text := stepOutput["text"].(string)
		
		expectedText := "Missing: "
		if text != expectedText {
			t.Fatalf("Expected '%s' (empty secret), got '%s'", expectedText, text)
		}
	})
}

// TestSecretsConfiguration tests that secrets work with configuration
func TestSecretsConfiguration(t *testing.T) {
	ctx := context.Background()
	
	// Set up test environment
	os.Setenv("CONFIG_SECRET", "config_value")
	defer os.Unsetenv("CONFIG_SECRET")
	
	t.Run("DefaultProvider", func(t *testing.T) {
		// Create secrets provider from nil config (should default to env)
		provider, err := secrets.NewSecretsProvider(ctx, nil)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		defer provider.Close()
		
		// Test that it works
		value, err := provider.GetSecret(ctx, "CONFIG_SECRET")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if value != "config_value" {
			t.Fatalf("Expected 'config_value', got '%s'", value)
		}
	})
	
	t.Run("ExplicitEnvProvider", func(t *testing.T) {
		cfg := &config.SecretsConfig{
			Driver: "env",
			Prefix: "",
		}
		
		provider, err := secrets.NewSecretsProvider(ctx, cfg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		defer provider.Close()
		
		// Test that it works
		value, err := provider.GetSecret(ctx, "CONFIG_SECRET")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if value != "config_value" {
			t.Fatalf("Expected 'config_value', got '%s'", value)
		}
	})
}