// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"azappconfig/provider/internal/properties"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"github.com/google/uuid"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/syncmap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

//go:generate mockgen -destination=mocks/mock_configuration_settings_retriever.go -package mocks . ConfigurationSettingsRetriever

type ConfigurationSettingLoader struct {
	acpv1.AzureAppConfigurationProvider
	getSettingsFunc GetSettingsFunc
	AppConfigClient *azappconfig.Client
}

type TargetKeyValueSettings struct {
	ConfigMapSettings         map[string]string
	SecretSettings            map[string][]byte
	KeyVaultReferencesToCache map[string]KeyVaultSecretUriSegment
}

type ConfigurationSettingsRetriever interface {
	CreateKeyValueSettings(ctx context.Context, resolveSecretReference ResolveSecretReference) (*TargetKeyValueSettings, error)
	CheckAndRefreshSentinels(ctx context.Context, provider *acpv1.AzureAppConfigurationProvider, eTags map[acpv1.Sentinel]*azcore.ETag) (bool, map[acpv1.Sentinel]*azcore.ETag, error)
	ResolveKeyVaultReferences(ctx context.Context, kvReferencesToResolve map[string]KeyVaultSecretUriSegment, kvResolver ResolveSecretReference) (map[string][]byte, error)
}

type GetSettingsFunc func(ctx context.Context, acpSpec acpv1.AzureAppConfigurationProviderSpec, client *azappconfig.Client, c chan []azappconfig.Setting, e chan error)

type ServicePrincipleAuthenticationParameters struct {
	ClientId     string
	ClientSecret string
	TenantId     string
}

const (
	KeyVaultReferenceContentType          string = "application/vnd.microsoft.appconfig.keyvaultref+json;charset=utf-8"
	FeatureFlagContentType                string = "application/vnd.microsoft.appconfig.ff+json;charset=utf-8"
	AzureClientId                         string = "azure_client_id"
	AzureClientSecret                     string = "azure_client_secret"
	AzureTenantId                         string = "azure_tenant_id"
	AzureAppConfigurationConnectionString string = "azure_app_configuration_connection_string"
)

var (
	clientOptionWithModuleInfo *azappconfig.ClientOptions = &azappconfig.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Telemetry: policy.TelemetryOptions{
				ApplicationID: fmt.Sprintf("%s/%s", properties.ModuleName, properties.ModuleVersion),
			},
		},
	}
)

func NewConfigurationSettingLoader(ctx context.Context, provider acpv1.AzureAppConfigurationProvider, getSettingsFunc GetSettingsFunc) (*ConfigurationSettingLoader, error) {
	if getSettingsFunc == nil {
		getSettingsFunc = getConfigurationSettings
	}

	var appConfigClient *azappconfig.Client = nil
	if provider.Spec.ConnectionStringReference != nil {
		connectionString, err := getConnectionStringParameter(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: *provider.Spec.ConnectionStringReference})
		if err != nil {
			return nil, err
		}
		appConfigClient, err = azappconfig.NewClientFromConnectionString(connectionString, clientOptionWithModuleInfo)
		if err != nil {
			return nil, err
		}
	} else {
		appConfigCredential, err := createTokenCredential(ctx, provider.Spec.Auth, provider.Namespace)
		if err != nil {
			return nil, err
		}
		appConfigClient, err = azappconfig.NewClient(*provider.Spec.Endpoint, appConfigCredential, clientOptionWithModuleInfo)
		if err != nil {
			return nil, err
		}
	}

	return &ConfigurationSettingLoader{
		AzureAppConfigurationProvider: provider,
		AppConfigClient:               appConfigClient,
		getSettingsFunc:               getSettingsFunc,
	}, nil
}

