// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"golang.org/x/crypto/pkcs12"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/syncmap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

//go:generate mockgen -destination=mocks/mock_configuration_settings_retriever.go -package mocks . ConfigurationSettingsRetriever

type ConfigurationSettingLoader struct {
	acpv1.AzureAppConfigurationProvider
	ClientManager   ClientManager
	SettingsClient  SettingsClient
	TracingFeatures TracingFeatures
}

type TargetKeyValueSettings struct {
	ConfigMapSettings map[string]string
	// Multiple secrets could be managed
	SecretSettings   map[string]corev1.Secret
	K8sSecrets       map[string]*TargetK8sSecretMetadata
	KeyValueETags    map[acpv1.ComparableSelector][]*azcore.ETag
	FeatureFlagETags map[acpv1.ComparableSelector][]*azcore.ETag
	SentinelETags    map[acpv1.Sentinel]*azcore.ETag
}

type TargetK8sSecretMetadata struct {
	Type                    corev1.SecretType
	SecretsKeyVaultMetadata map[string]KeyVaultSecretMetadata
	SecretResourceVersion   string
}

type RawSettings struct {
	KeyValueSettings     map[string]*string
	IsJsonContentTypeMap map[string]bool
	FeatureFlagSettings  map[string]interface{}
	SecretSettings       map[string]corev1.Secret
	K8sSecrets           map[string]*TargetK8sSecretMetadata
	KeyValueETags        map[acpv1.ComparableSelector][]*azcore.ETag
	FeatureFlagETags     map[acpv1.ComparableSelector][]*azcore.ETag
}

type ConfigurationSettingsRetriever interface {
	CreateTargetSettings(ctx context.Context, resolveSecretReference SecretReferenceResolver) (*TargetKeyValueSettings, error)
	CheckAndRefreshSentinels(ctx context.Context, provider *acpv1.AzureAppConfigurationProvider, eTags map[acpv1.Sentinel]*azcore.ETag) (bool, map[acpv1.Sentinel]*azcore.ETag, error)
	CheckPageETags(ctx context.Context, eTags map[acpv1.ComparableSelector][]*azcore.ETag) (bool, error)
	RefreshKeyValueSettings(ctx context.Context, existingConfigMapSettings *map[string]string, resolveSecretReference SecretReferenceResolver) (*TargetKeyValueSettings, error)
	RefreshFeatureFlagSettings(ctx context.Context, existingConfigMapSettings *map[string]string) (*TargetKeyValueSettings, error)
	ResolveSecretReferences(ctx context.Context, kvReferencesToResolve map[string]*TargetK8sSecretMetadata, kvResolver SecretReferenceResolver) (*TargetKeyValueSettings, error)
}

type ServicePrincipleAuthenticationParameters struct {
	ClientId     string
	ClientSecret string
	TenantId     string
}

const (
	SecretReferenceContentType            string = "application/vnd.microsoft.appconfig.keyvaultref+json;charset=utf-8"
	FeatureFlagContentType                string = "application/vnd.microsoft.appconfig.ff+json;charset=utf-8"
	AzureClientId                         string = "azure_client_id"
	AzureClientSecret                     string = "azure_client_secret"
	AzureTenantId                         string = "azure_tenant_id"
	AzureAppConfigurationConnectionString string = "azure_app_configuration_connection_string"
	FeatureFlagKeyPrefix                  string = ".appconfig.featureflag/"
	FeatureFlagSectionName                string = "feature_flags"
	FeatureManagementSectionName          string = "feature_management"
	PreservedSecretTypeTag                string = ".kubernetes.secret.type"
	CertTypePem                           string = "application/x-pem-file"
	CertTypePfx                           string = "application/x-pkcs12"
	TlsKey                                string = "tls.key"
	TlsCrt                                string = "tls.crt"
	RequestTracingEnabled                 string = "REQUEST_TRACING_ENABLED"
)

func NewConfigurationSettingLoader(provider acpv1.AzureAppConfigurationProvider, clientManager ClientManager, settingsClient SettingsClient) (*ConfigurationSettingLoader, error) {
	return &ConfigurationSettingLoader{
		AzureAppConfigurationProvider: provider,
		ClientManager:                 clientManager,
		SettingsClient:                settingsClient,
		TracingFeatures:               TracingFeatures{},
	}, nil
}

