package secrets

import (
	"context"
	"strings"
	"testing"
)

// TestAWSSecretsProvider tests the AWS Secrets Manager provider
func TestAWSSecretsProvider(t *testing.T) {
	ctx := context.Background()
	
	t.Run("RegionValidation", func(t *testing.T) {
		// Test that region is required
		_, err := NewAWSSecretsProvider(ctx, "", "")
		if err == nil {
			t.Fatal("Expected error when region is empty")
		}
		
		if !strings.Contains(err.Error(), "region") {
			t.Fatalf("Expected error to mention region, got: %v", err)
		}
		
		// Test whitespace region
		_, err = NewAWSSecretsProvider(ctx, "   ", "")
		if err == nil {
			t.Fatal("Expected error for whitespace region")
		}
	})
	
	t.Run("ValidRegionHandling", func(t *testing.T) {
		// This will fail in test environment without AWS credentials, which is expected
		_, err := NewAWSSecretsProvider(ctx, "us-west-2", "test-prefix/")
		
		// In test environment, should fail with AWS credential/config error, not our validation
		if err != nil && strings.Contains(err.Error(), "region is required") {
			t.Fatal("Should not fail with region validation when valid region is provided")
		}
		
		// If we get here, it means either:
		// 1. AWS credentials are available (unlikely in test env) - test passes
		// 2. AWS config error occurred (expected) - test passes
		// 3. Our validation failed (would be a bug) - test fails above
	})
	
	t.Run("InterfaceCompliance", func(t *testing.T) {
		// Compile-time check that AWSSecretsProvider implements SecretsProvider
		var _ SecretsProvider = (*AWSSecretsProvider)(nil)
		
		// This test passes if the code compiles
	})
}