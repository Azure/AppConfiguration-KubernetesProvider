//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationFeatureFlagOptions) DeepCopyInto(out *AzureAppConfigurationFeatureFlagOptions) {
	*out = *in
	if in.Selectors != nil {
		in, out := &in.Selectors, &out.Selectors
		*out = make([]KeyLabelSelector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Refresh != nil {
		in, out := &in.Refresh, &out.Refresh
		*out = new(RefreshSettings)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationFeatureFlagOptions.
func (in *AzureAppConfigurationFeatureFlagOptions) DeepCopy() *AzureAppConfigurationFeatureFlagOptions {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationFeatureFlagOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationKeyValueOptions) DeepCopyInto(out *AzureAppConfigurationKeyValueOptions) {
	*out = *in
	if in.Selectors != nil {
		in, out := &in.Selectors, &out.Selectors
		*out = make([]KeyLabelSelector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TrimKeyPrefixes != nil {
		in, out := &in.TrimKeyPrefixes, &out.TrimKeyPrefixes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Refresh != nil {
		in, out := &in.Refresh, &out.Refresh
		*out = new(DynamicConfigurationRefreshParameters)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationKeyValueOptions.
func (in *AzureAppConfigurationKeyValueOptions) DeepCopy() *AzureAppConfigurationKeyValueOptions {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationKeyValueOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationProvider) DeepCopyInto(out *AzureAppConfigurationProvider) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationProvider.
func (in *AzureAppConfigurationProvider) DeepCopy() *AzureAppConfigurationProvider {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureAppConfigurationProvider) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationProviderAuth) DeepCopyInto(out *AzureAppConfigurationProviderAuth) {
	*out = *in
	if in.ManagedIdentityClientId != nil {
		in, out := &in.ManagedIdentityClientId, &out.ManagedIdentityClientId
		*out = new(string)
		**out = **in
	}
	if in.ServicePrincipalReference != nil {
		in, out := &in.ServicePrincipalReference, &out.ServicePrincipalReference
		*out = new(string)
		**out = **in
	}
	if in.WorkloadIdentity != nil {
		in, out := &in.WorkloadIdentity, &out.WorkloadIdentity
		*out = new(WorkloadIdentityParameters)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationProviderAuth.
func (in *AzureAppConfigurationProviderAuth) DeepCopy() *AzureAppConfigurationProviderAuth {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationProviderAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationProviderList) DeepCopyInto(out *AzureAppConfigurationProviderList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AzureAppConfigurationProvider, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationProviderList.
func (in *AzureAppConfigurationProviderList) DeepCopy() *AzureAppConfigurationProviderList {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationProviderList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureAppConfigurationProviderList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationProviderSpec) DeepCopyInto(out *AzureAppConfigurationProviderSpec) {
	*out = *in
	if in.Endpoint != nil {
		in, out := &in.Endpoint, &out.Endpoint
		*out = new(string)
		**out = **in
	}
	if in.ConnectionStringReference != nil {
		in, out := &in.ConnectionStringReference, &out.ConnectionStringReference
		*out = new(string)
		**out = **in
	}
	in.Target.DeepCopyInto(&out.Target)
	if in.Auth != nil {
		in, out := &in.Auth, &out.Auth
		*out = new(AzureAppConfigurationProviderAuth)
		(*in).DeepCopyInto(*out)
	}
	in.Configuration.DeepCopyInto(&out.Configuration)
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(AzureKeyVaultReference)
		(*in).DeepCopyInto(*out)
	}
	if in.FeatureFlag != nil {
		in, out := &in.FeatureFlag, &out.FeatureFlag
		*out = new(AzureAppConfigurationFeatureFlagOptions)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationProviderSpec.
func (in *AzureAppConfigurationProviderSpec) DeepCopy() *AzureAppConfigurationProviderSpec {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationProviderSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureAppConfigurationProviderStatus) DeepCopyInto(out *AzureAppConfigurationProviderStatus) {
	*out = *in
	in.LastReconcileTime.DeepCopyInto(&out.LastReconcileTime)
	in.LastSyncTime.DeepCopyInto(&out.LastSyncTime)
	in.RefreshStatus.DeepCopyInto(&out.RefreshStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureAppConfigurationProviderStatus.
func (in *AzureAppConfigurationProviderStatus) DeepCopy() *AzureAppConfigurationProviderStatus {
	if in == nil {
		return nil
	}
	out := new(AzureAppConfigurationProviderStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureKeyVaultAuth) DeepCopyInto(out *AzureKeyVaultAuth) {
	*out = *in
	if in.AzureAppConfigurationProviderAuth != nil {
		in, out := &in.AzureAppConfigurationProviderAuth, &out.AzureAppConfigurationProviderAuth
		*out = new(AzureAppConfigurationProviderAuth)
		(*in).DeepCopyInto(*out)
	}
	if in.KeyVaults != nil {
		in, out := &in.KeyVaults, &out.KeyVaults
		*out = make([]AzureKeyVaultPerVaultAuth, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureKeyVaultAuth.
func (in *AzureKeyVaultAuth) DeepCopy() *AzureKeyVaultAuth {
	if in == nil {
		return nil
	}
	out := new(AzureKeyVaultAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureKeyVaultPerVaultAuth) DeepCopyInto(out *AzureKeyVaultPerVaultAuth) {
	*out = *in
	if in.AzureAppConfigurationProviderAuth != nil {
		in, out := &in.AzureAppConfigurationProviderAuth, &out.AzureAppConfigurationProviderAuth
		*out = new(AzureAppConfigurationProviderAuth)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureKeyVaultPerVaultAuth.
func (in *AzureKeyVaultPerVaultAuth) DeepCopy() *AzureKeyVaultPerVaultAuth {
	if in == nil {
		return nil
	}
	out := new(AzureKeyVaultPerVaultAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureKeyVaultReference) DeepCopyInto(out *AzureKeyVaultReference) {
	*out = *in
	out.Target = in.Target
	if in.Auth != nil {
		in, out := &in.Auth, &out.Auth
		*out = new(AzureKeyVaultAuth)
		(*in).DeepCopyInto(*out)
	}
	if in.Refresh != nil {
		in, out := &in.Refresh, &out.Refresh
		*out = new(RefreshSettings)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureKeyVaultReference.
func (in *AzureKeyVaultReference) DeepCopy() *AzureKeyVaultReference {
	if in == nil {
		return nil
	}
	out := new(AzureKeyVaultReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigMapDataOptions) DeepCopyInto(out *ConfigMapDataOptions) {
	*out = *in
	if in.Separator != nil {
		in, out := &in.Separator, &out.Separator
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigMapDataOptions.
func (in *ConfigMapDataOptions) DeepCopy() *ConfigMapDataOptions {
	if in == nil {
		return nil
	}
	out := new(ConfigMapDataOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigurationGenerationParameters) DeepCopyInto(out *ConfigurationGenerationParameters) {
	*out = *in
	if in.ConfigMapData != nil {
		in, out := &in.ConfigMapData, &out.ConfigMapData
		*out = new(ConfigMapDataOptions)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigurationGenerationParameters.
func (in *ConfigurationGenerationParameters) DeepCopy() *ConfigurationGenerationParameters {
	if in == nil {
		return nil
	}
	out := new(ConfigurationGenerationParameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicConfigurationRefreshParameters) DeepCopyInto(out *DynamicConfigurationRefreshParameters) {
	*out = *in
	if in.Monitoring != nil {
		in, out := &in.Monitoring, &out.Monitoring
		*out = new(RefreshMonitoring)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicConfigurationRefreshParameters.
func (in *DynamicConfigurationRefreshParameters) DeepCopy() *DynamicConfigurationRefreshParameters {
	if in == nil {
		return nil
	}
	out := new(DynamicConfigurationRefreshParameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KeyLabelSelector) DeepCopyInto(out *KeyLabelSelector) {
	*out = *in
	if in.LabelFilter != nil {
		in, out := &in.LabelFilter, &out.LabelFilter
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KeyLabelSelector.
func (in *KeyLabelSelector) DeepCopy() *KeyLabelSelector {
	if in == nil {
		return nil
	}
	out := new(KeyLabelSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagedIdentityReferenceParameters) DeepCopyInto(out *ManagedIdentityReferenceParameters) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagedIdentityReferenceParameters.
func (in *ManagedIdentityReferenceParameters) DeepCopy() *ManagedIdentityReferenceParameters {
	if in == nil {
		return nil
	}
	out := new(ManagedIdentityReferenceParameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RefreshMonitoring) DeepCopyInto(out *RefreshMonitoring) {
	*out = *in
	if in.Sentinels != nil {
		in, out := &in.Sentinels, &out.Sentinels
		*out = make([]Sentinel, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RefreshMonitoring.
func (in *RefreshMonitoring) DeepCopy() *RefreshMonitoring {
	if in == nil {
		return nil
	}
	out := new(RefreshMonitoring)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RefreshSettings) DeepCopyInto(out *RefreshSettings) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RefreshSettings.
func (in *RefreshSettings) DeepCopy() *RefreshSettings {
	if in == nil {
		return nil
	}
	out := new(RefreshSettings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RefreshStatus) DeepCopyInto(out *RefreshStatus) {
	*out = *in
	in.LastKeyVaultReferenceRefreshTime.DeepCopyInto(&out.LastKeyVaultReferenceRefreshTime)
	in.LastSentinelBasedRefreshTime.DeepCopyInto(&out.LastSentinelBasedRefreshTime)
	in.LastFeatureFlagRefreshTime.DeepCopyInto(&out.LastFeatureFlagRefreshTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RefreshStatus.
func (in *RefreshStatus) DeepCopy() *RefreshStatus {
	if in == nil {
		return nil
	}
	out := new(RefreshStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretGenerationParameters) DeepCopyInto(out *SecretGenerationParameters) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretGenerationParameters.
func (in *SecretGenerationParameters) DeepCopy() *SecretGenerationParameters {
	if in == nil {
		return nil
	}
	out := new(SecretGenerationParameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Sentinel) DeepCopyInto(out *Sentinel) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Sentinel.
func (in *Sentinel) DeepCopy() *Sentinel {
	if in == nil {
		return nil
	}
	out := new(Sentinel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadIdentityParameters) DeepCopyInto(out *WorkloadIdentityParameters) {
	*out = *in
	if in.ManagedIdentityClientId != nil {
		in, out := &in.ManagedIdentityClientId, &out.ManagedIdentityClientId
		*out = new(string)
		**out = **in
	}
	if in.ManagedIdentityClientIdReference != nil {
		in, out := &in.ManagedIdentityClientIdReference, &out.ManagedIdentityClientIdReference
		*out = new(ManagedIdentityReferenceParameters)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentityParameters.
func (in *WorkloadIdentityParameters) DeepCopy() *WorkloadIdentityParameters {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentityParameters)
	in.DeepCopyInto(out)
	return out
}