func (csp *ConfigurationSettingLoader) CreateKeyValueSettings(ctx context.Context, resolveSecretReference ResolveSecretReference) (*TargetKeyValueSettings, error) {
	settingsChan := make(chan []azappconfig.Setting)
	errChan := make(chan error)
	go csp.getSettingsFunc(ctx, csp.Spec, csp.AppConfigClient, settingsChan, errChan)

	targetSettings := &TargetKeyValueSettings{
		ConfigMapSettings:         make(map[string]string),
		SecretSettings:            make(map[string][]byte),
		KeyVaultReferencesToCache: make(map[string]KeyVaultSecretUriSegment),
	}
	var configMapSettings = make(map[string]*string)
	var isJsonContentTypeMap = make(map[string]bool)
	var settings []azappconfig.Setting
	var kvResolver ResolveSecretReference
	var kvReferencesToResolve map[string]KeyVaultSecretUriSegment = make(map[string]KeyVaultSecretUriSegment)

	for {
		select {
		case settings = <-settingsChan:
			if len(settings) == 0 {
				goto end
			}
			for _, setting := range settings {
				trimmedKey := trimPrefix(*setting.Key, csp.Spec.Configuration.TrimKeyPrefixes)
				if len(trimmedKey) == 0 {
					klog.Warningf("key of the setting '%s' is trimmed to the empty string, just ignore it", *setting.Key)
					continue
				}

				if setting.ContentType == nil {
					configMapSettings[trimmedKey] = setting.Value
					isJsonContentTypeMap[trimmedKey] = false
					continue
				}
				switch *setting.ContentType {
				case FeatureFlagContentType:
					continue // ignore feature flag at this moment, will support it in later version
				case KeyVaultReferenceContentType:
					if setting.Value == nil {
						return nil, fmt.Errorf("The value of Key Vault reference '%s' is null", *setting.Key)
					}
					if csp.Spec.Secret == nil {
						return nil, fmt.Errorf("A Key Vault reference is found in App Configuration, but 'spec.secret' was not configured in the Azure App Configuration provider '%s' in namespace '%s'", csp.Name, csp.Namespace)
					}
					if kvResolver == nil {
						if resolveSecretReference == nil {
							if newKvResolver, err := csp.createKeyVaultResolver(ctx); err != nil {
								return nil, err
							} else {
								kvResolver = newKvResolver
							}
						} else {
							kvResolver = resolveSecretReference
						}
					}

					currentUrl := *setting.Value
					secretUriSegment, err := parse(currentUrl)
					if err != nil {
						return nil, err
					}

					// Cache the non-versioned secret reference
					if secretUriSegment.SecretVersion == "" {
						targetSettings.KeyVaultReferencesToCache[trimmedKey] = *secretUriSegment
					}

					kvReferencesToResolve[trimmedKey] = *secretUriSegment
				default:
					configMapSettings[trimmedKey] = setting.Value
					isJsonContentTypeMap[trimmedKey] = isJsonContentType(setting.ContentType)
				}
			}

			// resolve the Key Vault reference settings
			if resolvedSecret, err := csp.ResolveKeyVaultReferences(ctx, kvReferencesToResolve, kvResolver); err != nil {
				return nil, err
			} else {
				maps.Copy(targetSettings.SecretSettings, resolvedSecret)
			}
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
		}
	}

end:
	typedSetting, err := createTypedSettings(configMapSettings, isJsonContentTypeMap, csp.Spec.Target.ConfigMapData)
	if err != nil {
		return nil, err
	}
	targetSettings.ConfigMapSettings = typedSetting
	return targetSettings, nil
}

func (csp *ConfigurationSettingLoader) CheckAndRefreshSentinels(ctx context.Context, provider *acpv1.AzureAppConfigurationProvider, eTags map[acpv1.Sentinel]*azcore.ETag) (bool, map[acpv1.Sentinel]*azcore.ETag, error) {
	sentinelChanged := false
	if provider.Spec.Configuration.Refresh == nil {
		return sentinelChanged, eTags, NewArgumentError("spec.configuration.refresh", fmt.Errorf("refresh is not specified"))
	}
	refreshedETags := make(map[acpv1.Sentinel]*azcore.ETag)

	for _, sentinel := range provider.Spec.Configuration.Refresh.Monitoring.Sentinels {
		if eTag, ok := eTags[sentinel]; ok {
			// Initialize the updatedETags with the current eTags
			refreshedETags[sentinel] = eTag
		}
		refreshedSentinel, err := csp.getSentinelSetting(ctx, provider, sentinel, refreshedETags[sentinel])
		if err != nil {
			return false, eTags, err
		}

		if refreshedSentinel.ETag != nil {
			sentinelChanged = true
			refreshedETags[sentinel] = refreshedSentinel.ETag
		}
	}

	return sentinelChanged, refreshedETags, nil
}

