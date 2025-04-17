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

type TracingFeatures struct {
	ReplicaCount                     int
	IsFailoverRequest                bool
	UseAIConfiguration               bool
	UseAIChatCompletionConfiguration bool
}

// Feature flag telemetry
const (
	TelemetryKey            string = "telemetry"
	EnabledKey              string = "enabled"
	MetadataKey             string = "metadata"
	ETagKey                 string = "ETag"
	FeatureFlagReferenceKey string = "FeatureFlagReference"
)

// AI Configuration telemetry
const (
	AIMimeProfileKey               string = "https://azconfig.io/mime-profiles/ai"
	AIChatCompletionMimeProfileKey string = "https://azconfig.io/mime-profiles/ai/chat-completion"
)

const (
	RequestTracingKey          TracingKey = TracingKey("tracing")
	AzureExtensionContext      string     = "AZURE_EXTENSION_CONTEXT"
	TracingFeatureDelimiterKey string     = "+"
	LoadBalancingKey           string     = "LB"
	AIConfigurationKey         string     = "AI"
	AIChatCompletionKey        string     = "AICC"
)

func createCorrelationContextHeader(ctx context.Context, provider acpv1.AzureAppConfigurationProvider, tracingFeatures TracingFeatures) http.Header {
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
		output = append(output, fmt.Sprintf("ReplicaCount=%d", tracingFeatures.ReplicaCount))

		if tracingFeatures.IsFailoverRequest {
			output = append(output, "FailoverRequest")
		}
	}

	features := make([]string, 0)
	if provider.Spec.LoadBalancingEnabled {
		features = append(features, LoadBalancingKey)
	}

	if tracingFeatures.UseAIConfiguration {
		features = append(features, AIConfigurationKey)
	}

	if tracingFeatures.UseAIChatCompletionConfiguration {
		features = append(features, AIChatCompletionKey)
	}

	if len(features) > 0 {
		featureStr := "Features=" + strings.Join(features, TracingFeatureDelimiterKey)
		output = append(output, featureStr)
	}

	if _, ok := os.LookupEnv(AzureExtensionContext); ok {
		output = append(output, "InstalledBy=Extension")
	} else {
		output = append(output, "InstalledBy=Helm")
	}

	header.Add("Correlation-Context", strings.Join(output, ","))

	return header
}
