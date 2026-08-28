// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package admission_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpv1 "azappconfig/provider/api/v1"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	testNamespace = "service-account-authorization-test"
	testUser      = "service-account-authorization-test-user"
)

func TestServiceAccountAuthorizationPolicy(t *testing.T) {
	ctx := context.Background()
	testEnvironment := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	config, err := testEnvironment.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testEnvironment.Stop())
	})

	testScheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(testScheme))
	require.NoError(t, acpv1.AddToScheme(testScheme))

	adminClient, err := client.New(config, client.Options{Scheme: testScheme})
	require.NoError(t, err)
	require.NoError(t, installAdmissionPolicy(ctx, adminClient))
	require.NoError(t, adminClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}))

	for _, name := range []string{"app-configuration-reader", "default-key-vault-reader", "per-vault-reader"} {
		require.NoError(t, adminClient.Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		}))
	}

	grantPermission(t, ctx, adminClient, "provider-writer", rbacv1.PolicyRule{
		APIGroups: []string{"azconfig.io"},
		Resources: []string{"azureappconfigurationproviders"},
		Verbs:     []string{"create", "update"},
	})

	userConfig := rest.CopyConfig(config)
	userConfig.Impersonate = rest.ImpersonationConfig{
		UserName: testUser,
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := client.New(userConfig, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	providerWithoutServiceAccount := newProvider("without-service-account", "", "", "")
	require.NoError(t, userClient.Create(ctx, providerWithoutServiceAccount))
	requireCreateForbidden(t, ctx, adminClient, userClient, func(attempt int) *acpv1.AzureAppConfigurationProvider {
		return newProvider(fmt.Sprintf("unauthorized-app-%d", attempt), "app-configuration-reader", "", "")
	})
	providerWithoutServiceAccount.Spec.Auth = workloadIdentityAuth("app-configuration-reader")
	err = userClient.Update(ctx, providerWithoutServiceAccount)
	require.True(t, apierrors.IsForbidden(err))
	require.Contains(t, err.Error(), "not authorized to use every referenced ServiceAccount")

	grantPermission(t, ctx, adminClient, "use-app-configuration-reader", rbacv1.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"serviceaccounts"},
		ResourceNames: []string{"app-configuration-reader"},
		Verbs:         []string{"use"},
	})
	requireCreateAllowed(t, ctx, userClient, func(attempt int) *acpv1.AzureAppConfigurationProvider {
		return newProvider(fmt.Sprintf("authorized-app-%d", attempt), "app-configuration-reader", "", "")
	})

	requireCreateForbidden(t, ctx, adminClient, userClient, func(attempt int) *acpv1.AzureAppConfigurationProvider {
		return newProvider(fmt.Sprintf("unauthorized-default-vault-%d", attempt), "app-configuration-reader", "default-key-vault-reader", "")
	})
	grantPermission(t, ctx, adminClient, "use-default-key-vault-reader", rbacv1.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"serviceaccounts"},
		ResourceNames: []string{"default-key-vault-reader"},
		Verbs:         []string{"use"},
	})
	requireCreateAllowed(t, ctx, userClient, func(attempt int) *acpv1.AzureAppConfigurationProvider {
		return newProvider(fmt.Sprintf("authorized-default-vault-%d", attempt), "app-configuration-reader", "default-key-vault-reader", "")
	})

	requireCreateForbidden(t, ctx, adminClient, userClient, func(attempt int) *acpv1.AzureAppConfigurationProvider {
		return newProvider(fmt.Sprintf("unauthorized-per-vault-%d", attempt), "app-configuration-reader", "default-key-vault-reader", "per-vault-reader")
	})
	grantPermission(t, ctx, adminClient, "mint-per-vault-reader-token", rbacv1.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"serviceaccounts/token"},
		ResourceNames: []string{"per-vault-reader"},
		Verbs:         []string{"create"},
	})
	requireCreateAllowed(t, ctx, userClient, func(attempt int) *acpv1.AzureAppConfigurationProvider {
		return newProvider(fmt.Sprintf("authorized-per-vault-%d", attempt), "app-configuration-reader", "default-key-vault-reader", "per-vault-reader")
	})
}

