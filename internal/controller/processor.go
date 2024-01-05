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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AppConfigurationProviderProcessor struct {
	Context                context.Context
	Retriever              *loader.ConfigurationSettingsRetriever
	Provider               *acpv1.AzureAppConfigurationProvider
	Settings               *loader.TargetKeyValueSettings
	ShouldReconcile        bool
	ReconciliationState    map[types.NamespacedName]*ReconciliationState
	CurrentTime            metav1.Time
	NamespacedName         types.NamespacedName
	RefreshOptions         *RefreshOptions
	ResolveSecretReference loader.ResolveSecretReference
}

type RefreshOptions struct {
	sentinelBasedRefreshEnabled bool
	sentinelChanged             bool
	keyVaultRefreshEnabled      bool
	keyVaultRefreshNeeded       bool
	updatedSentinelETags        map[acpv1.Sentinel]*azcore.ETag
	ConfigMapSettingPopulated   bool
	SecretSettingPopulated      bool
}

func (processor *AppConfigurationProviderProcessor) PopulateSettings(existingSecret *corev1.Secret) error {
	if err := processor.ProcessFullReconciliation(); err != nil {
		return err
	}

	if err := processor.ProcessKeyValueRefresh(); err != nil {
		return err
	}

	if err := processor.ProcessKeyVaultReferenceRefresh(existingSecret); err != nil {
		return err
	}

	return nil
}

func (processor *AppConfigurationProviderProcessor) ProcessFullReconciliation() error {
	if processor.ShouldReconcile {
		updatedSettings, err := (*processor.Retriever).CreateKeyValueSettings(processor.Context, processor.ResolveSecretReference)
		if err != nil {
			return err
		}
		processor.Settings = updatedSettings
		processor.ReconciliationState[processor.NamespacedName].CachedSecretReferences = updatedSettings.KeyVaultReferencesToCache
		processor.RefreshOptions.ConfigMapSettingPopulated = true
		if processor.Provider.Spec.Secret != nil {
			processor.RefreshOptions.SecretSettingPopulated = true
		}
	}

	return nil
}