func (csp *ConfigurationSettingLoader) ResolveKeyVaultReferences(ctx context.Context, keyVaultReferencesToResolve map[string]KeyVaultSecretUriSegment, keyVaultResolver ResolveSecretReference) (map[string][]byte, error) {
	if keyVaultResolver == nil {
		if kvResolver, err := csp.createKeyVaultResolver(ctx); err != nil {
			return nil, err
		} else {
			keyVaultResolver = kvResolver
		}
	}

	resolvedSecretReferences := make(map[string][]byte)
	if len(keyVaultReferencesToResolve) > 0 {
		var eg errgroup.Group
		lock := &sync.Mutex{}
		for key, kvReference := range keyVaultReferencesToResolve {
			currentKey := key
			currentReference := kvReference
			eg.Go(func() error {
				resolvedValue, err := keyVaultResolver.Resolve(currentReference, ctx)
				if err != nil {
					return fmt.Errorf("Fail to resolve the Key Vault reference type setting '%s': %s", currentKey, err.Error())
				}
				lock.Lock()
				defer lock.Unlock()
				resolvedSecretReferences[currentKey] = []byte(*resolvedValue)
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	return resolvedSecretReferences, nil
}

func (csp *ConfigurationSettingLoader) getSentinelSetting(ctx context.Context, provider *acpv1.AzureAppConfigurationProvider, sentinel acpv1.Sentinel, etag *azcore.ETag) (*azappconfig.Setting, error) {
	if provider.Spec.Configuration.Refresh == nil {
		return nil, NewArgumentError("spec.configuration.refresh", fmt.Errorf("refresh is not specified"))
	}

	sentinelSetting, err := csp.AppConfigClient.GetSetting(ctx, sentinel.Key, &azappconfig.GetSettingOptions{Label: &sentinel.Label, OnlyIfChanged: etag})
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			var label string
			if sentinel.Label == "\x00" { // NUL is escaped to \x00 in golang
				label = "no"
			} else {
				label = fmt.Sprintf("'%s'", sentinel.Label)
			}
			switch respErr.StatusCode {
			case 404:
				klog.Warningf("Sentinel key '%s' with %s label does not exists, revisit the sentinel after %s", sentinel.Key, label, provider.Spec.Configuration.Refresh.Interval)
				return &azappconfig.Setting{}, nil
			case 304:
				klog.V(3).Infof("There's no change to the sentinel key '%s' with %s label , just exit and revisit the sentinel after %s", sentinel.Key, label, provider.Spec.Configuration.Refresh.Interval)
				return &sentinelSetting.Setting, nil
			}
		}
		return nil, err
	}

	return &sentinelSetting.Setting, nil
}

func (csp *ConfigurationSettingLoader) createKeyVaultResolver(ctx context.Context) (ResolveSecretReference, error) {
	var defaultAuth *acpv1.AzureAppConfigurationProviderAuth = nil
	if csp.Spec.Secret != nil && csp.Spec.Secret.Auth != nil {
		defaultAuth = csp.Spec.Secret.Auth.AzureAppConfigurationProviderAuth
	}
	defaultCred, err := createTokenCredential(ctx, defaultAuth, csp.Namespace)
	if err != nil {
		return nil, err
	}
	secretClients, err := createSecretClients(ctx, csp.AzureAppConfigurationProvider)
	if err != nil {
		return nil, err
	}
	keyVaultResolver := &KeyVaultReferenceResolver{
		DefaultTokenCredential: defaultCred,
		Clients:                secretClients,
	}

	return keyVaultResolver, nil
}

func trimPrefix(key string, prefixToTrim []string) string {
	if len(prefixToTrim) > 0 {
		for _, v := range prefixToTrim {
			if strings.HasPrefix(key, v) {
				return strings.TrimPrefix(key, v)
			}
		}
	}

	return key
}

func getConfigurationSettings(ctx context.Context, acpSpec acpv1.AzureAppConfigurationProviderSpec, client *azappconfig.Client, c chan []azappconfig.Setting, e chan error) {
	filters := getFilters(acpSpec)
	nullString := "\x00"

	for _, filter := range filters {
		if filter.LabelFilter == nil {
			filter.LabelFilter = &nullString // NUL is escaped to \x00 in golang
		}
		selector := azappconfig.SettingSelector{
			KeyFilter:   &filter.KeyFilter,
			LabelFilter: filter.LabelFilter,
			Fields:      azappconfig.AllSettingFields(),
		}
		pager := client.NewListSettingsPager(selector, nil)

		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				e <- err
			} else if len(page.Settings) > 0 {
				c <- page.Settings
			}
		}

	}
	c <- make([]azappconfig.Setting, 0)
}

