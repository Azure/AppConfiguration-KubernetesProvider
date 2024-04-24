// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	acpv1 "azappconfig/provider/api/v1"
	"azappconfig/provider/internal/loader"
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AppConfigurationProviderProcessor struct {
	Context                 context.Context
	Retriever               *loader.ConfigurationSettingsRetriever
	Provider                *acpv1.AzureAppConfigurationProvider
	Settings                *loader.TargetKeyValueSettings
	ShouldReconcile         bool
	ReconciliationState     *ReconciliationState
	CurrentTime             metav1.Time
	RefreshOptions          *RefreshOptions
	SecretReferenceResolver loader.SecretReferenceResolver
}

type RefreshOptions struct {
	sentinelBasedRefreshEnabled   bool
	sentinelChanged               bool
	secretReferenceRefreshEnabled bool
	secretReferenceRefreshNeeded  bool
	featureFlagRefreshEnabled     bool
	featureFlagRefreshNeeded      bool
	ConfigMapSettingPopulated     bool
	SecretSettingPopulated        bool
	updatedSentinelETags          map[acpv1.Sentinel]*azcore.ETag
}

func (processor *AppConfigurationProviderProcessor) PopulateSettings(existingConfigMap *corev1.ConfigMap, existingSecrets map[string]corev1.Secret) error {
	if processor.ShouldReconcile = processor.shouldReconcile(existingConfigMap, existingSecrets); processor.ShouldReconcile {
		if err := processor.processFullReconciliation(); err != nil {
			return err
		}
	}

	if err := processor.processFeatureFlagRefresh(existingConfigMap); err != nil {
		return err
	}

	if err := processor.processKeyValueRefresh(existingConfigMap); err != nil {
		return err
	}

	if err := processor.processSecretReferenceRefresh(existingSecrets); err != nil {
		return err
	}

	return nil
}

func (processor *AppConfigurationProviderProcessor) processFullReconciliation() error {
	updatedSettings, err := (*processor.Retriever).CreateTargetSettings(processor.Context, processor.SecretReferenceResolver)
	if err != nil {
		return err
	}
	processor.Settings = updatedSettings
	processor.ReconciliationState.ExistingSecretReferences = updatedSettings.SecretReferences
	processor.RefreshOptions.ConfigMapSettingPopulated = true
	if processor.Provider.Spec.Secret != nil {
		processor.RefreshOptions.SecretSettingPopulated = true
	}

	return nil
}

func (processor *AppConfigurationProviderProcessor) processFeatureFlagRefresh(existingConfigMap *corev1.ConfigMap) error {
	provider := *processor.Provider
	reconcileState := processor.ReconciliationState
	var err error
	// Check if the feature flag dynamic feature if enabled
	if provider.Spec.FeatureFlag != nil &&
		provider.Spec.FeatureFlag.Refresh != nil &&
		provider.Spec.FeatureFlag.Refresh.Enabled {
		processor.RefreshOptions.featureFlagRefreshEnabled = true
	} else {
		reconcileState.NextFeatureFlagRefreshReconcileTime = metav1.Time{}
		return nil
	}

	refreshInterval, _ := time.ParseDuration(provider.Spec.FeatureFlag.Refresh.Interval)
	nextFeatureFlagRefreshReconcileTime := metav1.Time{Time: processor.CurrentTime.Add(refreshInterval)}
	if processor.ShouldReconcile {
		reconcileState.NextFeatureFlagRefreshReconcileTime = nextFeatureFlagRefreshReconcileTime
		return nil
	}

	if !processor.CurrentTime.After(reconcileState.NextFeatureFlagRefreshReconcileTime.Time) {
		return nil
	}

	processor.RefreshOptions.featureFlagRefreshNeeded = true
	featureFlagRefreshedSettings, err := (*processor.Retriever).RefreshFeatureFlagSettings(processor.Context, &existingConfigMap.Data)
	if err != nil {
		return err
	}
	processor.Settings = featureFlagRefreshedSettings
	processor.RefreshOptions.ConfigMapSettingPopulated = true
	// Update next refresh time only if settings updated successfully
	reconcileState.NextFeatureFlagRefreshReconcileTime = nextFeatureFlagRefreshReconcileTime

	return nil
}