func (processor *AppConfigurationProviderProcessor) ProcessKeyValueRefresh() error {
	provider := *processor.Provider
	reconcileState := processor.ReconciliationState[processor.NamespacedName]
	currentTime := processor.CurrentTime
	var err error
	// Check if the sentinel based refresh is enabled
	if provider.Spec.Configuration.Refresh != nil && provider.Spec.Configuration.Refresh.Enabled {
		processor.RefreshOptions.sentinelBasedRefreshEnabled = true
	} else {
		reconcileState.NextSentinelBasedRefreshReconcileTime = metav1.Time{}
		return nil
	}

	refreshInterval, _ := time.ParseDuration(provider.Spec.Configuration.Refresh.Interval)
	nextSentinelBasedRefreshReconcileTime := metav1.Time{Time: currentTime.Add(refreshInterval)}
	if processor.ShouldReconcile {
		reconcileState.NextSentinelBasedRefreshReconcileTime = nextSentinelBasedRefreshReconcileTime
		return nil
	}

	if !currentTime.After(reconcileState.NextSentinelBasedRefreshReconcileTime.Time) {
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
	updatedSettings, err := (*processor.Retriever).CreateKeyValueSettings(processor.Context, processor.ResolveSecretReference)
	if err != nil {
		return err
	}

	processor.Settings = updatedSettings
	reconcileState.CachedSecretReferences = updatedSettings.KeyVaultReferencesToCache
	processor.RefreshOptions.ConfigMapSettingPopulated = true
	if processor.Provider.Spec.Secret != nil {
		processor.RefreshOptions.SecretSettingPopulated = true
	}

	// Update next refresh time only if settings updated successfully
	reconcileState.NextSentinelBasedRefreshReconcileTime = nextSentinelBasedRefreshReconcileTime

	return nil
}

func (processor *AppConfigurationProviderProcessor) ProcessKeyVaultReferenceRefresh(existingSecret *corev1.Secret) error {
	provider := *processor.Provider
	reconcileState := processor.ReconciliationState[processor.NamespacedName]
	currentTime := processor.CurrentTime
	// Check if the key vault dynamic feature if enabled
	if provider.Spec.Secret != nil && provider.Spec.Secret.Refresh != nil && provider.Spec.Secret.Refresh.Enabled {
		processor.RefreshOptions.keyVaultRefreshEnabled = true
	}

	if !processor.RefreshOptions.keyVaultRefreshEnabled {
		reconcileState.NextKeyVaultReferenceRefreshReconcileTime = metav1.Time{}
		reconcileState.CachedSecretReferences = make(map[string]loader.KeyVaultSecretUriSegment)
		return nil
	}

	if !currentTime.After(reconcileState.NextKeyVaultReferenceRefreshReconcileTime.Time) {
		return nil
	}

	processor.RefreshOptions.keyVaultRefreshNeeded = true
	keyVaultRefreshInterval, _ := time.ParseDuration(provider.Spec.Secret.Refresh.Interval)
	nextKeyVaultReferenceRefreshReconcileTime := metav1.Time{Time: currentTime.Add(keyVaultRefreshInterval)}
	// When SecretSettingPopulated means ProcessFullReconciliation or ProcessKeyValueRefresh has executed, update next refresh time and return
	if processor.RefreshOptions.SecretSettingPopulated {
		reconcileState.NextKeyVaultReferenceRefreshReconcileTime = nextKeyVaultReferenceRefreshReconcileTime
		return nil
	}

	resolvedSecretData, err := (*processor.Retriever).ResolveKeyVaultReferences(processor.Context, reconcileState.CachedSecretReferences, processor.ResolveSecretReference)
	if err != nil {
		return err
	}
	maps.Copy(existingSecret.Data, resolvedSecretData)
	processor.Settings.SecretSettings = existingSecret.Data
	processor.RefreshOptions.SecretSettingPopulated = true

	// Update next refresh time only if settings updated successfully
	reconcileState.NextKeyVaultReferenceRefreshReconcileTime = nextKeyVaultReferenceRefreshReconcileTime

	return nil
}

func (processor *AppConfigurationProviderProcessor) Finish() (ctrl.Result, error) {
	processor.ReconciliationState[processor.NamespacedName].Generation = processor.Provider.Generation
	if !processor.RefreshOptions.keyVaultRefreshEnabled && !processor.RefreshOptions.sentinelBasedRefreshEnabled {
		// Do nothing, just complete the reconcile
		klog.V(1).Infof("Complete reconcile AzureAppConfigurationProvider %q in %q namespace", processor.Provider.Name, processor.Provider.Namespace)
		return reconcile.Result{}, nil
	} else {
		// Update the sentinel ETags and last sentinel refresh time
		if processor.RefreshOptions.sentinelChanged {
			processor.ReconciliationState[processor.NamespacedName].SentinelETags = processor.RefreshOptions.updatedSentinelETags
			processor.Provider.Status.RefreshStatus.LastSentinelBasedRefreshTime = processor.CurrentTime
		}
		// Update provider last key vault refresh time
		if processor.RefreshOptions.keyVaultRefreshNeeded {
			processor.Provider.Status.RefreshStatus.LastKeyVaultReferenceRefreshTime = processor.CurrentTime
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
		sentinelBasedRefreshEnabled: false,
		sentinelChanged:             false,
		keyVaultRefreshEnabled:      false,
		keyVaultRefreshNeeded:       false,
		updatedSentinelETags:        make(map[acpv1.Sentinel]*azcore.ETag),
		ConfigMapSettingPopulated:   false,
		SecretSettingPopulated:      false,
	}
}

func (processor *AppConfigurationProviderProcessor) calculateRequeueAfterInterval() time.Duration {
	reconcileState := processor.ReconciliationState[processor.NamespacedName]
	nextRefreshTimeList := []metav1.Time{reconcileState.NextSentinelBasedRefreshReconcileTime, reconcileState.NextKeyVaultReferenceRefreshReconcileTime}

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
