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

package controller

import (
	acpv1 "azappconfig/provider/api/v1"
	"azappconfig/provider/internal/loader"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AzureAppConfigurationProviderReconciler reconciles a AzureAppConfigurationProvider object
type AzureAppConfigurationProviderReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Retriever               loader.ConfigurationSettingsRetriever
	ProvidersReconcileState map[types.NamespacedName]*ReconciliationState
}

type ReconciliationState struct {
	Generation                              int64
	ConfigMapResourceVersion                *string
	SentinelETags                           map[acpv1.Sentinel]*azcore.ETag
	KeyValueETags                           map[acpv1.Selector][]*azcore.ETag
	FeatureFlagETags                        map[acpv1.Selector][]*azcore.ETag
	ExistingSecretReferences                map[string]*loader.TargetSecretReference
	NextKeyValueRefreshReconcileTime        metav1.Time
	NextSecretReferenceRefreshReconcileTime metav1.Time
	NextFeatureFlagRefreshReconcileTime     metav1.Time
	ClientManager                           loader.ClientManager
}

const (
	ProviderName                string        = "AzureAppConfigurationProvider"
	LastReconcileTimeAnnotation string        = "azconfig.io/LastReconcileTime"
	SecretReferenceContentType  string        = "application/vnd.microsoft.appconfig.keyvaultref+json;charset=utf-8"
	FeatureFlagContentType      string        = "application/vnd.microsoft.appconfig.ff+json;charset=utf-8"
	HeaderRetryAfter            string        = "Retry-After"
	RequeueReconcileAfter       time.Duration = time.Second * 30
	RetryAttempt                int           = 3
	DefaultRefreshInterval      time.Duration = time.Second * 30
)