func (processor *AppConfigurationProviderProcessor) processKeyValueRefresh(existingConfigMap *corev1.ConfigMap) error {
	provider := processor.Provider
	reconcileState := processor.ReconciliationState
	var err error
	// Check if the sentinel based refresh is enabled
	if provider.Spec.Configuration.Refresh != nil &&
		provider.Spec.Configuration.Refresh.Enabled {
		processor.RefreshOptions.sentinelBasedRefreshEnabled = true
	} else {
		reconcileState.NextSentinelBasedRefreshReconcileTime = metav1.Time{}
		return nil
	}

	refreshInterval, _ := time.ParseDuration(provider.Spec.Configuration.Refresh.Interval)
	nextSentinelBasedRefreshReconcileTime := metav1.Time{Time: processor.CurrentTime.Add(refreshInterval)}
	if processor.ShouldReconcile {
		reconcileState.NextSentinelBasedRefreshReconcileTime = nextSentinelBasedRefreshReconcileTime
		return nil
	}

	if !processor.CurrentTime.After(reconcileState.NextSentinelBasedRefreshReconcileTime.Time) {
		return nil
	}

	if processor.RefreshOptions.sentinelChanged, processor.RefreshOptions.updatedSentinelETags, err = (*processor.Retriever).CheckAndRefreshSentinels(processor.Context, processor.Provider, reconcileState.SentinelETags); err != nil {
		return err
	}

	if !processor.RefreshOptions.sentinelChanged {
		reconcileState.NextSentinelBasedRefreshReconcileTime = nextSentinelBasedRefreshReconcileTime
		return nil
	}
	// Get the latest key value settings
	existingConfigMapSettings := &existingConfigMap.Data
	if processor.Settings.ConfigMapSettings != nil {
		existingConfigMapSettings = &processor.Settings.ConfigMapSettings
	}
	keyValueRefreshedSettings, err := (*processor.Retriever).RefreshKeyValueSettings(processor.Context, existingConfigMapSettings, processor.SecretReferenceResolver)
	if err != nil {
		return err
	}

	processor.Settings = keyValueRefreshedSettings
	reconcileState.ExistingSecretReferences = keyValueRefreshedSettings.SecretReferences
	processor.RefreshOptions.ConfigMapSettingPopulated = true
	if processor.Provider.Spec.Secret != nil {
		processor.RefreshOptions.SecretSettingPopulated = true
	}
	// Update next refresh time only if settings updated successfully
	reconcileState.NextSentinelBasedRefreshReconcileTime = nextSentinelBasedRefreshReconcileTime

	return nil
}

func (processor *AppConfigurationProviderProcessor) processSecretReferenceRefresh(existingSecrets map[string]corev1.Secret) error {
	provider := processor.Provider
	reconcileState := processor.ReconciliationState
	// Check if the key vault dynamic feature if enabled
	if provider.Spec.Secret != nil &&
		provider.Spec.Secret.Refresh != nil &&
		provider.Spec.Secret.Refresh.Enabled {
		processor.RefreshOptions.secretReferenceRefreshEnabled = true
	}

	if !processor.RefreshOptions.secretReferenceRefreshEnabled {
		reconcileState.NextSecretReferenceRefreshReconcileTime = metav1.Time{}
		return nil
	}

	if !processor.CurrentTime.After(reconcileState.NextSecretReferenceRefreshReconcileTime.Time) {
		return nil
	}

	processor.RefreshOptions.secretReferenceRefreshNeeded = true
	keyVaultRefreshInterval, _ := time.ParseDuration(provider.Spec.Secret.Refresh.Interval)
	nextSecretReferenceRefreshReconcileTime := metav1.Time{Time: processor.CurrentTime.Add(keyVaultRefreshInterval)}
	// When SecretSettingPopulated means ProcessFullReconciliation or ProcessKeyValueRefresh has executed, update next refresh time and return
	if processor.RefreshOptions.SecretSettingPopulated {
		reconcileState.NextSecretReferenceRefreshReconcileTime = nextSecretReferenceRefreshReconcileTime
		return nil
	}

	// Only resolve the secret references that not specified the secret version
	secretReferencesToSolve := make(map[string]*loader.TargetSecretReference)
	for secretName, reference := range reconcileState.ExistingSecretReferences {
		for key, uriSegment := range reference.UriSegments {
			if uriSegment.SecretVersion == "" {
				if secretReferencesToSolve[secretName] == nil {
					secretReferencesToSolve[secretName] = &loader.TargetSecretReference{
						Type:        reference.Type,
						UriSegments: make(map[string]loader.KeyVaultSecretUriSegment),
					}
				}
				secretReferencesToSolve[secretName].UriSegments[key] = uriSegment
			}
		}
	}

	resolvedSecrets, err := (*processor.Retriever).ResolveSecretReferences(processor.Context, secretReferencesToSolve, processor.SecretReferenceResolver)
	if err != nil {
		return err
	}

	for secretName, resolvedSecret := range resolvedSecrets {
		existingSecret, ok := existingSecrets[secretName]
		if ok {
			maps.Copy(existingSecret.Data, resolvedSecret.Data)
		}
	}
	processor.Settings.SecretSettings = existingSecrets
	processor.RefreshOptions.SecretSettingPopulated = true

	// Update next refresh time only if settings updated successfully
	reconcileState.NextSecretReferenceRefreshReconcileTime = nextSecretReferenceRefreshReconcileTime

	return nil
}