func (csl *ConfigurationSettingLoader) CreateTargetSettings(ctx context.Context, resolveSecretReference SecretReferenceResolver) (*TargetKeyValueSettings, error) {
	rawSettings, err := csl.CreateKeyValueSettings(ctx, resolveSecretReference)
	if err != nil {
		return nil, err
	}

	initializedSentinelETags := make(map[acpv1.Sentinel]*azcore.ETag)
	if csl.Spec.Configuration.Refresh != nil &&
		csl.Spec.Configuration.Refresh.Enabled &&
		csl.Spec.Configuration.Refresh.Monitoring != nil {
		sentinels := normalizeSentinels(csl.Spec.Configuration.Refresh.Monitoring.Sentinels)
		eTags := make(map[acpv1.Sentinel]*azcore.ETag)
		for _, sentinel := range sentinels {
			eTags[sentinel] = nil
		}
		if _, initializedSentinelETags, err = csl.CheckAndRefreshSentinels(ctx, &csl.AzureAppConfigurationProvider, eTags); err != nil {
			return nil, err
		}
	}

	if csl.Spec.FeatureFlag != nil {
		if rawSettings.FeatureFlagSettings, rawSettings.FeatureFlagETags, err = csl.getFeatureFlagSettings(ctx); err != nil {
			return nil, err
		}
	}

	typedSettings, err := createTypedSettings(rawSettings, csl.Spec.Target.ConfigMapData)
	if err != nil {
		return nil, err
	}

	return &TargetKeyValueSettings{
		ConfigMapSettings: typedSettings,
		SecretSettings:    rawSettings.SecretSettings,
		K8sSecrets:        rawSettings.K8sSecrets,
		KeyValueETags:     rawSettings.KeyValueETags,
		FeatureFlagETags:  rawSettings.FeatureFlagETags,
		SentinelETags:     initializedSentinelETags,
	}, nil
}

func (csl *ConfigurationSettingLoader) RefreshKeyValueSettings(ctx context.Context, existingConfigMapSetting *map[string]string, resolveSecretReference SecretReferenceResolver) (*TargetKeyValueSettings, error) {
	rawSettings, err := csl.CreateKeyValueSettings(ctx, resolveSecretReference)
	if err != nil {
		return nil, err
	}

	if csl.Spec.FeatureFlag != nil {
		rawSettings.FeatureFlagSettings, _, err = unmarshalConfigMap(existingConfigMapSetting, csl.Spec.Target.ConfigMapData)
		if err != nil {
			return nil, err
		}
	}

	typedSettings, err := createTypedSettings(rawSettings, csl.Spec.Target.ConfigMapData)
	if err != nil {
		return nil, err
	}

	return &TargetKeyValueSettings{
		ConfigMapSettings: typedSettings,
		SecretSettings:    rawSettings.SecretSettings,
		K8sSecrets:        rawSettings.K8sSecrets,
		KeyValueETags:     rawSettings.KeyValueETags,
	}, nil
}

func (csl *ConfigurationSettingLoader) RefreshFeatureFlagSettings(ctx context.Context, existingConfigMapSetting *map[string]string) (*TargetKeyValueSettings, error) {
	latestFeatureFlagSettings, latestFeatureFlagETags, err := csl.getFeatureFlagSettings(ctx)
	if err != nil {
		return nil, err
	}

	_, existingSettings, err := unmarshalConfigMap(existingConfigMapSetting, csl.Spec.Target.ConfigMapData)
	if err != nil {
		return nil, err
	}

	existingSettings[FeatureManagementSectionName] = latestFeatureFlagSettings
	typedStr, err := marshalJsonYaml(existingSettings, csl.Spec.Target.ConfigMapData)
	if err != nil {
		return nil, err
	}

	return &TargetKeyValueSettings{
		ConfigMapSettings: map[string]string{
			csl.Spec.Target.ConfigMapData.Key: typedStr,
		},
		FeatureFlagETags: latestFeatureFlagETags,
	}, nil
}