//Markers for teaching kubebuiler how generate the rabc manifests, see https://book.kubebuilder.io/reference/markers/rbac.html for detail
//+kubebuilder:rbac:groups=azconfig.io,resources=azureappconfigurationproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=azconfig.io,resources=azureappconfigurationproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=azconfig.io,resources=azureappconfigurationproviders/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create;update;patch;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;update;delete;patch;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (reconciler *AzureAppConfigurationProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	provider := &acpv1.AzureAppConfigurationProvider{}
	err := reconciler.Get(ctx, req.NamespacedName, provider)
	//Get object, if not exists, exit reconcile
	if err != nil && apierrors.IsNotFound(err) {
		delete(reconciler.ProvidersReconcileState, req.NamespacedName)
		return reconcile.Result{}, nil
	} else if err != nil {
		klog.ErrorS(err, "Fail to get AzureAppConfigurationProvider object.")
		return reconcile.Result{}, err
	}

	/* Patch the object status when finish processing. */
	var patch client.Patch = client.MergeFrom(provider.DeepCopy())
	defer func() {
		retry := RetryAttempt
		patchSuccess := false
		for retry > 0 {
			err = reconciler.Status().Patch(ctx, provider, patch)
			if err != nil {
				retry--
			} else {
				patchSuccess = true
				break
			}
		}
		if !patchSuccess {
			klog.ErrorS(err, "Fail to patch the status of AzureAppConfigurationProvider.")
		}
	}()

	/* Status initialization and resource object verification. */
	if provider.Status.Phase == "" {
		provider.Status.Phase = acpv1.PhasePending
	}

	if provider.Status.Phase == acpv1.PhaseRunning {
		klog.V(3).Infof("The reconcile for AzureAppConfigurationProvider '%s' is running, just exit.", provider.Name)
		return reconcile.Result{}, nil
	}

	provider.Status = newProviderStatus(acpv1.PhaseRunning, acpv1.SyncRunningMessage, provider.Status.LastSyncTime, provider.Status.RefreshStatus)
	klog.V(3).Infof("Start reconcile AzureAppConfigurationProvider %q in %q namespace ", provider.Name, provider.Namespace)

	err = verifyObject(provider.Spec)
	if err != nil {
		reconciler.logAndSetFailStatus(provider, err)
		return reconcile.Result{Requeue: false}, nil
	}

	existingConfigMap := corev1.ConfigMap{}
	isExisting := false
	_, err = reconciler.verifyTargetObjectExistence(ctx, provider, &existingConfigMap)
	if err != nil {
		reconciler.logAndSetFailStatus(provider, err)
		return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, nil
	}

	existingSecrets := make(map[string]corev1.Secret)
	var existingSecret corev1.Secret
	if provider.Spec.Secret != nil {
		existingSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: provider.Spec.Secret.Target.SecretName,
			},
		}
		isExisting, err = reconciler.verifyTargetObjectExistence(ctx, provider, &existingSecret)
		if err != nil {
			reconciler.logAndSetFailStatus(provider, err)
			return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, nil
		}
		if isExisting {
			existingSecrets[provider.Spec.Secret.Target.SecretName] = existingSecret
		}
	}

	if reconciler.ProvidersReconcileState[req.NamespacedName] != nil {
		for name := range reconciler.ProvidersReconcileState[req.NamespacedName].ExistingSecretReferences {
			if _, ok := existingSecrets[name]; !ok {
				existingSecret = corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
				}
				isExisting, err = reconciler.verifyTargetObjectExistence(ctx, provider, &existingSecret)
				if err != nil {
					reconciler.logAndSetFailStatus(provider, err)
					return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, nil
				}
				if isExisting {
					existingSecrets[name] = existingSecret
				}
			}
		}
	} else {
		// Initialize the ReconcileState for the provider
		reconciler.ProvidersReconcileState[req.NamespacedName] = &ReconciliationState{
			Generation:               -1,
			ConfigMapResourceVersion: nil,
			SentinelETags:            make(map[acpv1.Sentinel]*azcore.ETag),
			KeyValueETags:            make(map[acpv1.Selector][]*azcore.ETag),
			FeatureFlagETags:         make(map[acpv1.Selector][]*azcore.ETag),
			ExistingSecretReferences: make(map[string]*loader.TargetSecretReference),
			ClientManager:            nil,
		}
	}

	// Reset the resource version if the configmap or secret was unexpected deleted
	if existingConfigMap.Name == "" {
		reconciler.ProvidersReconcileState[req.NamespacedName].ConfigMapResourceVersion = nil
	}

	if provider.Spec.Secret == nil {
		reconciler.ProvidersReconcileState[req.NamespacedName].ExistingSecretReferences = make(map[string]*loader.TargetSecretReference)
	} else {
		for name := range reconciler.ProvidersReconcileState[req.NamespacedName].ExistingSecretReferences {
			if _, ok := existingSecrets[name]; !ok {
				reconciler.ProvidersReconcileState[req.NamespacedName].ExistingSecretReferences[name].SecretResourceVersion = ""
			}
		}
	}

	if reconciler.ProvidersReconcileState[req.NamespacedName].ClientManager == nil ||
		reconciler.ProvidersReconcileState[req.NamespacedName].Generation != provider.Generation {
		clientManager, err := loader.NewConfigurationClientManager(ctx, *provider)
		if err != nil {
			reconciler.logAndSetFailStatus(provider, err)
			return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, nil
		}
		reconciler.ProvidersReconcileState[req.NamespacedName].ClientManager = clientManager
	}

	/* Create ConfigurationSettingLoader to get the key-value settings from Azure AppConfiguration. */
	clientManager := reconciler.ProvidersReconcileState[req.NamespacedName].ClientManager
	configLoader, err := loader.NewConfigurationSettingLoader(*provider, clientManager, nil)
	if err != nil {
		reconciler.logAndSetFailStatus(provider, err)
		return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, nil
	}
	var retriever loader.ConfigurationSettingsRetriever
	if reconciler.Retriever == nil {
		retriever = configLoader
	} else {
		retriever = reconciler.Retriever
	}

	ctx = context.WithValue(ctx, loader.RequestTracingKey, loader.RequestTracing{
		IsStartUp: reconciler.ProvidersReconcileState[req.NamespacedName].ConfigMapResourceVersion == nil,
	})

	// Initialize the processor setting in this reconcile
	processor := &AppConfigurationProviderProcessor{
		Context:                 ctx,
		Provider:                provider,
		Retriever:               retriever,
		CurrentTime:             metav1.Now(),
		ReconciliationState:     reconciler.ProvidersReconcileState[req.NamespacedName],
		Settings:                &loader.TargetKeyValueSettings{},
		RefreshOptions:          NewRefreshOptions(),
		SecretReferenceResolver: nil,
	}

	if err := processor.PopulateSettings(&existingConfigMap, existingSecrets); err != nil {
		return reconciler.requeueWhenGetSettingsFailed(provider, err)
	}

	/* Create ConfigMap from key-value settings */
	if processor.RefreshOptions.ConfigMapSettingPopulated {
		result, err := reconciler.createOrUpdateConfigMap(ctx, provider, processor.Settings)
		if err != nil {
			return result, nil
		}
	}

	/* Create secret when there are secret settings */
	if processor.RefreshOptions.SecretSettingPopulated {
		// Verify the existence of the secret which is not owned by the current provider
		for name := range processor.Settings.SecretSettings {
			if _, ok := existingSecrets[name]; !ok {
				_, err := reconciler.verifyTargetObjectExistence(ctx, provider, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
				})

				if err != nil {
					reconciler.logAndSetFailStatus(provider, err)
					return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, nil
				}
			}
		}

		result, err := reconciler.createOrUpdateSecrets(ctx, provider, processor)
		if err != nil {
			return result, nil
		}
	}

	// Expel the secrets which are no longer selected by the provider.
	if provider.Spec.Secret == nil || processor.RefreshOptions.SecretSettingPopulated {
		result, err := reconciler.expelRemovedSecrets(ctx, provider, existingSecrets, processor.Settings.SecretReferences)
		if err != nil {
			return result, nil
		}
	}

	/* Finish the reconcile */
	provider.Status = newProviderStatus(acpv1.PhaseComplete, acpv1.SyncCompleteMessage, metav1.Now(), provider.Status.RefreshStatus)
	return processor.Finish()
}

