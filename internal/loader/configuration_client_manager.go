// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"azappconfig/provider/internal/properties"
	"context"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	acpv1 "azappconfig/provider/api/v1"

	"math/rand"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

//go:generate mockgen -destination=mocks/mock_configuration_client_manager.go -package mocks . ClientManager

type ConfigurationClientManager struct {
	ReplicaDiscoveryEnabled   bool
	StaticClientWrappers      []*ConfigurationClientWrapper
	DynamicClientWrappers     []*ConfigurationClientWrapper
	validDomain               string
	endpoint                  string
	credential                azcore.TokenCredential
	secret                    string
	id                        string
	lastFallbackClientAttempt metav1.Time
	lastFallbackClientRefresh metav1.Time
}

type ConfigurationClientWrapper struct {
	Endpoint       string
	Client         *azappconfig.Client
	BackOffEndTime metav1.Time
	FailedAttempts int
}

type ClientManager interface {
	GetClients(ctx context.Context) ([]*ConfigurationClientWrapper, error)
	RefreshClients(ctx context.Context) error
}

const (
	TCP                                 string        = "tcp"
	Origin                              string        = "origin"
	Alt                                 string        = "alt"
	EndpointSection                     string        = "Endpoint"
	SecretSection                       string        = "Secret"
	IdSection                           string        = "Id"
	AzConfigDomainLabel                 string        = ".azconfig."
	AppConfigDomainLabel                string        = ".appconfig."
	FallbackClientRefreshExpireInterval time.Duration = time.Hour
	MinimalClientRefreshInterval        time.Duration = time.Second * 30
	MaxBackoffDuration                  time.Duration = time.Minute * 10
	MinBackoffDuration                  time.Duration = time.Second * 30
	JitterRatio                         float64       = 0.25
	SafeShiftLimit                      int           = 63
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

func NewConfigurationClientManager(ctx context.Context, provider acpv1.AzureAppConfigurationProvider) (ClientManager, error) {
	manager := &ConfigurationClientManager{
		ReplicaDiscoveryEnabled: provider.Spec.ReplicaDiscoveryEnabled,
	}

	var err error
	var staticClient *azappconfig.Client
	if provider.Spec.ConnectionStringReference != nil {
		connectionString, err := getConnectionStringParameter(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: *provider.Spec.ConnectionStringReference})
		if err != nil {
			return nil, err
		}
		if manager.endpoint, err = parseConnectionString(connectionString, EndpointSection); err != nil {
			return nil, err
		}
		if manager.secret, err = parseConnectionString(connectionString, SecretSection); err != nil {
			return nil, err
		}
		if manager.id, err = parseConnectionString(connectionString, IdSection); err != nil {
			return nil, err
		}
		if staticClient, err = azappconfig.NewClientFromConnectionString(connectionString, clientOptionWithModuleInfo); err != nil {
			return nil, err
		}
	} else {
		if manager.credential, err = CreateTokenCredential(ctx, provider.Spec.Auth, provider.Namespace); err != nil {
			return nil, err
		}
		if staticClient, err = azappconfig.NewClient(*provider.Spec.Endpoint, manager.credential, clientOptionWithModuleInfo); err != nil {
			return nil, err
		}
		manager.endpoint = *provider.Spec.Endpoint
	}

	manager.validDomain = getValidDomain(manager.endpoint)
	manager.StaticClientWrappers = []*ConfigurationClientWrapper{{
		Endpoint:       manager.endpoint,
		Client:         staticClient,
		BackOffEndTime: metav1.Time{},
		FailedAttempts: 0,
	}}

	return manager, nil
}

