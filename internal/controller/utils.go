// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	acpv1 "azappconfig/provider/api/v1"
	"azappconfig/provider/internal/loader"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MinimalSentinelBasedRefreshInterval         time.Duration = time.Second
	MinimalSecretRefreshInterval                time.Duration = time.Minute
	MinimalFeatureFlagRefreshInterval           time.Duration = time.Second
	WorkloadIdentityEnabled                     string        = "WORKLOAD_IDENTITY_ENABLED"
	WorkloadIdentityDisableGlobalServiceAccount string        = "WORKLOAD_IDENTITY_DISABLE_GLOBAL_SERVICE_ACCOUNT"
)

func verifyObject(spec acpv1.AzureAppConfigurationProviderSpec) error {
	var err error = nil
	if spec.Endpoint == nil && spec.ConnectionStringReference == nil {
		return loader.NewArgumentError("spec", fmt.Errorf("one of endpoint and connectionStringReference field must be set"))
	}
	if spec.ConnectionStringReference != nil {
		if spec.Endpoint != nil {
			return loader.NewArgumentError("spec", fmt.Errorf("both endpoint and connectionStringReference field are set"))
		}
		if spec.Auth != nil {
			return loader.NewArgumentError("spec.auth", fmt.Errorf("auth field is not allowed when connectionStringReference field is set"))
		}
	}

	if spec.Target.ConfigMapData != nil {
		if spec.Target.ConfigMapData.Type == acpv1.Default {
			if spec.Target.ConfigMapData.Key != "" {
				return loader.NewArgumentError("spec.target.configMapData.key", fmt.Errorf("key field is not allowed when type is default"))
			}
		} else {
			if spec.Target.ConfigMapData.Key == "" {
				return loader.NewArgumentError("spec.target.configMapData.key", fmt.Errorf("key field is required when type is json, yaml or properties"))
			}
		}

		if spec.Target.ConfigMapData.Separator != nil &&
			(spec.Target.ConfigMapData.Type == acpv1.Default ||
				spec.Target.ConfigMapData.Type == acpv1.Properties) {
			return loader.NewArgumentError("spec.target.configMapData.separator", fmt.Errorf("separator field is not allowed when type is %s", spec.Target.ConfigMapData.Type))
		}
	}

	for i := range spec.Configuration.Selectors {
		err = verifySelectorObject(spec.Configuration.Selectors[i])
		if err != nil {
			return loader.NewArgumentError("spec.configuration.selectors", err)
		}
	}

	if spec.FeatureFlag != nil {
		if spec.Target.ConfigMapData == nil || spec.Target.ConfigMapData.Type == acpv1.Default || spec.Target.ConfigMapData.Type == acpv1.Properties {
			return loader.NewArgumentError("spec.target.configMapData", fmt.Errorf("configMap data type must be json or yaml when FeatureFlag is set"))
		}

		if len(spec.FeatureFlag.Selectors) == 0 {
			return loader.NewArgumentError("spec.featureFlag.selectors", fmt.Errorf("featureFlag.selectors must be specified when FeatureFlag is set"))
		}

		// Check if feature flag label filters are valid
		for i := range spec.FeatureFlag.Selectors {
			err = verifySelectorObject(spec.FeatureFlag.Selectors[i])
			if err != nil {
				return loader.NewArgumentError("spec.featureFlag.selectors", err)
			}
		}

		// Check if feature flag refresh interval is valid
		if spec.FeatureFlag.Refresh != nil {
			err = verifyRefreshInterval(spec.FeatureFlag.Refresh.Interval, MinimalFeatureFlagRefreshInterval, "featureFlag.refresh.interval")
			if err != nil {
				return err
			}
		}
	}

	if spec.Endpoint != nil {
		err = verifyEndpoint(*spec.Endpoint)
		if err != nil {
			return err
		}
	}
	err = verifyAuthObject(spec.Auth)
	if err != nil {
		return err
	}
	if spec.Secret != nil && spec.Secret.Auth != nil {
		err = verifyAuthObject(spec.Secret.Auth.AzureAppConfigurationProviderAuth)
		if err != nil {
			return err
		}
		for _, v := range spec.Secret.Auth.KeyVaults {
			err = verifyEndpoint(v.Uri)
			if err != nil {
				return err
			}
			if v.AzureAppConfigurationProviderAuth == nil {
				return loader.NewArgumentError("secret.auth.keyVaults", fmt.Errorf("authentication method must be specified for Key Vault '%s'", v.Uri))
			}
			err = verifyAuthObject(v.AzureAppConfigurationProviderAuth)
			if err != nil {
				return err
			}
		}
	}

	if spec.Configuration.Refresh != nil {
		sentinelMap := make(map[acpv1.Sentinel]bool)
		for _, sentinel := range spec.Configuration.Refresh.Monitoring.Sentinels {
			if _, ok := sentinelMap[sentinel]; ok {
				return loader.NewArgumentError("spec.configuration.refresh.monitoring.keyValues", fmt.Errorf("monitoring duplicated key '%s'", sentinel.Key))
			}
			sentinelMap[sentinel] = true
		}

		if spec.Configuration.Refresh.Interval != "" {
			err = verifyRefreshInterval(spec.Configuration.Refresh.Interval, MinimalSentinelBasedRefreshInterval, "configuration.refresh.interval")
			if err != nil {
				return err
			}
		}
	}

	if spec.Secret != nil && spec.Secret.Refresh != nil {
		err = verifyRefreshInterval(spec.Secret.Refresh.Interval, MinimalSecretRefreshInterval, "secret.refresh.interval")
		if err != nil {
			return err
		}
	}

	return nil
}

