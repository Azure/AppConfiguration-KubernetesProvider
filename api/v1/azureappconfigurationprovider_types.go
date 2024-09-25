// Portions Copyright (c) Microsoft Corporation.

/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make manifests" to regenerate code after modifying this file

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AzureAppConfigurationProviderSpec defines the desired state of AzureAppConfigurationProvider
type AzureAppConfigurationProviderSpec struct {
	// The endpoint url of AppConfiguration which should sync the configuration key-values from.
	// +kubebuilder:validation:Format=uri
	Endpoint *string `json:"endpoint,omitempty"`
	// +kubebuilder:default=true
	ReplicaDiscoveryEnabled bool `json:"replicaDiscoveryEnabled,omitempty"`
	// +kubebuilder:default=false
	LoadBalancingEnabled      bool                                     `json:"loadBalancingEnabled,omitempty"`
	ConnectionStringReference *string                                  `json:"connectionStringReference,omitempty"`
	Target                    ConfigurationGenerationParameters        `json:"target"`
	Auth                      *AzureAppConfigurationProviderAuth       `json:"auth,omitempty"`
	Configuration             AzureAppConfigurationKeyValueOptions     `json:"configuration,omitempty"`
	Secret                    *SecretReference                         `json:"secret,omitempty"`
	FeatureFlag               *AzureAppConfigurationFeatureFlagOptions `json:"featureFlag,omitempty"`
}

// AzureAppConfigurationProviderStatus defines the observed state of AzureAppConfigurationProvider
type AzureAppConfigurationProviderStatus struct {
	LastReconcileTime metav1.Time               `json:"lastReconcileTime,omitempty"`
	LastSyncTime      metav1.Time               `json:"lastSyncTime,omitempty"`
	Phase             AppConfigurationSyncPhase `json:"phase,omitempty"`
	Message           string                    `json:"message,omitempty"`
	RefreshStatus     RefreshStatus             `json:"refreshStatus,omitempty"`
}

// RefreshStatus defines last refresh time of configmap and secret when dynamic feature is enabled
type RefreshStatus struct {
	LastKeyVaultReferenceRefreshTime metav1.Time `json:"lastKeyVaultReferenceRefreshTime,omitempty"`
	LastKeyValueRefreshTime          metav1.Time `json:"lastKeyValueRefreshTime,omitempty"`
	LastFeatureFlagRefreshTime       metav1.Time `json:"lastFeatureFlagRefreshTime,omitempty"`
}

// ConfigurationGenerationParameters defines the name of target ConfigMap
type ConfigurationGenerationParameters struct {
	// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string                `json:"configMapName"`
	ConfigMapData *ConfigMapDataOptions `json:"configMapData,omitempty"`
}

// AzureAppConfigurationKeyValueOptions defines the options of fetching key-values from AppConfiguration.
type AzureAppConfigurationKeyValueOptions struct {
	Selectors       []Selector                             `json:"selectors,omitempty"`
	TrimKeyPrefixes []string                               `json:"trimKeyPrefixes,omitempty"`
	Refresh         *DynamicConfigurationRefreshParameters `json:"refresh,omitempty"`
}

// AzureAppConfigurationFeatureFlagOptions defines the options of fetching feature flags from AppConfiguration.
type AzureAppConfigurationFeatureFlagOptions struct {
	Selectors []Selector                  `json:"selectors,omitempty"`
	Refresh   *FeatureFlagRefreshSettings `json:"refresh,omitempty"`
}

// KeyLabelSelector defines the filters when fetching the data from Azure AppConfiguration
type Selector struct {
	KeyFilter    *string `json:"keyFilter,omitempty"`
	LabelFilter  *string `json:"labelFilter,omitempty"`
	SnapshotName *string `json:"snapshotName,omitempty"`
}

// Defines the settings for dynamic configuration.
type DynamicConfigurationRefreshParameters struct {
	Monitoring *RefreshMonitoring `json:"monitoring,omitempty"`
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
}

// Defines the settings for change monitoring.
type RefreshMonitoring struct {
	// +kubebuilder:validation:MinItems=1
	Sentinels []Sentinel `json:"keyValues"`
}

// Defines the keyValues to be watched.
type Sentinel struct {
	Key string `json:"key"`
	// +kubebuilder:default="\x00"
	Label *string `json:"label,omitempty"`
}

// ConfigMapDataOptions defines the options of generating ConfigMap data
type ConfigMapDataOptions struct {
	// +kubebuilder:default="default"
	Type ConfigMapDataType `json:"type,omitempty"`
	Key  string            `json:"key,omitempty"`
	// The delimiter that is used to output the ConfigMap data in hierarchical format when the type is set to json or yaml.
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:MinLength=1
	Separator *string `json:"separator,omitempty"`
}

