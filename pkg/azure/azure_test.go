// Copyright 2025 Stakater AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/go-kit/log"
	"github.com/jm-stakater/alert-az-do/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBasicCredential(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "valid PAT credential",
			username: "",
			password: "pat-token-123",
			wantErr:  false,
		},
		{
			name:     "valid username and password",
			username: "user",
			password: "pass",
			wantErr:  false,
		},
		{
			name:     "empty username with password",
			username: "",
			password: "password",
			wantErr:  false,
		},
		{
			name:     "username with empty password",
			username: "user",
			password: "",
			wantErr:  false,
		},
		{
			name:     "both empty should fail",
			username: "",
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := NewBasicCredential(tt.username, tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cred)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cred)
				assert.Equal(t, tt.username, cred.Username)
				assert.Equal(t, tt.password, cred.Password)
			}
		})
	}
}

func TestBasicCredential_GetToken(t *testing.T) {
	cred, err := NewBasicCredential("", "test-pat-token")
	require.NoError(t, err)

	ctx := context.Background()
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: getScopes(),
	})
	require.NoError(t, err)

	// Token should be base64 encoded ":<password>"
	assert.NotEmpty(t, token.Token)
	// The token should be base64 encoded, we can test if it's not the raw password
	assert.NotEqual(t, "test-pat-token", token.Token)
}

func TestGetAuthenticationCredential(t *testing.T) {
	logger := log.NewNopLogger()

	tests := []struct {
		name        string
		config      *config.ReceiverConfig
		wantErr     bool
		description string
	}{
		{
			name: "service_principal_authentication",
			config: &config.ReceiverConfig{
				TenantID:     "tenant-123",
				ClientID:     "client-123",
				ClientSecret: config.Secret("secret-123"),
			},
			wantErr:     false,
			description: "Should successfully create Service Principal credential",
		},
		{
			name: "workload_identity_authentication",
			config: &config.ReceiverConfig{
				TenantID: "tenant-123",
				ClientID: "client-123",
			},
			wantErr:     false,
			description: "Should successfully create Workload Identity credential",
		},
		{
			name: "managed_identity_authentication",
			config: &config.ReceiverConfig{
				ClientID:       "client-123",
				SubscriptionID: "sub-123",
			},
			wantErr:     false,
			description: "Should successfully create Managed Identity credential",
		},
		{
			name: "pat_authentication",
			config: &config.ReceiverConfig{
				PersonalAccessToken: config.Secret("pat-token-123"),
			},
			wantErr:     false,
			description: "Should successfully create PAT credential",
		},
		{
			name: "invalid_service_principal_missing_client_id",
			config: &config.ReceiverConfig{
				TenantID:     "tenant-123",
				ClientSecret: config.Secret("secret-123"),
				// Missing ClientID
			},
			wantErr:     true,
			description: "Should fail when ClientID is missing for Service Principal",
		},
		{
			name: "partial_service_principal_becomes_workload_identity",
			config: &config.ReceiverConfig{
				TenantID: "tenant-123",
				ClientID: "client-123",
				// No ClientSecret - this becomes Workload Identity, not an error
			},
			wantErr:     false,
			description: "Should successfully create Workload Identity when TenantID+ClientID provided without ClientSecret",
		},
		{
			name: "invalid_managed_identity_missing_subscription",
			config: &config.ReceiverConfig{
				ClientID: "client-123",
			},
			wantErr:     true,
			description: "Should fail when SubscriptionID is missing for Managed Identity",
		},
		{
			name: "invalid_managed_identity_missing_client_id",
			config: &config.ReceiverConfig{
				SubscriptionID: "sub-123",
			},
			wantErr:     true,
			description: "Should fail when ClientID is missing for Managed Identity",
		},
		{
			name: "no_authentication_provided",
			config: &config.ReceiverConfig{
				Organization: "test-org",
			},
			wantErr:     true,
			description: "Should fail when no authentication method is configured",
		},
		{
			name: "mixed_authentication_service_principal_with_subscription",
			config: &config.ReceiverConfig{
				TenantID:       "tenant-123",
				ClientID:       "client-123",
				ClientSecret:   config.Secret("secret-123"),
				SubscriptionID: "sub-123", // This creates ambiguity
			},
			wantErr:     true,
			description: "Should fail when multiple authentication methods are mixed",
		},
		{
			name: "mixed_authentication_service_principal_with_pat",
			config: &config.ReceiverConfig{
				TenantID:            "tenant-123",
				ClientID:            "client-123",
				ClientSecret:        config.Secret("secret-123"),
				PersonalAccessToken: config.Secret("pat-123"), // This creates ambiguity
			},
			wantErr:     true,
			description: "Should fail when Service Principal is mixed with PAT",
		},
		{
			name: "mixed_authentication_managed_identity_with_pat",
			config: &config.ReceiverConfig{
				ClientID:            "client-123",
				SubscriptionID:      "sub-123",
				PersonalAccessToken: config.Secret("pat-123"), // This creates ambiguity
			},
			wantErr:     true,
			description: "Should fail when Managed Identity is mixed with PAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := GetAuthenticationCredential(logger, tt.config)
			if tt.wantErr {
				assert.Error(t, err, tt.description)
				assert.Nil(t, cred)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, cred)
			}
		})
	}
}