func (csl *ConfigurationSettingLoader) CreateKeyValueSettings(ctx context.Context, secretReferenceResolver SecretReferenceResolver) (*RawSettings, error) {
	keyValueFilters := GetKeyValueFilters(csl.Spec)
	settingsClient := csl.SettingsClient
	if settingsClient == nil {
		settingsClient = &SelectorSettingsClient{
			selectors: keyValueFilters,
		}
	}
	settingsResponse, err := csl.ExecuteFailoverPolicy(ctx, settingsClient)
	if err != nil {
		return nil, err
	}

	rawSettings := &RawSettings{
		KeyValueSettings:     make(map[string]*string),
		IsJsonContentTypeMap: make(map[string]bool),
		SecretSettings:       make(map[string]corev1.Secret),
		K8sSecrets:           make(map[string]*TargetK8sSecretMetadata),
		KeyValueETags:        settingsResponse.Etags,
	}

	if csl.Spec.Secret != nil {
		rawSettings.K8sSecrets[csl.Spec.Secret.Target.SecretName] = &TargetK8sSecretMetadata{
			Type:                    corev1.SecretTypeOpaque,
			SecretsKeyVaultMetadata: make(map[string]KeyVaultSecretMetadata),
		}
	}

	resolver := secretReferenceResolver
	useAIConfiguration := false
	useAIChatCompletionConfiguration := false
	for _, setting := range settingsResponse.Settings {
		trimmedKey := trimPrefix(*setting.Key, csl.Spec.Configuration.TrimKeyPrefixes)
		if len(trimmedKey) == 0 {
			klog.Warningf("key of the setting '%s' is trimmed to the empty string, just ignore it", *setting.Key)
			continue
		}

		if setting.ContentType == nil {
			rawSettings.KeyValueSettings[trimmedKey] = setting.Value
			rawSettings.IsJsonContentTypeMap[trimmedKey] = false
			continue
		}
		switch strings.TrimSpace(strings.ToLower(*setting.ContentType)) {
		case FeatureFlagContentType:
			continue // ignore feature flag while getting key value settings
		case SecretReferenceContentType:
			if setting.Value == nil {
				return nil, fmt.Errorf("the value of Key Vault reference '%s' is null", *setting.Key)
			}

			if csl.Spec.Secret == nil {
				return nil, fmt.Errorf("a Key Vault reference is found in App Configuration, but 'spec.secret' was not configured in the Azure App Configuration provider '%s' in namespace '%s'", csl.Name, csl.Namespace)
			}

			var secretType corev1.SecretType = corev1.SecretTypeOpaque
			var err error
			if secretTypeTag, ok := setting.Tags[PreservedSecretTypeTag]; ok {
				if secretTypeTag == nil {
					return nil, fmt.Errorf("the secret type tag '%s' for setting '%s' is invalid", PreservedSecretTypeTag, *setting.Key)
				}

				secretType, err = parseSecretType(*secretTypeTag)
				if err != nil {
					return nil, err
				}
			}

			if resolver == nil {
				if newResolver, err := csl.createSecretReferenceResolver(ctx); err != nil {
					return nil, err
				} else {
					resolver = newResolver
				}
			}

			currentUrl := *setting.Value
			secretMetadata, err := parse(currentUrl)
			if err != nil {
				return nil, err
			}

			secretName := trimmedKey
			// If the secret type is not specified, reside it to the Secret with name specified
			if secretType == corev1.SecretTypeOpaque {
				secretName = csl.Spec.Secret.Target.SecretName
			}

			if _, ok := rawSettings.K8sSecrets[secretName]; !ok {
				rawSettings.K8sSecrets[secretName] = &TargetK8sSecretMetadata{
					Type:                    secretType,
					SecretsKeyVaultMetadata: make(map[string]KeyVaultSecretMetadata),
				}
			}
			rawSettings.K8sSecrets[secretName].SecretsKeyVaultMetadata[trimmedKey] = *secretMetadata
		default:
			rawSettings.KeyValueSettings[trimmedKey] = setting.Value
			rawSettings.IsJsonContentTypeMap[trimmedKey] = isJsonContentType(setting.ContentType)
			if !rawSettings.IsJsonContentTypeMap[trimmedKey] {
				continue
			}
			if isAIConfigurationContentType(setting.ContentType) {
				useAIConfiguration = true
			}
			if isAIChatCompletionContentType(setting.ContentType) {
				useAIChatCompletionConfiguration = true
			}
		}
	}

	csl.TracingFeatures.UseAIConfiguration = useAIConfiguration
	csl.TracingFeatures.UseAIChatCompletionConfiguration = useAIChatCompletionConfiguration

	// resolve the secret reference settings
	if resolvedSecret, err := csl.ResolveSecretReferences(ctx, rawSettings.K8sSecrets, resolver); err != nil {
		return nil, err
	} else {
		rawSettings.K8sSecrets = resolvedSecret.K8sSecrets
		err = MergeSecret(rawSettings.SecretSettings, resolvedSecret.SecretSettings)
		if err != nil {
			return nil, err
		}
	}

	return rawSettings, nil
}