func createTokenCredential(ctx context.Context, acpAuth *acpv1.AzureAppConfigurationProviderAuth, namespace string) (azcore.TokenCredential, error) {
	// If User explicitly specify the authentication method
	if acpAuth != nil {
		if acpAuth.WorkloadIdentity != nil {
			workloadIdentityClientId, err := getWorkloadIdentityClientId(ctx, acpAuth.WorkloadIdentity, namespace)
			if err != nil {
				return nil, fmt.Errorf("fail to retrieve workload identity client ID from configMap '%s' : %s", acpAuth.WorkloadIdentity.ManagedIdentityClientIdReference.ConfigMap, err.Error())
			}
			return azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
				ClientID: workloadIdentityClientId,
			})
		}
		if acpAuth.ServicePrincipalReference != nil {
			parameter, err := getServicePrincipleAuthenticationParameters(ctx, types.NamespacedName{Namespace: namespace, Name: *acpAuth.ServicePrincipalReference})
			if err != nil {
				return nil, fmt.Errorf("fail to retrieve service principal secret from '%s': %s", *acpAuth.ServicePrincipalReference, err.Error())
			}
			return azidentity.NewClientSecretCredential(parameter.TenantId, parameter.ClientId, parameter.ClientSecret, nil)
		}
		if acpAuth.ManagedIdentityClientId != nil {
			return azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(*acpAuth.ManagedIdentityClientId),
			})
		}
	} else {
		return azidentity.NewManagedIdentityCredential(nil)
	}

	return nil, nil
}

func getWorkloadIdentityClientId(ctx context.Context, workloadIdentityAuth *acpv1.WorkloadIdentityParameters, namespace string) (string, error) {
	if workloadIdentityAuth.ManagedIdentityClientIdReference == nil {
		return *workloadIdentityAuth.ManagedIdentityClientId, nil
	} else {
		configMap, err := getConfigMap(ctx, types.NamespacedName{Namespace: namespace, Name: workloadIdentityAuth.ManagedIdentityClientIdReference.ConfigMap})
		if err != nil {
			return "", err
		}

		if _, ok := configMap.Data[workloadIdentityAuth.ManagedIdentityClientIdReference.Key]; !ok {
			return "", fmt.Errorf("key '%s' does not exist", workloadIdentityAuth.ManagedIdentityClientIdReference.Key)
		}

		managedIdentityClientId := configMap.Data[workloadIdentityAuth.ManagedIdentityClientIdReference.Key]
		if _, err = uuid.Parse(managedIdentityClientId); err != nil {
			return "", fmt.Errorf("managedIdentityClientId %q is not a valid uuid", managedIdentityClientId)
		}

		return managedIdentityClientId, nil
	}
}

func getConnectionStringParameter(ctx context.Context, namespacedSecretName types.NamespacedName) (string, error) {
	secret, err := getSecret(ctx, namespacedSecretName)
	if err != nil {
		return "", err
	}

	return string(secret.Data[AzureAppConfigurationConnectionString]), nil
}