func TestGetConnection(t *testing.T) {
	ctx := context.Background()
	logger := log.NewNopLogger()

	t.Run("successful_pat_connection", func(t *testing.T) {
		config := &config.ReceiverConfig{
			Organization:        "test-org",
			PersonalAccessToken: config.Secret("test-pat"),
		}

		conn, err := GetConnection(ctx, logger, config)
		require.NoError(t, err)
		assert.NotNil(t, conn)
		assert.Equal(t, "https://dev.azure.com/test-org", conn.BaseUrl)
		assert.Contains(t, conn.AuthorizationString, "Basic ")
	})

	t.Run("successful_service_principal_connection", func(t *testing.T) {
		// Note: This test will fail in CI without real Azure credentials
		// but it tests the credential creation logic
		config := &config.ReceiverConfig{
			Organization: "test-org",
			TenantID:     "tenant-123",
			ClientID:     "client-123",
			ClientSecret: config.Secret("secret-123"),
		}

		// We can test credential creation but not token acquisition without real credentials
		cred, err := GetAuthenticationCredential(logger, config)
		assert.NoError(t, err)
		assert.NotNil(t, cred)
	})

	t.Run("failed_connection_no_auth", func(t *testing.T) {
		config := &config.ReceiverConfig{
			Organization: "test-org",
		}

		conn, err := GetConnection(ctx, logger, config)
		assert.Error(t, err)
		assert.Nil(t, conn)
		assert.Contains(t, err.Error(), "no valid authentication method")
	})
}

func TestGetScopes(t *testing.T) {
	scopes := getScopes()
	assert.Len(t, scopes, 1)
	assert.Equal(t, "499b84ac-1321-427f-aa17-267ca6975798/.default", scopes[0])
}

// Integration test helper to test actual authentication flow (requires environment setup)
func TestAuthenticationIntegration(t *testing.T) {
	// Skip this test in CI unless specific environment variables are set
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := log.NewNopLogger()

	t.Run("pat_integration", func(t *testing.T) {
		// This test requires actual PAT token to be set in environment
		// Skip if not available for testing
		t.Skip("requires actual Azure DevOps PAT token for integration testing")

		config := &config.ReceiverConfig{
			Organization:        "your-org",
			PersonalAccessToken: config.Secret("your-actual-pat-token"),
		}

		conn, err := GetConnection(ctx, logger, config)
		if err != nil {
			t.Logf("Integration test failed (expected if no real credentials): %v", err)
			return
		}

		assert.NotNil(t, conn)
		assert.Equal(t, "https://dev.azure.com/your-org", conn.BaseUrl)
	})
}

// Benchmark tests for credential creation
func BenchmarkGetAuthenticationCredential(b *testing.B) {
	logger := log.NewNopLogger()
	config := &config.ReceiverConfig{
		PersonalAccessToken: config.Secret("test-pat-token"),
	}

	for b.Loop() {
		_, err := GetAuthenticationCredential(logger, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBasicCredentialGetToken(b *testing.B) {
	cred, err := NewBasicCredential("", "test-token")
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		_, err := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: getScopes(),
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
