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

// TestSecretsEngineIntegration tests that secrets work properly with the BeemFlow engine
func TestSecretsEngineIntegration(t *testing.T) {
	ctx := context.Background()
	
	// Set up test environment
	os.Setenv("INTEGRATION_SECRET", "integration_value")
	os.Setenv("BEEMFLOW_API_KEY", "beemflow_api_key")
	defer func() {
		os.Unsetenv("INTEGRATION_SECRET")
		os.Unsetenv("BEEMFLOW_API_KEY")
	}()
	
	t.Run("BasicSecretAccess", func(t *testing.T) {
		// Create engine with secrets provider
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		
		// Set up secrets provider
		secretsProvider := secrets.NewEnvSecretsProvider("")
		eng.SetSecretsProvider(secretsProvider)
		
		// Create flow that uses secrets
		flow := &model.Flow{
			Name: "secret_test",
			Steps: []model.Step{
				{
					ID:  "test_secret",
					Use: "core.echo",
					With: map[string]any{
						"text": "Secret value: {{ secrets.INTEGRATION_SECRET }}",
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
		stepOutput, ok := outputs["test_secret"].(map[string]any)
		if !ok {
			t.Fatal("Expected step output to be map[string]any")
		}
		
		text, ok := stepOutput["text"].(string)
		if !ok {
			t.Fatal("Expected text output to be string")
		}
		
		expectedText := "Secret value: integration_value"
		if text != expectedText {
			t.Fatalf("Expected '%s', got '%s'", expectedText, text)
		}
	})
	
	t.Run("SecretWithPrefix", func(t *testing.T) {
		// Create engine with prefixed secrets provider
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		
		// Set up secrets provider with prefix
		secretsProvider := secrets.NewEnvSecretsProvider("BEEMFLOW_")
		eng.SetSecretsProvider(secretsProvider)
		
		// Create flow that uses prefixed secrets
		flow := &model.Flow{
			Name: "prefixed_secret_test",
			Steps: []model.Step{
				{
					ID:  "test_prefixed_secret",
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
		stepOutput, ok := outputs["test_prefixed_secret"].(map[string]any)
		if !ok {
			t.Fatal("Expected step output to be map[string]any")
		}
		
		text, ok := stepOutput["text"].(string)
		if !ok {
			t.Fatal("Expected text output to be string")
		}
		
		expectedText := "API Key: beemflow_api_key"
		if text != expectedText {
			t.Fatalf("Expected '%s', got '%s'", expectedText, text)
		}
	})
	
	t.Run("MultipleSecretsInFlow", func(t *testing.T) {
		// Set up additional test secrets
		os.Setenv("SECRET_ONE", "value_one")
		os.Setenv("SECRET_TWO", "value_two")
		defer func() {
			os.Unsetenv("SECRET_ONE")
			os.Unsetenv("SECRET_TWO")
		}()
		
		// Create engine
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		
		// Set up secrets provider
		secretsProvider := secrets.NewEnvSecretsProvider("")
		eng.SetSecretsProvider(secretsProvider)
		
		// Create flow with multiple secrets
		flow := &model.Flow{
			Name: "multi_secret_test",
			Steps: []model.Step{
				{
					ID:  "step1",
					Use: "core.echo",
					With: map[string]any{
						"text": "First: {{ secrets.SECRET_ONE }}",
					},
				},
				{
					ID:  "step2",
					Use: "core.echo",
					With: map[string]any{
						"text": "Second: {{ secrets.SECRET_TWO }}, Combined: {{ secrets.SECRET_ONE }}-{{ secrets.SECRET_TWO }}",
					},
				},
			},
		}
		
		// Execute flow
		outputs, err := eng.Execute(ctx, flow, map[string]any{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check first step
		step1Output, ok := outputs["step1"].(map[string]any)
		if !ok {
			t.Fatal("Expected step1 output to be map[string]any")
		}
		
		text1, ok := step1Output["text"].(string)
		if !ok {
			t.Fatal("Expected step1 text output to be string")
		}
		
		expectedText1 := "First: value_one"
		if text1 != expectedText1 {
			t.Fatalf("Expected '%s', got '%s'", expectedText1, text1)
		}
		
		// Check second step
		step2Output, ok := outputs["step2"].(map[string]any)
		if !ok {
			t.Fatal("Expected step2 output to be map[string]any")
		}
		
		text2, ok := step2Output["text"].(string)
		if !ok {
			t.Fatal("Expected step2 text output to be string")
		}
		
		expectedText2 := "Second: value_two, Combined: value_one-value_two"
		if text2 != expectedText2 {
			t.Fatalf("Expected '%s', got '%s'", expectedText2, text2)
		}
	})
	
	t.Run("SecretNotFound", func(t *testing.T) {
		// Create engine
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		
		// Set up secrets provider
		secretsProvider := secrets.NewEnvSecretsProvider("")
		eng.SetSecretsProvider(secretsProvider)
		
		// Create flow with non-existent secret
		flow := &model.Flow{
			Name: "missing_secret_test",
			Steps: []model.Step{
				{
					ID:  "test_missing",
					Use: "core.echo",
					With: map[string]any{
						"text": "Missing: {{ secrets.NON_EXISTENT_SECRET }}",
					},
				},
			},
		}
		
		// Execute flow - should succeed but with empty secret value
		outputs, err := eng.Execute(ctx, flow, map[string]any{})
		if err != nil {
			t.Fatalf("Expected no error for missing secret (should render empty), got %v", err)
		}
		
		// Check that the missing secret rendered as empty string
		stepOutput, ok := outputs["test_missing"].(map[string]any)
		if !ok {
			t.Fatal("Expected step output to be map[string]any")
		}
		
		text, ok := stepOutput["text"].(string)
		if !ok {
			t.Fatal("Expected text output to be string")
		}
		
		expectedText := "Missing: "
		if text != expectedText {
			t.Fatalf("Expected '%s' (empty secret), got '%s'", expectedText, text)
		}
	})
	
	t.Run("SecretsWithComplexTemplating", func(t *testing.T) {
		// Set up test secrets
		os.Setenv("BASE_URL", "https://api.example.com")
		os.Setenv("API_TOKEN", "secret_token_123")
		defer func() {
			os.Unsetenv("BASE_URL")
			os.Unsetenv("API_TOKEN")
		}()
		
		// Create engine
		adapters := NewDefaultAdapterRegistry(ctx)
		templater := dsl.NewTemplater()
		eventBus := event.NewInProcEventBus()
		storage := storage.NewMemoryStorage()
		
		eng := NewEngine(adapters, templater, eventBus, nil, storage)
		
		// Set up secrets provider
		secretsProvider := secrets.NewEnvSecretsProvider("")
		eng.SetSecretsProvider(secretsProvider)
		
		// Create flow with complex templating
		flow := &model.Flow{
			Name: "complex_template_test",
			Vars: map[string]any{
				"endpoint": "/users",
			},
			Steps: []model.Step{
				{
					ID:  "build_url",
					Use: "core.echo",
					With: map[string]any{
						"text": "{{ secrets.BASE_URL }}{{ vars.endpoint }}?token={{ secrets.API_TOKEN }}",
					},
				},
			},
		}
		
		// Execute flow
		outputs, err := eng.Execute(ctx, flow, map[string]any{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check result
		stepOutput, ok := outputs["build_url"].(map[string]any)
		if !ok {
			t.Fatal("Expected step output to be map[string]any")
		}
		
		text, ok := stepOutput["text"].(string)
		if !ok {
			t.Fatal("Expected text output to be string")
		}
		
		expectedText := "https://api.example.com/users?token=secret_token_123"
		if text != expectedText {
			t.Fatalf("Expected '%s', got '%s'", expectedText, text)
		}
	})
}

// TestSecretsConfigurationIntegration tests that secrets work with configuration
func TestSecretsConfigurationIntegration(t *testing.T) {
	ctx := context.Background()
	
	// Set up test environment
	os.Setenv("CONFIG_SECRET", "config_value")
	defer os.Unsetenv("CONFIG_SECRET")
	
	t.Run("DefaultConfiguration", func(t *testing.T) {
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
	
	t.Run("ExplicitEnvConfiguration", func(t *testing.T) {
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
	
	t.Run("PrefixedConfiguration", func(t *testing.T) {
		// Set up prefixed secret
		os.Setenv("TEST_PREFIX_SECRET", "prefixed_config_value")
		defer os.Unsetenv("TEST_PREFIX_SECRET")
		
		cfg := &config.SecretsConfig{
			Driver: "env",
			Prefix: "TEST_PREFIX_",
		}
		
		provider, err := secrets.NewSecretsProvider(ctx, cfg)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		defer provider.Close()
		
		// Test that it works with prefix
		value, err := provider.GetSecret(ctx, "SECRET")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if value != "prefixed_config_value" {
			t.Fatalf("Expected 'prefixed_config_value', got '%s'", value)
		}
	})
}

// TestSecretsProviderCleanup tests that providers clean up properly
func TestSecretsProviderCleanup(t *testing.T) {
	ctx := context.Background()
	
	t.Run("EnvProviderCleanup", func(t *testing.T) {
		provider := secrets.NewEnvSecretsProvider("")
		
		// Should not error on close
		err := provider.Close()
		if err != nil {
			t.Fatalf("Expected no error on close, got %v", err)
		}
		
		// Should still work after close (env provider has no resources to clean up)
		os.Setenv("CLEANUP_TEST", "cleanup_value")
		defer os.Unsetenv("CLEANUP_TEST")
		
		value, err := provider.GetSecret(ctx, "CLEANUP_TEST")
		if err != nil {
			t.Fatalf("Expected no error after close, got %v", err)
		}
		
		if value != "cleanup_value" {
			t.Fatalf("Expected 'cleanup_value', got '%s'", value)
		}
	})
	
	t.Run("MultipleCloseOperations", func(t *testing.T) {
		provider := secrets.NewEnvSecretsProvider("")
		
		// Should handle multiple close operations gracefully
		err := provider.Close()
		if err != nil {
			t.Fatalf("Expected no error on first close, got %v", err)
		}
		
		err = provider.Close()
		if err != nil {
			t.Fatalf("Expected no error on second close, got %v", err)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}