func (manager *ConfigurationClientManager) GetClients(ctx context.Context) ([]*ConfigurationClientWrapper, error) {
	currentTime := metav1.Now()
	clients := make([]*ConfigurationClientWrapper, 0)
	for _, clientWrapper := range manager.StaticClientWrappers {
		if currentTime.After(clientWrapper.BackOffEndTime.Time) {
			clients = append(clients, clientWrapper)
		}
	}

	if !manager.ReplicaDiscoveryEnabled {
		return clients, nil
	}

	if currentTime.After(manager.lastFallbackClientAttempt.Time.Add(MinimalClientRefreshInterval)) &&
		(manager.DynamicClientWrappers == nil ||
			currentTime.After(manager.lastFallbackClientRefresh.Time.Add(FallbackClientRefreshExpireInterval))) {
		manager.lastFallbackClientAttempt = currentTime
		url, _ := url.Parse(manager.endpoint)
		go manager.DiscoverFallbackClients(ctx, url.Host)
	}

	for _, clientWrapper := range manager.DynamicClientWrappers {
		if currentTime.After(clientWrapper.BackOffEndTime.Time) {
			clients = append(clients, clientWrapper)
		}
	}

	return clients, nil
}

func (manager *ConfigurationClientManager) RefreshClients(ctx context.Context) error {
	currentTime := metav1.Now()
	if manager.ReplicaDiscoveryEnabled &&
		currentTime.After(manager.lastFallbackClientAttempt.Time.Add(MinimalClientRefreshInterval)) {
		manager.lastFallbackClientAttempt = currentTime
		url, _ := url.Parse(manager.endpoint)
		go manager.DiscoverFallbackClients(ctx, url.Host)
	}

	return nil
}

func (manager *ConfigurationClientManager) DiscoverFallbackClients(ctx context.Context, host string) {
	srvTargetHosts, err := QuerySrvTargetHost(ctx, host)
	if err != nil {
		klog.Warningf("Fail to build fall back clients %s", err.Error())
		return
	}

	// Shuffle the list of SRV target hosts
	for i := range srvTargetHosts {
		j := rand.Intn(i + 1)
		srvTargetHosts[i], srvTargetHosts[j] = srvTargetHosts[j], srvTargetHosts[i]
	}

	newDynamicClients := make([]*ConfigurationClientWrapper, 0)
	for _, host := range srvTargetHosts {
		if isValidEndpoint(host, manager.validDomain) {
			targetEndpoint := "https://" + host
			if strings.ToLower(targetEndpoint) == strings.ToLower(manager.endpoint) {
				continue
			}
			client, err := manager.newConfigurationClient(targetEndpoint)
			if err != nil {
				klog.Warningf("build fallback clients failed, %s", err.Error())
				return
			}
			newDynamicClients = append(newDynamicClients, &ConfigurationClientWrapper{
				Endpoint:       targetEndpoint,
				Client:         client,
				BackOffEndTime: metav1.Time{},
				FailedAttempts: 0,
			})
		}
	}

	manager.DynamicClientWrappers = newDynamicClients
	manager.lastFallbackClientRefresh = metav1.Now()
}

func QuerySrvTargetHost(ctx context.Context, host string) ([]string, error) {
	results := make([]string, 0)
	resolver := net.DefaultResolver

	_, originRecords, err := resolver.LookupSRV(ctx, Origin, TCP, host)
	if err != nil {
		// If the host does not have SRV records => no replicas
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return results, nil
		} else {
			return results, err
		}
	}

	if len(originRecords) == 0 {
		return results, nil
	}

	originHost := strings.TrimSuffix(originRecords[0].Target, ".")
	results = append(results, originHost)
	index := 0
	for {
		currentAlt := Alt + strconv.Itoa(index)
		_, altRecords, err := net.LookupSRV(currentAlt, TCP, originHost)
		if err != nil {
			// If the host does not have SRV records => no more replicas
			if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
				break
			} else {
				return results, err
			}
		}

		for _, record := range altRecords {
			altHost := strings.TrimSuffix(record.Target, ".")
			if altHost != "" {
				results = append(results, altHost)
			}
		}
		index = index + 1
	}

	return results, nil
}

func (manager *ConfigurationClientManager) newConfigurationClient(endpoint string) (*azappconfig.Client, error) {
	if manager.credential != nil {
		return azappconfig.NewClient(endpoint, manager.credential, clientOptionWithModuleInfo)
	}

	connectionStr := buildConnectionString(endpoint, manager.secret, manager.id)
	if connectionStr == "" {
		return nil, fmt.Errorf("failed to build connection string for fallback client")
	}

	return azappconfig.NewClientFromConnectionString(connectionStr, clientOptionWithModuleInfo)
}

