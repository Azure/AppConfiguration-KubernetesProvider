// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type TracingKey string

type RequestTracing struct {
	IsStartUp bool
}

const (
	RequestTracingKey     TracingKey = TracingKey("tracing")
	AzureExtensionContext string     = "AZURE_EXTENSION_CONTEXT"
)

func createCorrelationContextHeader(ctx context.Context, provider acpv1.AzureAppConfigurationProvider, clientManager ClientManager) http.Header {
	header := http.Header{}
	output := make([]string, 0)

	output = append(output, "Host=Kubernetes")

	if tracing := ctx.Value(RequestTracingKey); tracing != nil {
		if tracing.(RequestTracing).IsStartUp {
			output = append(output, "RequestType=StartUp")
		} else {
			output = append(output, "RequestType=Watch")
		}
	}

	if provider.Spec.Configuration.Refresh != nil &&
		provider.Spec.Configuration.Refresh.Enabled {
		output = append(output, "RefreshesKeyValue")
	}

	if provider.Spec.Secret != nil {
		output = append(output, "UsesKeyVault")

		if provider.Spec.Secret.Refresh != nil &&
			provider.Spec.Secret.Refresh.Enabled {
			output = append(output, "RefreshesKeyVault")
		}
	}

	if provider.Spec.FeatureFlag != nil {
		output = append(output, "UsesFeatureFlag")
	}

	if provider.Spec.ReplicaDiscoveryEnabled {
		if manager, ok := clientManager.(*ConfigurationClientManager); ok {
			replicaCount := 0
			if manager.DynamicClientWrappers != nil {
				replicaCount = len(manager.DynamicClientWrappers)
			}

			output = append(output, fmt.Sprintf("ReplicaCount=%d", replicaCount))
		}
	}

	if _, ok := os.LookupEnv(AzureExtensionContext); ok {
		output = append(output, "InstalledBy=Extension")
	} else {
		output = append(output, "InstalledBy=Helm")
	}

	header.Add("Correlation-Context", strings.Join(output, ","))

	return header
}