// verifyEndpoint verifies if the endpoint is a valid key vault endpoint
func verifyEndpoint(endpoint string) error {
	url, err := url.Parse(strings.ToLower(endpoint))
	if err != nil {
		return loader.NewArgumentError("endpoint", err)
	}
	if url.Host == "" {
		return loader.NewArgumentError("endpoint", fmt.Errorf("%q is not a valid endpoint. Host must be specified", endpoint))
	}
	if url.Scheme != "https" {
		return loader.NewArgumentError("endpoint", fmt.Errorf("%q is not a valid endpoint. Only https scheme is allowed", endpoint))
	}
	if strings.Trim(url.Path, "/") != "" {
		return loader.NewArgumentError("endpoint", fmt.Errorf("%q is not a valid endpoint. Only host name is allowed", endpoint))
	}

	return nil
}

func verifyAuthObject(auth *acpv1.AzureAppConfigurationProviderAuth) error {
	if auth != nil {
		var authCount int = 0

		if auth.ServicePrincipalReference != nil {
			authCount++
		}
		if auth.ManagedIdentityClientId != nil {
			authCount++
			_, err := uuid.Parse(*auth.ManagedIdentityClientId)
			if err != nil {
				return loader.NewArgumentError("auth", fmt.Errorf("ManagedIdentityClientId %q in auth field is not a valid uuid", *auth.ManagedIdentityClientId))
			}
		}
		if auth.WorkloadIdentity != nil {
			authCount++
			err := verifyWorkloadIdentityParameters(auth.WorkloadIdentity)
			if err != nil {
				return err
			}
		}
		if authCount > 1 {
			return loader.NewArgumentError("auth", fmt.Errorf("more than one authentication methods are specified in 'auth' field"))
		}
	}

	return nil
}

func verifyExistingTargetObject[T client.Object](targetObj T, targetName string, providerName string) error {
	objectKind := targetObj.GetObjectKind().GroupVersionKind().Kind
	if targetObj.GetName() != targetName {
		return nil
	}

	// If existing object is created by current provider, just skip it.
	for _, ownerRef := range targetObj.GetOwnerReferences() {
		if ownerRef.Name == providerName {
			return nil
		}
	}

	return fmt.Errorf("a %s with name '%s' already exists in namespace '%s'", objectKind, targetName, targetObj.GetNamespace())
}