// AzureAppConfigurationProviderAuth defines the authentication type used to connect Azure AppConfiguration
type AzureAppConfigurationProviderAuth struct {
	// ClientId of the Managed Identity
	ManagedIdentityClientId *string `json:"managedIdentityClientId,omitempty"`
	// Secret reference for Service Principle
	ServicePrincipalReference *string `json:"servicePrincipalReference,omitempty"`
	// Workload Identity
	WorkloadIdentity *WorkloadIdentityParameters `json:"workloadIdentity,omitempty"`
}

// WorkloadIdentityParameters defines the parameters for workload identity
type WorkloadIdentityParameters struct {
	ManagedIdentityClientId          *string                             `json:"managedIdentityClientId,omitempty"`
	ManagedIdentityClientIdReference *ManagedIdentityReferenceParameters `json:"managedIdentityClientIdReference,omitempty"`
	ServiceAccountName               *string                             `json:"serviceAccountName,omitempty"`
}

// ManagedIdentityReferenceParameters defines the parameters for configmap reference
type ManagedIdentityReferenceParameters struct {
	// ConfigMap contains the managed identity client id
	ConfigMap string `json:"configMap"`
	// Key of the managed identity client id
	Key string `json:"key"`
}

// SecretReference defines the settings for resolving secret reference type items
type SecretReference struct {
	Target  SecretGenerationParameters `json:"target"`
	Auth    *AzureKeyVaultAuth         `json:"auth,omitempty"`
	Refresh *RefreshSettings           `json:"refresh,omitempty"`
}

// Defines the settings for refresh.
type RefreshSettings struct {
	// +kubebuilder:validation:Format=duration
	Interval string `json:"interval"`
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
}

// Defines the settings for feature flag refresh.
type FeatureFlagRefreshSettings struct {
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:default="30s"
	Interval string `json:"interval"`
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
}

type AzureKeyVaultAuth struct {
	*AzureAppConfigurationProviderAuth `json:",inline,omitempty"`
	KeyVaults                          []AzureKeyVaultPerVaultAuth `json:"keyVaults,omitempty"`
}

// AzureKeyVaultPerVaultAuth defines the authentication type used to Azure KeyVault resolve KeyVaultReference
type AzureKeyVaultPerVaultAuth struct {
	// The uri of KeyVault which should sync the secret reference item from
	// +kubebuilder:validation:Format=uri
	Uri                                string `json:"uri"`
	*AzureAppConfigurationProviderAuth `json:",inline"`
}

// SecretGenerationParameters defines the name of target Secret
type SecretGenerationParameters struct {
	// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
}

// +kubebuilder:validation:Enum=default;json;yaml;properties
type ConfigMapDataType string

const (
	Default    ConfigMapDataType = "default"
	Properties ConfigMapDataType = "properties"
	Yaml       ConfigMapDataType = "yaml"
	Json       ConfigMapDataType = "json"
)

type AppConfigurationSyncPhase string

const (
	PhasePending      AppConfigurationSyncPhase = "Pending"
	PhaseRunning      AppConfigurationSyncPhase = "Running"
	PhaseComplete     AppConfigurationSyncPhase = "Complete"
	PhaseFailed       AppConfigurationSyncPhase = "Failed"
	PhaseUpdateFailed AppConfigurationSyncPhase = "UpdateFailed"
)

const (
	SyncCompleteMessage = "Complete sync key-values from App Configuration to target ConfigMap or Secret."
	CreateFailMessage   = "Fail to create key-values from App Configuration to target ConfigMap or Secret."
	UpdateFailMessage   = "Fail to update present key-values from App Configuration to target ConfigMap or Secret."
	SyncRunningMessage  = "Azure App Configuration provider is syncing the key-values to cluster."
)

//Makers for teaching kubebuilder how generate the CRD, see https://book.kubebuilder.io/reference/markers/crd.html for detail
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AzureAppConfigurationProvider is the Schema for the AzureAppConfigurationProviders API
type AzureAppConfigurationProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureAppConfigurationProviderSpec   `json:"spec,omitempty"`
	Status AzureAppConfigurationProviderStatus `json:"status,omitempty"`
}

//Makers for teaching kubebuilder how generate the CRD, see https://book.kubebuilder.io/reference/markers/crd.html for detail
//+kubebuilder:object:root=true

// AzureAppConfigurationProviderList contains a list of AzureAppConfigurationProvider
type AzureAppConfigurationProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureAppConfigurationProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureAppConfigurationProvider{}, &AzureAppConfigurationProviderList{})
}