func isValidEndpoint(host string, validDomain string) bool {
	if validDomain == "" {
		return false
	}

	return strings.HasSuffix(strings.ToLower(host), strings.ToLower(validDomain))
}

func getValidDomain(endpoint string) string {
	url, _ := url.Parse(endpoint)
	TrustedDomainLabels := []string{AzConfigDomainLabel, AppConfigDomainLabel}
	for _, label := range TrustedDomainLabels {
		index := strings.LastIndex(strings.ToLower(url.Host), strings.ToLower(label))
		if index != -1 {
			return url.Host[index:]
		}
	}

	return ""
}

func buildConnectionString(endpoint string, secret string, id string) string {
	if secret == "" || id == "" {
		return ""
	}

	return fmt.Sprintf("%s=%s;%s=%s;%s=%s",
		EndpointSection, endpoint,
		IdSection, id,
		SecretSection, secret)
}

func parseConnectionString(connectionString string, token string) (string, error) {
	if connectionString == "" {
		return "", fmt.Errorf("connectionString is empty")
	}

	parseToken := token + "="
	startIndex := strings.Index(connectionString, parseToken)
	if startIndex < 0 {
		return "", fmt.Errorf("invalid connectionString %s", connectionString)
	}

	endIndex := strings.Index(connectionString[startIndex:], ";")
	if endIndex < 0 {
		endIndex = len(connectionString)
	} else {
		endIndex += startIndex
	}

	return connectionString[startIndex+len(parseToken) : endIndex], nil
}

func verfityEndpointFromConnectionString(endpoint string) error {
	url, err := url.Parse(strings.ToLower(endpoint))
	if err != nil {
		return fmt.Errorf("invalid endpoint %q from connectionString", endpoint)
	}
	if url.Host == "" {
		return fmt.Errorf("invalid endpoint %q from connectionString, host must be specified", endpoint)
	}
	if url.Scheme != "https" {
		return fmt.Errorf("invalid endpoint %q from connectionString, only https scheme is allowed", endpoint)
	}
	if strings.Trim(url.Path, "/") != "" {
		return fmt.Errorf("invalid endpoint %q from connectionString, only host name is allowed", endpoint)
	}

	return nil
}

func CreateTokenCredential(ctx context.Context, acpAuth *acpv1.AzureAppConfigurationProviderAuth, namespace string) (azcore.TokenCredential, error) {
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
	secret, err := GetSecret(ctx, namespacedSecretName)
	if err != nil {
		return "", err
	}

	return string(secret.Data[AzureAppConfigurationConnectionString]), nil
}

func getServicePrincipleAuthenticationParameters(ctx context.Context, namespacedSecretName types.NamespacedName) (*ServicePrincipleAuthenticationParameters, error) {
	secret, err := GetSecret(ctx, namespacedSecretName)
	if err != nil {
		return nil, err
	}

	return &ServicePrincipleAuthenticationParameters{
		ClientId:     string(secret.Data[AzureClientId]),
		ClientSecret: string(secret.Data[AzureClientSecret]),
		TenantId:     string(secret.Data[AzureTenantId]),
	}, nil
}

func calculateBackoffDuration(failedAttempts int) time.Duration {
	if failedAttempts <= 1 {
		return MinBackoffDuration
	}

	calculatedMilliseconds := math.Max(1, float64(MinBackoffDuration.Milliseconds())) * math.Pow(2, math.Min(float64(failedAttempts-1), float64(SafeShiftLimit)))
	if calculatedMilliseconds > float64(MaxBackoffDuration.Milliseconds()) || calculatedMilliseconds <= 0 {
		calculatedMilliseconds = float64(MaxBackoffDuration.Milliseconds())
	}

	calculatedDuration := time.Duration(calculatedMilliseconds) * time.Millisecond
	return Jitter(calculatedDuration)
}

func Jitter(duration time.Duration) time.Duration {
	// Calculate the amount of jitter to add to the duration
	jitter := float64(duration) * JitterRatio

	// Generate a random number between -jitter and +jitter
	randomJitter := rand.Float64()*(2*jitter) - jitter

	// Apply the random jitter to the original duration
	return duration + time.Duration(randomJitter)
}