func (reconciler *AzureAppConfigurationProviderReconciler) verifyTargetObjectExistence(
	ctx context.Context,
	provider *acpv1.AzureAppConfigurationProvider,
	obj client.Object) (bool, error) {
	// Get and verify the existing configMap or secret, if there's existing configMap/secret which is not owned by current provider, throw error
	var targetName string
	if _, ok := obj.(*corev1.ConfigMap); ok {
		targetName = provider.Spec.Target.ConfigMapName
	} else if _, ok := obj.(*corev1.Secret); ok {
		targetName = obj.GetName()
	} else {
		// Only verify ConfigMap and Secret object
		return false, nil
	}
	err := reconciler.Client.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: targetName}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, verifyExistingTargetObject(obj, targetName, provider.Name)
}

func (reconciler *AzureAppConfigurationProviderReconciler) logAndSetFailStatus(
	provider *acpv1.AzureAppConfigurationProvider,
	err error) {
	var showErrorAsWarning bool = false
	namespacedName := types.NamespacedName{
		Name:      provider.Name,
		Namespace: provider.Namespace,
	}
	reconcileState := reconciler.ProvidersReconcileState[namespacedName]
	if _, ok := err.(*loader.ArgumentError); ok {
		// If the error is caused by invalid argument, just show it as error.
		showErrorAsWarning = false
	} else if reconcileState != nil &&
		reconcileState.ConfigMapResourceVersion != nil &&
		(provider.Spec.Secret == nil ||
			len(reconcileState.ExistingSecretReferences) == 0) {
		// If the target ConfigMap or Secret does exists, just show error as warning.
		showErrorAsWarning = true
	}

	if showErrorAsWarning {
		klog.Warningf("Fail to update the target ConfigMap or Secret of AzureAppConfigurationProvider '%s' in '%s' namespace: %s", provider.Name, provider.Namespace, err.Error())
		provider.Status = newProviderStatus(acpv1.PhaseUpdateFailed, acpv1.UpdateFailMessage, provider.Status.LastSyncTime, provider.Status.RefreshStatus)
	} else {
		klog.Errorf("Fail to create the target ConfigMap or Secret of AzureAppConfigurationProvider '%s' in '%s' namespace: %s", provider.Name, provider.Namespace, err.Error())
		provider.Status = newProviderStatus(acpv1.PhaseFailed, acpv1.CreateFailMessage, provider.Status.LastSyncTime, provider.Status.RefreshStatus)
	}
}

func (reconciler *AzureAppConfigurationProviderReconciler) requeueWhenGetSettingsFailed(
	provider *acpv1.AzureAppConfigurationProvider,
	err error) (ctrl.Result, error) {
	requeueAfter := RequeueReconcileAfter
	reconciler.logAndSetFailStatus(provider, err)
	if errors.Is(err, &loader.ArgumentError{}) {
		return reconcile.Result{Requeue: false}, nil
	}
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) && respErr.StatusCode == 429 {
		retryAfter, err := strconv.Atoi(respErr.RawResponse.Header.Get(HeaderRetryAfter))
		if err == nil {
			requeueAfter = time.Duration(retryAfter) * time.Second
			klog.Errorf("Too many requests to the Azure App Configuration endpoint %s, retry the reconciliation after %d seconds", *provider.Spec.Endpoint, retryAfter)
		} else {
			klog.ErrorS(err, "Fail to parse the response header 'Retry-After'")
		}
	}
	return reconcile.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
}

