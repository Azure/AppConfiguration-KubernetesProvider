// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"os"
	"strings"
	"testing"
)

func TestCreateCorrelationContextHeader(t *testing.T) {
	tests := []struct {
		name            string
		provider        acpv1.AzureAppConfigurationProvider
		tracingFeatures TracingFeatures
		tracing         *RequestTracing
		envVars         map[string]string
		expected        []string
	}{
		{
			name:            "basic test",
			provider:        acpv1.AzureAppConfigurationProvider{},
			tracingFeatures: TracingFeatures{},
			expected: []string{
				"Host=Kubernetes",
				"InstalledBy=Helm",
			},
		},
		{
			name:            "startup request",
			provider:        acpv1.AzureAppConfigurationProvider{},
			tracingFeatures: TracingFeatures{},
			tracing:         &RequestTracing{IsStartUp: true},
			expected: []string{
				"Host=Kubernetes",
				"RequestType=StartUp",
				"InstalledBy=Helm",
			},
		},
		{
			name:            "watch request",
			provider:        acpv1.AzureAppConfigurationProvider{},
			tracingFeatures: TracingFeatures{},
			tracing:         &RequestTracing{IsStartUp: false},
			expected: []string{
				"Host=Kubernetes",
				"RequestType=Watch",
				"InstalledBy=Helm",
			},
		},
		{
			name: "with secret configuration",
			provider: acpv1.AzureAppConfigurationProvider{
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Secret: &acpv1.SecretReference{
						Refresh: &acpv1.RefreshSettings{
							Enabled: true,
						},
					},
				},
			},
			tracingFeatures: TracingFeatures{},
			expected: []string{
				"Host=Kubernetes",
				"UsesKeyVault",
				"RefreshesKeyVault",
				"InstalledBy=Helm",
			},
		},
		{
			name: "with feature flag",
			provider: acpv1.AzureAppConfigurationProvider{
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{},
				},
			},
			tracingFeatures: TracingFeatures{},
			expected: []string{
				"Host=Kubernetes",
				"UsesFeatureFlag",
				"InstalledBy=Helm",
			},
		},
		{
			name: "with replica discovery",
			provider: acpv1.AzureAppConfigurationProvider{
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					ReplicaDiscoveryEnabled: true,
				},
			},
			tracingFeatures: TracingFeatures{
				ReplicaCount:      3,
				IsFailoverRequest: true,
			},
			expected: []string{
				"Host=Kubernetes",
				"ReplicaCount=3",
				"FailoverRequest",
				"InstalledBy=Helm",
			},
		},
		{
			name: "with features",
			provider: acpv1.AzureAppConfigurationProvider{
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					LoadBalancingEnabled: true,
				},
			},
			tracingFeatures: TracingFeatures{
				UseAIConfiguration:               true,
				UseAIChatCompletionConfiguration: true,
			},
			expected: []string{
				"Host=Kubernetes",
				"Features=LB+AI+AICC",
				"InstalledBy=Helm",
			},
		},
		{
			name:            "with azure extension context",
			provider:        acpv1.AzureAppConfigurationProvider{},
			tracingFeatures: TracingFeatures{},
			envVars: map[string]string{
				AzureExtensionContext: "test",
			},
			expected: []string{
				"Host=Kubernetes",
				"InstalledBy=Extension",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			if tt.envVars != nil {
				for k, v := range tt.envVars {
					os.Setenv(k, v)
					defer os.Unsetenv(k)
				}
			}

			// Setup context
			ctx := context.Background()
			if tt.tracing != nil {
				ctx = context.WithValue(ctx, RequestTracingKey, *tt.tracing)
			}

			// Call the function
			header := createCorrelationContextHeader(ctx, tt.provider, tt.tracingFeatures)

			// Check the result
			corr := header.Get("Correlation-Context")
			if corr == "" {
				t.Fatal("Expected Correlation-Context header, got none")
			}

			// Check each expected string is present
			for _, exp := range tt.expected {
				if !strings.Contains(corr, exp) {
					t.Errorf("Expected correlation context to contain '%s', got '%s'", exp, corr)
				}
			}

			// Split by comma and count items
			items := strings.Split(corr, ",")
			if len(items) != len(tt.expected) {
				t.Errorf("Expected %d items in correlation context, got %d: %s",
					len(tt.expected), len(items), corr)
			}
		})
	}
}
