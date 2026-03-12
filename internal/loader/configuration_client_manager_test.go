// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"azappconfig/provider/internal/properties"
	"fmt"
	"os"
	"testing"

	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
)

func TestNewClientOptions(t *testing.T) {
	expectedAppID := fmt.Sprintf("%s/%s", properties.ModuleName, properties.ModuleVersion)

	tests := []struct {
		name             string
		envVars          map[string]string
		expectedAudience string
		hasCloudConfig   bool
	}{
		{
			name:             "no audience set - default behavior",
			envVars:          nil,
			expectedAudience: "",
			hasCloudConfig:   false,
		},
		{
			name: "audience set to Azure Government",
			envVars: map[string]string{
				AzureAppConfigAudience: "https://appconfig.azure.us",
			},
			expectedAudience: "https://appconfig.azure.us",
			hasCloudConfig:   true,
		},
		{
			name: "audience set to Azure China",
			envVars: map[string]string{
				AzureAppConfigAudience: "https://appconfig.azure.cn",
			},
			expectedAudience: "https://appconfig.azure.cn",
			hasCloudConfig:   true,
		},
		{
			name: "audience set to empty string",
			envVars: map[string]string{
				AzureAppConfigAudience: "",
			},
			expectedAudience: "",
			hasCloudConfig:   false,
		},
		{
			name: "audience set to custom value",
			envVars: map[string]string{
				AzureAppConfigAudience: "https://custom.audience.example",
			},
			expectedAudience: "https://custom.audience.example",
			hasCloudConfig:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up the env var before each test
			if err := os.Unsetenv(AzureAppConfigAudience); err != nil {
				t.Fatalf("Failed to unset environment variable: %v", err)
			}

			// Setup environment variables
			if tt.envVars != nil {
				for k, v := range tt.envVars {
					if err := os.Setenv(k, v); err != nil {
						t.Fatalf("Failed to set environment variable: %v", err)
					}
					defer func(key string) {
						if err := os.Unsetenv(key); err != nil {
							t.Errorf("Failed to unset environment variable: %v", err)
						}
					}(k)
				}
			}

			options := newClientOptions()

			// Verify telemetry ApplicationID is always set
			if options.ClientOptions.Telemetry.ApplicationID != expectedAppID {
				t.Errorf("Expected ApplicationID %q, got %q", expectedAppID, options.ClientOptions.Telemetry.ApplicationID)
			}

			// Verify cloud configuration / audience
			if tt.hasCloudConfig {
				serviceConfig, exists := options.ClientOptions.Cloud.Services[azappconfig.ServiceName]
				if !exists {
					t.Fatal("Expected cloud service configuration to be set for azappconfig.ServiceName, but it was not found")
				}
				if serviceConfig.Audience != tt.expectedAudience {
					t.Errorf("Expected audience %q, got %q", tt.expectedAudience, serviceConfig.Audience)
				}
			} else {
				if len(options.ClientOptions.Cloud.Services) != 0 {
					t.Errorf("Expected no cloud service configuration, but got %v", options.ClientOptions.Cloud.Services)
				}
			}
		})
	}
}