func (reconciler *AzureAppConfigurationProviderReconciler) createOrUpdateConfigMap(
	ctx context.Context,
	provider *acpv1.AzureAppConfigurationProvider,
	settings *loader.TargetKeyValueSettings) (reconcile.Result, error) {
	configMapObj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      provider.Spec.Target.ConfigMapName,
			Namespace: provider.Namespace,
		},
	}
	// Important: set the ownership of configMap
	if err := controllerutil.SetControllerReference(provider, configMapObj, reconciler.Scheme); err != nil {
		reconciler.logAndSetFailStatus(provider, err)
		return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, err
	}

	if provider.Annotations == nil {
		provider.Annotations = make(map[string]string)
	}
	provider.Annotations[LastReconcileTimeAnnotation] = metav1.Now().UTC().String()
	if len(settings.ConfigMapSettings) == 0 {
		klog.V(3).Info("No configMap settings are fetched from Azure AppConfiguration")
	}
	operationResult, err := ctrl.CreateOrUpdate(ctx, reconciler.Client, configMapObj, func() error {
		configMapObj.Data = settings.ConfigMapSettings
		configMapObj.Labels = provider.Labels
		configMapObj.Annotations = provider.Annotations

		return nil
	})
	if err != nil {
		reconciler.logAndSetFailStatus(provider, err)
		return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, err
	}

	namespacedName := types.NamespacedName{
		Name:      provider.Name,
		Namespace: provider.Namespace,
	}
	reconciler.ProvidersReconcileState[namespacedName].ConfigMapResourceVersion = &configMapObj.ResourceVersion
	klog.V(5).Infof("configMap %q in %q namespace is %s", configMapObj.Name, configMapObj.Namespace, string(operationResult))

	return reconcile.Result{}, nil
}

func (reconciler *AzureAppConfigurationProviderReconciler) createOrUpdateSecrets(
	ctx context.Context,
	provider *acpv1.AzureAppConfigurationProvider,
	processor *AppConfigurationProviderProcessor) (reconcile.Result, error) {
	if len(processor.Settings.SecretSettings) == 0 {
		klog.V(3).Info("No secret settings are fetched from Azure AppConfiguration")
	}

	if provider.Annotations == nil {
		provider.Annotations = make(map[string]string)
	}

	namespacedName := types.NamespacedName{
		Name:      provider.Name,
		Namespace: provider.Namespace,
	}

	for secretName, secret := range processor.Settings.SecretSettings {
		if !shouldCreateOrUpdate(processor, secretName) {
			if _, ok := reconciler.ProvidersReconcileState[namespacedName].ExistingSecretReferences[secretName]; ok {
				processor.Settings.SecretReferences[secretName].SecretResourceVersion = reconciler.ProvidersReconcileState[namespacedName].ExistingSecretReferences[secretName].SecretResourceVersion
			}
			klog.V(5).Infof("Skip updating the secret %q in %q namespace since data is not changed", secretName, provider.Namespace)
			continue
		}

		secretObj := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: provider.Namespace,
			},
			Type: secret.Type,
		}

		// Important: set the ownership of secret
		if err := controllerutil.SetControllerReference(provider, secretObj, reconciler.Scheme); err != nil {
			reconciler.logAndSetFailStatus(provider, err)
			return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, err
		}

		provider.Annotations[LastReconcileTimeAnnotation] = metav1.Now().UTC().String()
		operationResult, err := ctrl.CreateOrUpdate(ctx, reconciler.Client, secretObj, func() error {
			secretObj.Data = secret.Data
			secretObj.Labels = provider.Labels
			secretObj.Annotations = provider.Annotations

			return nil
		})
		if err != nil {
			reconciler.logAndSetFailStatus(provider, err)
			return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, err
		}

		processor.Settings.SecretReferences[secretName].SecretResourceVersion = secretObj.ResourceVersion
		klog.V(5).Infof("Secret %q in %q namespace is %s", secretObj.Name, secretObj.Namespace, string(operationResult))
	}

	return reconcile.Result{}, nil
}

func (reconciler *AzureAppConfigurationProviderReconciler) expelRemovedSecrets(
	ctx context.Context,
	provider *acpv1.AzureAppConfigurationProvider,
	existingSecrets map[string]corev1.Secret,
	secretReferences map[string]*loader.TargetSecretReference) (reconcile.Result, error) {
	for name := range existingSecrets {
		if _, ok := secretReferences[name]; !ok {
			err := reconciler.Client.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: provider.Namespace,
				},
			})
			if err != nil {
				reconciler.logAndSetFailStatus(provider, err)
				return reconcile.Result{Requeue: true, RequeueAfter: RequeueReconcileAfter}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func newProviderStatus(
	phase acpv1.AppConfigurationSyncPhase,
	message string,
	syncTime metav1.Time,
	refreshStatus acpv1.RefreshStatus) acpv1.AzureAppConfigurationProviderStatus {
	return acpv1.AzureAppConfigurationProviderStatus{
		Message:           message,
		Phase:             phase,
		LastReconcileTime: metav1.Now(),
		LastSyncTime:      syncTime,
		RefreshStatus:     refreshStatus,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AzureAppConfigurationProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&acpv1.AzureAppConfigurationProvider{}, builder.WithPredicates(newEventFilter())).
		Watches(&corev1.ConfigMap{},
			&EnqueueRequestsFromWatchedObject{},
			builder.WithPredicates(WatchedObjectPredicate{})).
		Watches(&corev1.Secret{},
			&EnqueueRequestsFromWatchedObject{},
			builder.WithPredicates(WatchedObjectPredicate{})).
		Complete(r)
}