func installAdmissionPolicy(ctx context.Context, kubeClient client.Client) error {
	manifest, err := os.ReadFile("validatingadmissionpolicy.yaml")
	if err != nil {
		return err
	}

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	for {
		object := &unstructured.Unstructured{}
		if err := decoder.Decode(object); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if len(object.Object) == 0 {
			continue
		}
		if err := kubeClient.Create(ctx, object); err != nil {
			return err
		}
	}
}

func grantPermission(t *testing.T, ctx context.Context, kubeClient client.Client, name string, rule rbacv1.PolicyRule) {
	t.Helper()
	require.NoError(t, kubeClient.Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Rules:      []rbacv1.PolicyRule{rule},
	}))
	require.NoError(t, kubeClient.Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Subjects: []rbacv1.Subject{{
			Kind:     "User",
			Name:     testUser,
			APIGroup: rbacv1.GroupName,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     name,
		},
	}))
}

func requireCreateForbidden(
	t *testing.T,
	ctx context.Context,
	adminClient client.Client,
	userClient client.Client,
	providerFactory func(int) *acpv1.AzureAppConfigurationProvider,
) {
	t.Helper()
	attempt := 0
	var lastError error
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		attempt++
		provider := providerFactory(attempt)
		lastError = userClient.Create(ctx, provider)
		if apierrors.IsForbidden(lastError) && strings.Contains(lastError.Error(), "not authorized to use every referenced ServiceAccount") {
			return true, nil
		}
		if lastError == nil {
			if err := adminClient.Delete(ctx, provider); err != nil {
				return false, err
			}
			return false, nil
		}
		return false, lastError
	})
	require.NoError(t, err, "expected admission denial, last error: %v", lastError)
}

func requireCreateAllowed(
	t *testing.T,
	ctx context.Context,
	userClient client.Client,
	providerFactory func(int) *acpv1.AzureAppConfigurationProvider,
) {
	t.Helper()
	attempt := 0
	var lastError error
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		attempt++
		lastError = userClient.Create(ctx, providerFactory(attempt))
		if lastError == nil {
			return true, nil
		}
		if apierrors.IsForbidden(lastError) {
			return false, nil
		}
		return false, lastError
	})
	require.NoError(t, err, "expected admission success, last error: %v", lastError)
}

func newProvider(name, appConfigurationServiceAccount, defaultKeyVaultServiceAccount, perVaultServiceAccount string) *acpv1.AzureAppConfigurationProvider {
	endpoint := "https://example.azconfig.io"
	provider := &acpv1.AzureAppConfigurationProvider{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &endpoint,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: name + "-config",
			},
		},
	}

	if appConfigurationServiceAccount != "" {
		provider.Spec.Auth = workloadIdentityAuth(appConfigurationServiceAccount)
	}
	if defaultKeyVaultServiceAccount != "" || perVaultServiceAccount != "" {
		keyVaultAuth := &acpv1.AzureKeyVaultAuth{}
		if defaultKeyVaultServiceAccount != "" {
			keyVaultAuth.AzureAppConfigurationProviderAuth = workloadIdentityAuth(defaultKeyVaultServiceAccount)
		}
		if perVaultServiceAccount != "" {
			keyVaultAuth.KeyVaults = []acpv1.AzureKeyVaultPerVaultAuth{{
				Uri:                               "https://example.vault.azure.net",
				AzureAppConfigurationProviderAuth: workloadIdentityAuth(perVaultServiceAccount),
			}}
		}
		provider.Spec.Secret = &acpv1.SecretReference{
			Target: acpv1.SecretGenerationParameters{SecretName: name + "-secret"},
			Auth:   keyVaultAuth,
		}
	}

	return provider
}

func workloadIdentityAuth(serviceAccountName string) *acpv1.AzureAppConfigurationProviderAuth {
	return &acpv1.AzureAppConfigurationProviderAuth{
		WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
			ServiceAccountName: &serviceAccountName,
		},
	}
}