func getServicePrincipleAuthenticationParameters(ctx context.Context, namespacedSecretName types.NamespacedName) (*ServicePrincipleAuthenticationParameters, error) {
	secret, err := getSecret(ctx, namespacedSecretName)
	if err != nil {
		return nil, err
	}

	return &ServicePrincipleAuthenticationParameters{
		ClientId:     string(secret.Data[AzureClientId]),
		ClientSecret: string(secret.Data[AzureClientSecret]),
		TenantId:     string(secret.Data[AzureTenantId]),
	}, nil
}

func getConfigMap(ctx context.Context, namespacedConfigMapName types.NamespacedName) (*corev1.ConfigMap, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	client, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	configMapObject := &corev1.ConfigMap{}
	err = client.Get(ctx, namespacedConfigMapName, configMapObject)
	if err != nil {
		return nil, err
	}

	return configMapObject, nil
}

func getSecret(ctx context.Context, namespacedSecretName types.NamespacedName) (*corev1.Secret, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	client, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	secretObject := &corev1.Secret{}
	err = client.Get(ctx, namespacedSecretName, secretObject)
	if err != nil {
		return nil, err
	}

	return secretObject, nil
}

func getFilters(acpSpec acpv1.AzureAppConfigurationProviderSpec) []acpv1.KeyValueSelector {
	var filters []acpv1.KeyValueSelector
	findDuplicate := false
	if len(acpSpec.Configuration.Selectors) > 0 {
		//
		// Deduplicate the filters in a way that in honor of what user tell us
		// If user populate the selectors with  `{KeyFilter: "one*", LabelFilter: "prod"}, {KeyFilter: "two*", LabelFilter: "dev"}, {KeyFilter: "one*", LabelFilter: "prod"}`
		// We deduplicate it into `{KeyFilter: "two*", LabelFilter: "dev"}, {KeyFilter: "one*", LabelFilter: "prod"}`
		// not `{KeyFilter: "one*", LabelFilter: "prod"}, {KeyFilter: "two*", LabelFilter: "dev"}`
		for i := len(acpSpec.Configuration.Selectors) - 1; i >= 0; i-- {
			findDuplicate = false
			for j := 0; j < len(filters); j++ {
				if strings.Compare(filters[j].KeyFilter, acpSpec.Configuration.Selectors[i].KeyFilter) == 0 &&
					compare(filters[j].LabelFilter, acpSpec.Configuration.Selectors[i].LabelFilter) {
					findDuplicate = true
					break
				}
			}
			if !findDuplicate {
				filters = append(filters, acpSpec.Configuration.Selectors[i])
			}
		}
		reverse(filters)
	} else {
		filters = append(filters, acpv1.KeyValueSelector{
			KeyFilter:   "*",
			LabelFilter: nil,
		})
	}

	return filters
}

func compare(a *string, b *string) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return strings.Compare(*a, *b) == 0
}

func reverse(arr []acpv1.KeyValueSelector) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func createSecretClients(ctx context.Context, acp acpv1.AzureAppConfigurationProvider) (*syncmap.Map, error) {
	secretClients := &syncmap.Map{}
	if acp.Spec.Secret == nil || acp.Spec.Secret.Auth == nil {
		return secretClients, nil
	}
	for _, keyVault := range acp.Spec.Secret.Auth.KeyVaults {
		url, _ := url.Parse(keyVault.Uri)
		tokenCredential, err := createTokenCredential(ctx, keyVault.AzureAppConfigurationProviderAuth, acp.Namespace)
		if err != nil {
			klog.ErrorS(err, fmt.Sprintf("Fail to create token credential for %q", keyVault.Uri))
			return nil, err
		}

		hostName := strings.ToLower(url.Host)
		newSecretClient, err := azsecrets.NewClient("https://"+hostName, tokenCredential, nil)
		if err != nil {
			klog.ErrorS(err, fmt.Sprintf("Fail to create key vault secret client for %q", keyVault.Uri))
			return nil, err
		}
		secretClients.Store(hostName, newSecretClient)
	}

	return secretClients, nil
}