func hasNonEscapedValueInLabel(label string) bool {
	length := len(label)
	i := 0
	for i < length {
		if label[i] == '\\' {
			i += 2
		} else if label[i] == '*' || label[i] == ',' {
			return true
		} else {
			i++
		}
	}

	return false
}

func verifyRefreshInterval(interval string, allowedMinimalRefreshInterval time.Duration, refreshArgument string) error {
	refreshInterval, err := time.ParseDuration(interval)
	if err == nil {
		if refreshInterval < allowedMinimalRefreshInterval {
			return loader.NewArgumentError(refreshArgument, fmt.Errorf("%s can not be shorter than %s", refreshArgument, allowedMinimalRefreshInterval.String()))
		}
	} else {
		return loader.NewArgumentError(refreshArgument, err)
	}

	return nil
}

func verifyWorkloadIdentityParameters(workloadIdentity *acpv1.WorkloadIdentityParameters) error {
	if !strings.EqualFold(os.Getenv(WorkloadIdentityEnabled), "true") {
		return loader.NewArgumentError("auth.workloadIdentity", fmt.Errorf("workloadIdentity is not enabled"))
	}

	var authCount int = 0

	if workloadIdentity.ManagedIdentityClientId != nil {
		if strings.EqualFold(os.Getenv(WorkloadIdentityDisableGlobalServiceAccount), "true") {
			return loader.NewArgumentError("auth.workloadIdentity.managedIdentityClientId", fmt.Errorf("'managedIdentityClientId' is not allowed since global service account is disabled"))
		}
		authCount++
	}

	if workloadIdentity.ManagedIdentityClientIdReference != nil {
		if strings.EqualFold(os.Getenv(WorkloadIdentityDisableGlobalServiceAccount), "true") {
			return loader.NewArgumentError("auth.workloadIdentity.managedIdentityClientIdReference", fmt.Errorf("'managedIdentityClientIdReference' is not allowed since global service account is disabled"))
		}
		authCount++
	}

	if workloadIdentity.ServiceAccountName != nil {
		authCount++
	}

	if authCount == 0 {
		return loader.NewArgumentError("auth.workloadIdentity", fmt.Errorf("setting one of 'managedIdentityClientId', 'managedIdentityClientIdReference' or 'serviceAccountName' field is required"))
	}

	if authCount > 1 {
		return loader.NewArgumentError("auth.workloadIdentity", fmt.Errorf("setting only one of 'managedIdentityClientId', 'managedIdentityClientIdReference' or 'serviceAccountName' field is allowed"))
	}

	if workloadIdentity.ManagedIdentityClientId != nil {
		_, err := uuid.Parse(*workloadIdentity.ManagedIdentityClientId)
		if err != nil {
			return loader.NewArgumentError("auth.workloadIdentity.managedIdentityClientId", fmt.Errorf("managedIdentityClientId %q in auth.workloadIdentity is not a valid uuid", *workloadIdentity.ManagedIdentityClientId))
		}
	}

	return nil
}

func verifySelectorObject(selector acpv1.Selector) error {
	if selector.KeyFilter == nil && selector.SnapshotName == nil {
		return fmt.Errorf("one of keyFilter and snapshotName field must be set")
	}

	if selector.SnapshotName != nil {
		if selector.KeyFilter != nil {
			return fmt.Errorf("set both keyFilter and snapshotName in one selector causes ambiguity, only one of them should be set")
		}
		if selector.LabelFilter != nil {
			return fmt.Errorf("labelFilter is not allowed when snapshotName is set")
		}
	}

	if selector.LabelFilter != nil && hasNonEscapedValueInLabel(*selector.LabelFilter) {
		return fmt.Errorf("non-escaped reserved wildcard character '*' and multiple labels separator ',' are not supported in label filters")
	}

	return nil
}