func (csl *ConfigurationSettingLoader) CheckAndRefreshSentinels(
	ctx context.Context,
	provider *acpv1.AzureAppConfigurationProvider,
	eTags map[acpv1.Sentinel]*azcore.ETag) (bool, map[acpv1.Sentinel]*azcore.ETag, error) {
	sentinelChanged := false
	refreshedETags := make(map[acpv1.Sentinel]*azcore.ETag)
	for sentinel, currentETag := range eTags {
		refreshedETags[sentinel] = currentETag
		settingsClient := csl.SettingsClient
		if settingsClient == nil {
			settingsClient = &SentinelSettingsClient{
				sentinel:        sentinel,
				etag:            currentETag,
				refreshInterval: provider.Spec.Configuration.Refresh.Interval,
			}
		}
		response, err := csl.ExecuteFailoverPolicy(ctx, settingsClient)
		if err != nil {
			return false, eTags, err
		}

		if response != nil && response.Settings != nil && response.Settings[0].ETag != nil {
			sentinelChanged = true
			refreshedETags[sentinel] = response.Settings[0].ETag
		}
	}

	return sentinelChanged, refreshedETags, nil
}

func (csl *ConfigurationSettingLoader) CheckPageETags(ctx context.Context, eTags map[acpv1.ComparableSelector][]*azcore.ETag) (bool, error) {
	settingsClient := csl.SettingsClient
	if settingsClient == nil {
		settingsClient = &EtagSettingsClient{
			etags:           eTags,
			refreshInterval: csl.Spec.Configuration.Refresh.Interval,
		}
	}

	settingsResponse, err := csl.ExecuteFailoverPolicy(ctx, settingsClient)
	if err != nil {
		return false, err
	}

	// when the etag is nil, it means the page eTags are not changed
	return settingsResponse.Etags != nil, nil
}

func (csl *ConfigurationSettingLoader) getFeatureFlagSettings(ctx context.Context) (map[string]interface{}, map[acpv1.ComparableSelector][]*azcore.ETag, error) {
	featureFlagFilters := GetFeatureFlagFilters(csl.Spec)
	settingsClient := csl.SettingsClient
	if settingsClient == nil {
		settingsClient = &SelectorSettingsClient{
			selectors: featureFlagFilters,
		}
	}
	settingsResponse, err := csl.ExecuteFailoverPolicy(ctx, settingsClient)
	if err != nil {
		return nil, nil, err
	}

	settingsLength := len(settingsResponse.Settings)
	featureFlagExist := make(map[string]bool, settingsLength)
	deduplicatedFeatureFlags := make([]interface{}, 0)
	clientEndpoint := ""
	if manager, ok := csl.ClientManager.(*ConfigurationClientManager); ok {
		// use primary client endpoint in feature flag reference
		clientEndpoint = manager.StaticClientWrappers[0].Endpoint
	}

	// if settings returned like this: [{"id": "Beta"...}, {"id": "Alpha"...}, {"id": "Beta"...}], we need to deduplicate it to [{"id": "Alpha"...}, {"id": "Beta"...}], the last one wins
	for i := settingsLength - 1; i >= 0; i-- {
		key := *settingsResponse.Settings[i].Key
		if featureFlagExist[key] {
			continue
		}
		featureFlagExist[key] = true
		var out map[string]interface{}
		err := json.Unmarshal([]byte(*settingsResponse.Settings[i].Value), &out)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal feature flag settings: %s", err.Error())
		}
		populateTelemetryMetadata(out, settingsResponse.Settings[i], clientEndpoint)
		deduplicatedFeatureFlags = append(deduplicatedFeatureFlags, out)
	}

	// reverse the deduplicateFeatureFlags to keep the order
	for i, j := 0, len(deduplicatedFeatureFlags)-1; i < j; i, j = i+1, j-1 {
		deduplicatedFeatureFlags[i], deduplicatedFeatureFlags[j] = deduplicatedFeatureFlags[j], deduplicatedFeatureFlags[i]
	}

	// featureFlagSection = {"feature_flags": [{...}, {...}]}
	var featureFlagSection = map[string]interface{}{
		FeatureFlagSectionName: deduplicatedFeatureFlags,
	}

	return featureFlagSection, settingsResponse.Etags, nil
}