func (processor *AppConfigurationProviderProcessor) shouldReconcile(
	existingConfigMap *corev1.ConfigMap,
	existingSecrets map[string]corev1.Secret) bool {

	if processor.Provider.Generation != processor.ReconciliationState.Generation {
		// If the provider is updated, we need to reconcile anyway
		return true
	}

	if processor.ReconciliationState.ConfigMapResourceVersion == nil ||
		*processor.ReconciliationState.ConfigMapResourceVersion != existingConfigMap.ResourceVersion {
		// If the ConfigMap is removed or updated, we need to reconcile anyway
		return true
	}

	if processor.Provider.Spec.Secret == nil {
		return false
	}

	if len(processor.ReconciliationState.ExistingSecretReferences) == 0 ||
		len(processor.ReconciliationState.ExistingSecretReferences) != len(existingSecrets) {
		return true
	}

	for name, secret := range existingSecrets {
		if processor.ReconciliationState.ExistingSecretReferences[name].SecretResourceVersion != secret.ResourceVersion {
			return true
		}
	}

	return false
}

func (processor *AppConfigurationProviderProcessor) Finish() (ctrl.Result, error) {
	processor.ReconciliationState.Generation = processor.Provider.Generation
	if !processor.RefreshOptions.secretReferenceRefreshEnabled &&
		!processor.RefreshOptions.sentinelBasedRefreshEnabled &&
		!processor.RefreshOptions.featureFlagRefreshEnabled {
		// Do nothing, just complete the reconcile
		klog.V(1).Infof("Complete reconcile AzureAppConfigurationProvider %q in %q namespace", processor.Provider.Name, processor.Provider.Namespace)
		return reconcile.Result{}, nil
	} else {
		// Update the sentinel ETags and last sentinel refresh time
		if processor.RefreshOptions.sentinelChanged {
			processor.ReconciliationState.SentinelETags = processor.RefreshOptions.updatedSentinelETags
			processor.Provider.Status.RefreshStatus.LastSentinelBasedRefreshTime = processor.CurrentTime
		}
		// Update provider last key vault refresh time
		if processor.RefreshOptions.secretReferenceRefreshNeeded {
			processor.Provider.Status.RefreshStatus.LastKeyVaultReferenceRefreshTime = processor.CurrentTime
		}
		// Update provider last feature flag refresh time
		if processor.RefreshOptions.featureFlagRefreshNeeded {
			processor.Provider.Status.RefreshStatus.LastFeatureFlagRefreshTime = processor.CurrentTime
		}
		// At least one dynamic feature is enabled, requeueAfterInterval need be recalculated
		requeueAfterInterval := processor.calculateRequeueAfterInterval()
		klog.V(3).Infof("Revisit AzureAppConfigurationProvider %q in %q namespace after %s",
			processor.Provider.Name, processor.Provider.Namespace, requeueAfterInterval.String())
		return reconcile.Result{Requeue: true, RequeueAfter: requeueAfterInterval}, nil
	}
}

func NewRefreshOptions() *RefreshOptions {
	return &RefreshOptions{
		sentinelBasedRefreshEnabled:   false,
		sentinelChanged:               false,
		secretReferenceRefreshEnabled: false,
		secretReferenceRefreshNeeded:  false,
		featureFlagRefreshEnabled:     false,
		featureFlagRefreshNeeded:      false,
		ConfigMapSettingPopulated:     false,
		SecretSettingPopulated:        false,
		updatedSentinelETags:          make(map[acpv1.Sentinel]*azcore.ETag),
	}
}

func (processor *AppConfigurationProviderProcessor) calculateRequeueAfterInterval() time.Duration {
	reconcileState := processor.ReconciliationState
	nextRefreshTimeList := []metav1.Time{reconcileState.NextSentinelBasedRefreshReconcileTime,
		reconcileState.NextSecretReferenceRefreshReconcileTime, reconcileState.NextFeatureFlagRefreshReconcileTime}

	var nextRequeueTime metav1.Time
	for _, time := range nextRefreshTimeList {
		if !time.IsZero() && (nextRequeueTime.IsZero() || time.Before(&nextRequeueTime)) {
			nextRequeueTime = time
		}
	}

	requeueAfterInterval := nextRequeueTime.Sub(metav1.Now().Time)
	// If the requeueAfterInterval is smaller than one sencond, reset the value to one second
	if requeueAfterInterval < time.Second {
		return time.Second
	}

	return requeueAfterInterval
}