func (csl *ConfigurationSettingLoader) ResolveSecretReferences(
	ctx context.Context,
	secretReferencesToResolve map[string]*TargetK8sSecretMetadata,
	resolver SecretReferenceResolver) (*TargetKeyValueSettings, error) {
	if resolver == nil {
		if kvResolver, err := csl.createSecretReferenceResolver(ctx); err != nil {
			return nil, err
		} else {
			resolver = kvResolver
		}
	}

	resolvedSecrets := make(map[string]corev1.Secret)
	for name, targetSecretReference := range secretReferencesToResolve {
		resolvedSecrets[name] = corev1.Secret{
			Data: make(map[string][]byte),
			Type: targetSecretReference.Type,
		}

		var eg errgroup.Group
		if targetSecretReference.Type == corev1.SecretTypeOpaque {
			if len(targetSecretReference.SecretsKeyVaultMetadata) > 0 {
				lock := &sync.Mutex{}
				secretsMap := make(map[string]string)

				for key, kvReference := range targetSecretReference.SecretsKeyVaultMetadata {
					currentKey := key
					currentReference := kvReference
					eg.Go(func() error {
						resolvedSecret, err := resolver.Resolve(currentReference, ctx)
						if err != nil {
							return fmt.Errorf("fail to resolve the Key Vault reference type setting '%s': %s", currentKey, err.Error())
						}
						lock.Lock()
						defer lock.Unlock()
						if resolvedSecret.Value != nil {
							secretsMap[currentKey] = *resolvedSecret.Value
						}

						return nil
					})
				}

				if err := eg.Wait(); err != nil {
					return nil, err
				}

				typedResolvedSecrets, err := createTypedSecrets(secretsMap, csl.Spec.Secret.Target.SecretData)
				if err != nil {
					return nil, err
				}

				for k, v := range typedResolvedSecrets {
					resolvedSecrets[name].Data[k] = []byte(v)
				}
			}
		} else if targetSecretReference.Type == corev1.SecretTypeTLS {
			eg.Go(func() error {
				resolvedSecret, err := resolver.Resolve(targetSecretReference.SecretsKeyVaultMetadata[name], ctx)
				if err != nil {
					return fmt.Errorf("fail to resolve the Key Vault reference type setting '%s': %s", name, err.Error())
				}

				if resolvedSecret.ContentType == nil {
					return fmt.Errorf("unspecified content type")
				}

				switch *resolvedSecret.ContentType {
				case CertTypePfx:
					resolvedSecrets[name].Data[TlsKey], resolvedSecrets[name].Data[TlsCrt], err = decodePkcs12(*resolvedSecret.Value)
				case CertTypePem:
					resolvedSecrets[name].Data[TlsKey], resolvedSecrets[name].Data[TlsCrt], err = decodePem(*resolvedSecret.Value)
				default:
					err = fmt.Errorf("unknown content type '%s'", *resolvedSecret.ContentType)
				}

				if err != nil {
					return fmt.Errorf("fail to decode the cert '%s': %s", name, err.Error())
				}

				return nil
			})
		}
		// All other types are not supported

		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	return &TargetKeyValueSettings{
		SecretSettings: resolvedSecrets,
		K8sSecrets:     secretReferencesToResolve,
	}, nil
}

func (csl *ConfigurationSettingLoader) createSecretReferenceResolver(ctx context.Context) (SecretReferenceResolver, error) {
	var defaultAuth *acpv1.AzureAppConfigurationProviderAuth = nil
	if csl.Spec.Secret != nil && csl.Spec.Secret.Auth != nil {
		defaultAuth = csl.Spec.Secret.Auth.AzureAppConfigurationProviderAuth
	}
	defaultCred, err := CreateTokenCredential(ctx, defaultAuth, csl.Namespace)
	if err != nil {
		return nil, err
	}
	secretClients, err := createSecretClients(ctx, csl.AzureAppConfigurationProvider)
	if err != nil {
		return nil, err
	}
	resolver := &KeyVaultConnector{
		DefaultTokenCredential: defaultCred,
		Clients:                secretClients,
	}

	return resolver, nil
}

func (csl *ConfigurationSettingLoader) ExecuteFailoverPolicy(ctx context.Context, settingsClient SettingsClient) (*SettingsResponse, error) {
	clients, err := csl.ClientManager.GetClients(ctx)
	if err != nil {
		return nil, err
	}

	if len(clients) == 0 {
		csl.ClientManager.RefreshClients(ctx)
		return nil, fmt.Errorf("no client is available to connect to the target App Configuration store")
	}

	manager, ok := csl.ClientManager.(*ConfigurationClientManager)
	if csl.AzureAppConfigurationProvider.Spec.LoadBalancingEnabled && ok && manager.lastSuccessfulEndpoint != "" && len(clients) > 1 {
		nextClientIndex := 0
		for _, clientWrapper := range clients {
			nextClientIndex++
			if clientWrapper.Endpoint == manager.lastSuccessfulEndpoint {
				break
			}
		}

		// If we found the last successful client,we'll rotate the list so that the next client is at the beginning
		if nextClientIndex < len(clients) {
			rotate(clients, nextClientIndex)
		}
	}

	errors := make([]error, 0)
	var tracingEnabled, isFailoverRequest bool
	if value, ok := os.LookupEnv(RequestTracingEnabled); ok {
		tracingEnabled, _ = strconv.ParseBool(value)
	}
	for _, clientWrapper := range clients {
		if tracingEnabled {
			csl.TracingFeatures.IsFailoverRequest = isFailoverRequest
			csl.TracingFeatures.ReplicaCount = len(manager.DynamicClientWrappers)
			ctx = policy.WithHTTPHeader(ctx, createCorrelationContextHeader(ctx, csl.AzureAppConfigurationProvider, csl.TracingFeatures))
		}
		settingsResponse, err := settingsClient.GetSettings(ctx, clientWrapper.Client)
		successful := true
		if err != nil {
			successful = false
			updateClientBackoffStatus(clientWrapper, successful)
			if IsFailoverable(err) {
				klog.Warningf("current client of '%s' failed to get settings: %s", clientWrapper.Endpoint, err.Error())
				errors = append(errors, err)
				isFailoverRequest = true
				continue
			}
			return nil, err
		}

		if manager, ok := csl.ClientManager.(*ConfigurationClientManager); ok {
			manager.lastSuccessfulEndpoint = clientWrapper.Endpoint
		}
		updateClientBackoffStatus(clientWrapper, successful)
		return settingsResponse, nil
	}

	// Failed to execute failover policy
	csl.ClientManager.RefreshClients(ctx)
	return nil, fmt.Errorf("all app configuration clients failed to get settings: %v", errors)
}

func updateClientBackoffStatus(clientWrapper *ConfigurationClientWrapper, successful bool) {
	if successful {
		clientWrapper.BackOffEndTime = metav1.Time{}
		// Reset FailedAttempts when client succeeded
		if clientWrapper.FailedAttempts > 0 {
			clientWrapper.FailedAttempts = 0
		}
		// Use negative value to indicate that successful attempt
		clientWrapper.FailedAttempts--
	} else {
		//Reset FailedAttempts when client failed
		if clientWrapper.FailedAttempts < 0 {
			clientWrapper.FailedAttempts = 0
		}
		clientWrapper.FailedAttempts++
		clientWrapper.BackOffEndTime = metav1.Time{Time: metav1.Now().Add(calculateBackoffDuration(clientWrapper.FailedAttempts))}
	}
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

func GetSecret(ctx context.Context,
	namespacedSecretName types.NamespacedName) (*corev1.Secret, error) {
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

func GetKeyValueFilters(acpSpec acpv1.AzureAppConfigurationProviderSpec) []acpv1.Selector {
	return deduplicateFilters(normalizeLabelFilter(acpSpec.Configuration.Selectors))
}

func GetFeatureFlagFilters(acpSpec acpv1.AzureAppConfigurationProviderSpec) []acpv1.Selector {
	featureFlagFilters := make([]acpv1.Selector, 0)

	if acpSpec.FeatureFlag != nil {
		featureFlagFilters = deduplicateFilters(normalizeLabelFilter(acpSpec.FeatureFlag.Selectors))
		for i := 0; i < len(featureFlagFilters); i++ {
			if featureFlagFilters[i].KeyFilter != nil {
				prefixedFeatureFlagFilter := FeatureFlagKeyPrefix + *featureFlagFilters[i].KeyFilter
				featureFlagFilters[i].KeyFilter = &prefixedFeatureFlagFilter
			}
		}
	}

	return featureFlagFilters
}

func normalizeSentinels(sentinels []acpv1.Sentinel) []acpv1.Sentinel {
	var results []acpv1.Sentinel
	nullString := "\x00"
	for _, sentinel := range sentinels {
		label := sentinel.Label
		if sentinel.Label == nil || len(*sentinel.Label) == 0 {
			label = &nullString
		}

		results = append(results, acpv1.Sentinel{
			Key:   sentinel.Key,
			Label: label,
		})

	}
	return results
}

func normalizeLabelFilter(filters []acpv1.Selector) []acpv1.Selector {
	var result []acpv1.Selector
	nullString := "\x00"
	for i := 0; i < len(filters); i++ {
		labelFilter := filters[i].LabelFilter
		if filters[i].LabelFilter == nil || len(*filters[i].LabelFilter) == 0 {
			labelFilter = &nullString
		}

		result = append(result, acpv1.Selector{
			KeyFilter:    filters[i].KeyFilter,
			LabelFilter:  labelFilter,
			SnapshotName: filters[i].SnapshotName,
			TagFilters:   filters[i].TagFilters,
		})
	}

	return result
}

func deduplicateFilters(filters []acpv1.Selector) []acpv1.Selector {
	var result []acpv1.Selector
	findDuplicate := false

	if len(filters) > 0 {
		//
		// Deduplicate the filters in a way that in honor of what user tell us
		// If user populate the selectors with  `{KeyFilter: "one*", LabelFilter: "prod", TagFilters: ["tag1"]}, {KeyFilter: "two*", LabelFilter: "dev"}, {KeyFilter: "one*", LabelFilter: "prod", TagFilters: ["tag1"]}`
		// We deduplicate it into `{KeyFilter: "two*", LabelFilter: "dev"}, {KeyFilter: "one*", LabelFilter: "prod", TagFilters: ["tag1"]}`
		// not `{KeyFilter: "one*", LabelFilter: "prod", TagFilters: ["tag1"]}, {KeyFilter: "two*", LabelFilter: "dev"}`
		// Comparison includes KeyFilter, LabelFilter, SnapshotName, and TagFilters
		for i := len(filters) - 1; i >= 0; i-- {
			findDuplicate = false
			for j := 0; j < len(result); j++ {
				if compare(result[j].KeyFilter, filters[i].KeyFilter) &&
					compare(result[j].LabelFilter, filters[i].LabelFilter) &&
					compare(result[j].SnapshotName, filters[i].SnapshotName) &&
					compareTagFilters(result[j].TagFilters, filters[i].TagFilters) {
					findDuplicate = true
					break
				}
			}
			if !findDuplicate {
				result = append(result, filters[i])
			}
		}
		reverse(result)
	} else {
		wildcard := "*"
		result = append(result, acpv1.Selector{
			KeyFilter:   &wildcard,
			LabelFilter: nil,
		})
	}

	return result
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

func compareTagFilters(a []string, b []string) bool {
	// If both are nil or both are empty, they are equal
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	// If lengths are different, they are not equal
	if len(a) != len(b) {
		return false
	}

	// Create sorted copies to compare
	aSorted := make([]string, len(a))
	bSorted := make([]string, len(b))
	copy(aSorted, a)
	copy(bSorted, b)

	// Sort both slices for comparison
	sort.Strings(aSorted)
	sort.Strings(bSorted)

	// Compare sorted slices element by element
	for i := 0; i < len(aSorted); i++ {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}

	return true
}

func reverse(arr []acpv1.Selector) {
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func createSecretClients(
	ctx context.Context,
	acp acpv1.AzureAppConfigurationProvider) (*syncmap.Map, error) {
	secretClients := &syncmap.Map{}
	if acp.Spec.Secret == nil || acp.Spec.Secret.Auth == nil {
		return secretClients, nil
	}
	for _, keyVault := range acp.Spec.Secret.Auth.KeyVaults {
		url, _ := url.Parse(keyVault.Uri)
		tokenCredential, err := CreateTokenCredential(ctx, keyVault.AzureAppConfigurationProviderAuth, acp.Namespace)
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

func parseSecretType(secretType string) (corev1.SecretType, error) {
	secretTypeMap := map[string]corev1.SecretType{
		"opaque":            corev1.SecretTypeOpaque,
		"kubernetes.io/tls": corev1.SecretTypeTLS,
	}

	if parsedType, ok := secretTypeMap[secretType]; ok {
		if parsedType != corev1.SecretTypeTLS {
			return "", fmt.Errorf("secret type %q is not supported", secretType)
		} else {
			return parsedType, nil
		}
	} else {
		return "", fmt.Errorf("secret type %q is not supported", secretType)
	}
}

func decodePkcs12(value string) (key []byte, crt []byte, err error) {
	pfxRaw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, nil, err
	}
	// using ToPEM to extract more than one certificate and key in pfxData
	pemBlock, err := pkcs12.ToPEM(pfxRaw, "")
	if err != nil {
		return nil, nil, err
	}

	return parsePemBlock(pemBlock)
}

func decodePem(value string) (key []byte, crt []byte, err error) {
	pemBlocks := []*pem.Block{}
	for pemBlock, rest := pem.Decode([]byte(value)); pemBlock != nil; pemBlock, rest = pem.Decode(rest) {
		pemBlocks = append(pemBlocks, pemBlock)
	}
	if len(pemBlocks) == 0 {
		return nil, nil, fmt.Errorf("failed to decode pem block")
	}

	return parsePemBlock(pemBlocks)
}

func parsePemBlock(pemBlock []*pem.Block) ([]byte, []byte, error) {
	// PEM block encoded form contains the headers
	//    -----BEGIN Type-----
	//    Headers
	//    base64-encoded Bytes
	//    -----END Type-----
	// Setting headers to nil to ensure no headers included in the encoded block
	var pemKeyData, pemCertData []byte
	for _, block := range pemBlock {

		block.Headers = make(map[string]string)
		if block.Type == "CERTIFICATE" {
			pemCertData = append(pemCertData, pem.EncodeToMemory(block)...)
		} else {
			key, err := parsePrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, err
			}
			// pkcs1 RSA private key PEM file is specific for RSA keys. RSA is not used exclusively inside X509
			// and SSL/TLS, a more generic key format is available in the form of PKCS#8 that identifies the type
			// of private key and contains the relevant data.
			// Converting to pkcs8 private key as ToPEM uses pkcs1
			// The driver determines the key type from the pkcs8 form of the key and marshals appropriately
			block.Bytes, err = x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return nil, nil, err
			}
			pemKeyData = append(pemKeyData, pem.EncodeToMemory(block)...)
		}
	}

	return pemKeyData, pemCertData, nil
}

func parsePrivateKey(block []byte) (interface{}, error) {
	if key, err := x509.ParsePKCS1PrivateKey(block); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse key for type pkcs1, pkcs8 or ec")
}

func MergeSecret(secret map[string]corev1.Secret, newSecret map[string]corev1.Secret) error {
	for k, v := range newSecret {
		if _, ok := secret[k]; !ok {
			secret[k] = v
		} else if secret[k].Type != v.Type {
			return fmt.Errorf("secret type mismatch for key %q", k)
		} else {
			maps.Copy(secret[k].Data, v.Data)
		}
	}

	return nil
}

// rotates the slice to the left by k positions
func rotate(clients []*ConfigurationClientWrapper, k int) {
	n := len(clients)
	k = k % n
	if k == 0 {
		return
	}
	// Reverse the entire slice
	reverseClients(clients, 0, n-1)
	// Reverse the first part
	reverseClients(clients, 0, n-k-1)
	// Reverse the second part
	reverseClients(clients, n-k, n-1)
}

func reverseClients(clients []*ConfigurationClientWrapper, start, end int) {
	for start < end {
		clients[start], clients[end] = clients[end], clients[start]
		start++
		end--
	}
}

func generateFeatureFlagReference(setting azappconfig.Setting, endpoint string) string {
	featureFlagReference := fmt.Sprintf("%s/kv/%s", endpoint, *setting.Key)

	// Check if the label is present and not empty
	if setting.Label != nil && strings.TrimSpace(*setting.Label) != "" {
		featureFlagReference += fmt.Sprintf("?label=%s", *setting.Label)
	}

	return featureFlagReference
}

func populateTelemetryMetadata(featureFlag map[string]interface{}, setting azappconfig.Setting, endpoint string) {
	if telemetry, ok := featureFlag[TelemetryKey].(map[string]interface{}); ok {
		if enabled, ok := telemetry[EnabledKey].(bool); ok && enabled {
			metadata, _ := telemetry[MetadataKey].(map[string]interface{})
			if metadata == nil {
				metadata = make(map[string]interface{})
			}

			// Set the new metadata
			metadata[ETagKey] = *setting.ETag
			metadata[FeatureFlagReferenceKey] = generateFeatureFlagReference(setting, endpoint)
			telemetry[MetadataKey] = metadata
		}
	}
}
