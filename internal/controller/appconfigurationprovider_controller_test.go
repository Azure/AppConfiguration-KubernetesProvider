// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"azappconfig/provider/internal/loader"
	"context"
	"fmt"
	"os"
	"time"

	acpv1 "azappconfig/provider/api/v1"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("AppConfiguationProvider controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ProviderName      = "test-appconfigurationprovider"
		ProviderNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		EndpointName = "https://fake-endpoint"
	)

	Context("When create AzureAppConfigurationProvider object", func() {
		It("Should update AzureAppConfigurationProvider Status to COMPLETE after reconcile finish", func() {
			By("By creating a new AzureAppConfigurationProvider")
			mapResult := make(map[string]string)
			mapResult["testKey"] = "testValue"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider"
			configMapName := "configmap-to-be-created"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint:                &EndpointName,
					ReplicaDiscoveryEnabled: false,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())

			providerLookupKey := types.NamespacedName{Name: providerName, Namespace: ProviderNamespace}
			createdProvider := &acpv1.AzureAppConfigurationProvider{}

			// We'll need to retry getting this newly created AppConfiguationProvider, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, providerLookupKey, createdProvider)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(*createdProvider.Spec.Endpoint).Should(Equal(EndpointName))
			Expect(createdProvider.Spec.Target.ConfigMapName).Should(Equal(configMapName))
			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(createdProvider.Status.Phase).Should(Equal(acpv1.PhaseComplete))
		})

		It("Should create new configMap", func() {
			By("By getting multiple configuration settings from AppConfig")
			mapResult := make(map[string]string)
			mapResult["testKey"] = "testValue"
			mapResult["testKey2"] = "testValue2"
			mapResult["testKey3"] = "testValue3"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-2"
			configMapName := "configmap-to-be-created-2"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Labels["foo"]).Should(Equal("fooValue"))
			Expect(configmap.Labels["bar"]).Should(Equal("barValue"))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))
		})

		It("Should create new secret", func() {
			By("By getting multiple secret reference settings from AppConfig")
			secretResult1 := make(map[string][]byte)
			secretResult1["tls.crt"] = []byte("fakeCrt")
			secretResult1["tls.key"] = []byte("fakeKey")

			secretResult2 := make(map[string][]byte)
			secretResult2["testSecretKey"] = []byte("testSecretValue")
			secretResult2["testSecretKey2"] = []byte("testSecretValue2")
			secretResult2["testSecretKey3"] = []byte("testSecretValue3")

			secretName := "secret-to-be-created-3"
			secretName2 := "secret-to-be-created-3-1"
			allSettings := &loader.TargetKeyValueSettings{
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult1,
						Type: corev1.SecretTypeTLS,
					},
					secretName2: {
						Data: secretResult2,
						Type: corev1.SecretTypeOpaque,
					},
				},
				SecretReferences: map[string]*loader.TargetSecretReference{
					secretName: {
						Type:            corev1.SecretTypeTLS,
						SecretsMetadata: make(map[string]loader.KeyVaultSecretMetadata),
					},
					secretName2: {
						Type:            corev1.SecretTypeOpaque,
						SecretsMetadata: make(map[string]loader.KeyVaultSecretMetadata),
					},
				},
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-3"
			configMapName := "configmap-to-be-created-3"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			secretLookupKey := types.NamespacedName{Name: secretName, Namespace: ProviderNamespace}
			secretLookupKey2 := types.NamespacedName{Name: secretName2, Namespace: ProviderNamespace}
			secret := &corev1.Secret{}
			secret2 := &corev1.Secret{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				if err != nil {
					fmt.Print(err.Error())
				}
				return err == nil
			}, time.Second*5, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey2, secret2)
				if err != nil {
					fmt.Print(err.Error())
				}
				return err == nil
			}, time.Second*5, interval).Should(BeTrue())

			Expect(secret.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret.Data["tls.crt"])).Should(Equal("fakeCrt"))
			Expect(string(secret.Data["tls.key"])).Should(Equal("fakeKey"))
			Expect(secret.Type).Should(Equal(corev1.SecretTypeTLS))

			Expect(secret2.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret2.Data["testSecretKey"])).Should(Equal("testSecretValue"))
			Expect(string(secret2.Data["testSecretKey2"])).Should(Equal("testSecretValue2"))
			Expect(string(secret2.Data["testSecretKey3"])).Should(Equal("testSecretValue3"))
			Expect(secret2.Type).Should(Equal(corev1.SecretTypeOpaque))
		})

		It("Should create proper configmap and secret", func() {
			By("By getting normal configuration and secret reference settings from AppConfig")
			configMapResult := make(map[string]string)
			configMapResult["testKey"] = "testValue"
			configMapResult["testKey2"] = "testValue2"
			configMapResult["testKey3"] = "testValue3"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")
			secretResult["testSecretKey2"] = []byte("testSecretValue2")
			secretResult["testSecretKey3"] = []byte("testSecretValue3")

			secretName := "secret-to-be-created-5"
			allSettings := &loader.TargetKeyValueSettings{
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				ConfigMapSettings: configMapResult,
				SecretReferences: map[string]*loader.TargetSecretReference{
					secretName: {
						Type:            corev1.SecretType("Opaque"),
						SecretsMetadata: make(map[string]loader.KeyVaultSecretMetadata),
					},
				},
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-5"
			configMapName := "configmap-to-be-created-5"

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				if err != nil {
					fmt.Print(err.Error())
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			secretLookupKey := types.NamespacedName{Name: secretName, Namespace: ProviderNamespace}
			secret := &corev1.Secret{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			Expect(secret.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret.Data["testSecretKey"])).Should(Equal("testSecretValue"))
			Expect(string(secret.Data["testSecretKey2"])).Should(Equal("testSecretValue2"))
			Expect(string(secret.Data["testSecretKey3"])).Should(Equal("testSecretValue3"))
			Expect(secret.Type).Should(Equal(corev1.SecretType("Opaque")))
		})

		It("Should create file style configMap", func() {
			By("By getting multiple configuration settings from AppConfig")
			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"testKey2\":\"testValue2\",\"testKey3\":\"testValue3\"}"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-7"
			configMapName := "file-style-configmap-to-be-created"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			time.Sleep(time.Second * 5) //Wait few seconds to wait the second round reconcile complete
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Labels["foo"]).Should(Equal("fooValue"))
			Expect(configmap.Labels["bar"]).Should(Equal("barValue"))
			Expect(configmap.Data["filestyle.json"]).Should(Equal("{\"testKey\":\"testValue\",\"testKey2\":\"testValue2\",\"testKey3\":\"testValue3\"}"))
			Expect(len(configmap.Data)).Should(Equal(1))
		})

		It("Should create file style ConfigMap with feature flag settings", func() {
			By("By getting multiple configuration settings and feature flags from AppConfig")
			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-8"
			configMapName := "file-style-configmap-to-be-created-2"
			wildcard := "*"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &wildcard,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			time.Sleep(time.Second * 5) //Wait few seconds to wait the second round reconcile complete
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Data["filestyle.json"]).Should(Equal("{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"))
			Expect(len(configmap.Data)).Should(Equal(1))
		})

		It("Should refresh configMap", func() {
			By("By updating the provider and trigger reconciliation")
			mapResult := make(map[string]string)
			mapResult["testKey"] = "testValue"
			mapResult["testKey2"] = "testValue2"
			mapResult["testKey3"] = "testValue3"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "refresh-appconfigurationprovider-1"
			configMapName := "configmap-to-be-refresh-1"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Labels["foo"]).Should(Equal("fooValue"))
			Expect(configmap.Labels["bar"]).Should(Equal("barValue"))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			newEndpoint := "https://fake-endpoint-2"

			mapResult2 := make(map[string]string)
			mapResult2["testKey"] = "newtestValue"
			mapResult2["testKey2"] = "newtestValue2"
			mapResult2["testKey3"] = "newtestValue3"

			allSettings2 := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult2,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings2, nil)

			_ = k8sClient.Get(ctx, types.NamespacedName{Name: providerName, Namespace: ProviderNamespace}, configProvider)
			configProvider.Spec.Endpoint = &newEndpoint

			Expect(k8sClient.Update(ctx, configProvider)).Should(Succeed())

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Data["testKey"]).Should(Equal("newtestValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("newtestValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("newtestValue3"))

		})

		It("Should refresh configMap", func() {
			By("By sentinel value updated in Azure App Configuration")
			mapResult := make(map[string]string)
			mapResult["testKey"] = "testValue"
			mapResult["testKey2"] = "testValue2"
			mapResult["testKey3"] = "testValue3"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			mapResult2 := make(map[string]string)
			mapResult2["testKey"] = "newtestValue"
			mapResult2["testKey2"] = "newtestValue2"
			mapResult2["testKey3"] = "newtestValue3"

			allSettings2 := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult2,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)
			mockConfigurationSettings.EXPECT().CheckAndRefreshSentinels(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings2, nil)

			ctx := context.Background()
			providerName := "refresh-appconfigurationprovider-2"
			configMapName := "configmap-to-be-refresh-2"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Interval: "5s",
							Enabled:  true,
							Monitoring: &acpv1.RefreshMonitoring{
								Sentinels: []acpv1.Sentinel{
									{Key: "testKey", Label: "testLabel"},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Labels["foo"]).Should(Equal("fooValue"))
			Expect(configmap.Labels["bar"]).Should(Equal("barValue"))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			time.Sleep(6 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Data["testKey"]).Should(Equal("newtestValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("newtestValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("newtestValue3"))

			_ = k8sClient.Delete(ctx, configProvider)
		})

		It("Should refresh secret when data change", func() {
			By("By enabling refresh on secret")
			configMapResult := make(map[string]string)
			configMapResult["testKey"] = "testValue"
			configMapResult["testKey2"] = "testValue2"
			configMapResult["testKey3"] = "testValue3"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")

			secretName := "secret-to-be-refreshed-3"
			var fakeId azsecrets.ID = "fakeSecretId"
			secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			secretMetadata["testSecretKey"] = loader.KeyVaultSecretMetadata{
				SecretId: &fakeId,
			}

			allSettings := &loader.TargetKeyValueSettings{
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				ConfigMapSettings: configMapResult,
				SecretReferences: map[string]*loader.TargetSecretReference{
					secretName: {
						Type:            corev1.SecretType("Opaque"),
						SecretsMetadata: secretMetadata,
					},
				},
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "refresh-appconfigurationprovider-3"
			configMapName := "configmap-to-be-refreshed-3"

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
						Refresh: &acpv1.RefreshSettings{
							Interval: "1m",
							Enabled:  true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				if err != nil {
					fmt.Print(err.Error())
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

			secretLookupKey := types.NamespacedName{Name: secretName, Namespace: ProviderNamespace}
			secret := &corev1.Secret{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			Expect(secret.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret.Data["testSecretKey"])).Should(Equal("testSecretValue"))
			Expect(secret.Type).Should(Equal(corev1.SecretType("Opaque")))

			newSecretResult := make(map[string][]byte)
			newSecretResult["testSecretKey"] = []byte("newTestSecretValue")

			newResolvedSecret := map[string]corev1.Secret{
				secretName: {
					Data: newSecretResult,
					Type: corev1.SecretType("Opaque"),
				},
			}

			var newFakeId azsecrets.ID = "newFakeSecretId"
			newSecretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			newSecretMetadata["testSecretKey"] = loader.KeyVaultSecretMetadata{
				SecretId: &newFakeId,
			}
			mockedSecretReference := make(map[string]*loader.TargetSecretReference)
			mockedSecretReference[secretName] = &loader.TargetSecretReference{
				Type:            corev1.SecretType("Opaque"),
				SecretsMetadata: newSecretMetadata,
			}

			newTargetSettings := &loader.TargetKeyValueSettings{
				SecretSettings:   newResolvedSecret,
				SecretReferences: mockedSecretReference,
			}

			mockConfigurationSettings.EXPECT().ResolveSecretReferences(gomock.Any(), gomock.Any(), gomock.Any()).Return(newTargetSettings, nil)
			// Refresh interval is 1 minute, wait for 65 seconds to make sure the refresh is triggered
			time.Sleep(65 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(secret.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret.Data["testSecretKey"])).Should(Equal("newTestSecretValue"))
			Expect(secret.Type).Should(Equal(corev1.SecretType("Opaque")))

			// Mocked secret refresh scenario when secretMetadata is not changed
			newTargetSettings2 := &loader.TargetKeyValueSettings{
				SecretSettings:   make(map[string]corev1.Secret),
				SecretReferences: mockedSecretReference,
			}

			mockConfigurationSettings.EXPECT().ResolveSecretReferences(gomock.Any(), gomock.Any(), gomock.Any()).Return(newTargetSettings2, nil)
			// Refresh interval is 1 minute, wait for 65 seconds to make sure the refresh is triggered
			time.Sleep(65 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(secret.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret.Data["testSecretKey"])).Should(Equal("newTestSecretValue"))
			Expect(secret.Type).Should(Equal(corev1.SecretType("Opaque")))
		})

		It("Should refresh configMap by watching all keys", func() {
			By("When key values updated in Azure App Configuration")
			mapResult := make(map[string]string)
			mapResult["testKey"] = "testValue"
			mapResult["testKey2"] = "testValue2"
			mapResult["testKey3"] = "testValue3"

			keyValueEtags := make(map[acpv1.Selector][]*azcore.ETag)
			featureFlagEtags := make(map[acpv1.Selector][]*azcore.ETag)
			keyFilter := "*"
			keyValueEtags[acpv1.Selector{KeyFilter: &keyFilter}] = []*azcore.ETag{}

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     keyValueEtags,
				FeatureFlagETags:  featureFlagEtags,
			}

			mapResult2 := make(map[string]string)
			mapResult2["testKey"] = "newtestValue"
			mapResult2["testKey2"] = "newtestValue2"
			mapResult2["testKey3"] = "newtestValue3"

			allSettings2 := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult2,
				KeyValueETags:     keyValueEtags,
				FeatureFlagETags:  featureFlagEtags,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings2, nil)

			ctx := context.Background()
			providerName := "refresh-appconfigurationprovider-4"
			configMapName := "configmap-to-be-refresh-4"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Interval: "5s",
							Enabled:  true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Labels["foo"]).Should(Equal("fooValue"))
			Expect(configmap.Labels["bar"]).Should(Equal("barValue"))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			time.Sleep(6 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Data["testKey"]).Should(Equal("newtestValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("newtestValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("newtestValue3"))

			_ = k8sClient.Delete(ctx, configProvider)
		})

		It("Should not refresh configMap by watching all keys", func() {
			By("When key values not updated in Azure App Configuration")
			mapResult := make(map[string]string)
			keyValueEtags := make(map[acpv1.Selector][]*azcore.ETag)
			featureFlagEtags := make(map[acpv1.Selector][]*azcore.ETag)
			mapResult["testKey"] = "testValue"
			mapResult["testKey2"] = "testValue2"
			mapResult["testKey3"] = "testValue3"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     keyValueEtags,
				FeatureFlagETags:  featureFlagEtags,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)

			ctx := context.Background()
			providerName := "refresh-appconfigurationprovider-5"
			configMapName := "configmap-to-be-refresh-5"
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      providerName,
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Interval: "5s",
							Enabled:  true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			configmapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ProviderNamespace}
			configmap := &corev1.ConfigMap{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Name).Should(Equal(configMapName))
			Expect(configmap.Namespace).Should(Equal(ProviderNamespace))
			Expect(configmap.Labels["foo"]).Should(Equal("fooValue"))
			Expect(configmap.Labels["bar"]).Should(Equal("barValue"))
			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			time.Sleep(6 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, configmapLookupKey, configmap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(configmap.Data["testKey"]).Should(Equal("testValue"))
			Expect(configmap.Data["testKey2"]).Should(Equal("testValue2"))
			Expect(configmap.Data["testKey3"]).Should(Equal("testValue3"))

			_ = k8sClient.Delete(ctx, configProvider)
		})
	})

	Context("Verify exist non escaped value in label", func() {
		It("Should return false if all character is escaped", func() {
			Expect(hasNonEscapedValueInLabel(`some\,valid\,label`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`somevalidlabel`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel("")).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`some\*`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\\some\,\*\valid\,\label\*`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\,`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\\`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`'\`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\\\,`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\a\\\,`)).Should(BeFalse())
			Expect(hasNonEscapedValueInLabel(`\\\\\\\,`)).Should(BeFalse())
		})

		It("Should return true if any character is not escaped", func() {
			Expect(hasNonEscapedValueInLabel(`some\,invalid,label`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`,`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`some,,value\\`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`some\,,value`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`some\,*value`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`\\,`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`\x\,y\\,z`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`\x\,\y*`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`\x\*\\\*some\\*value`)).Should(BeTrue())
			Expect(hasNonEscapedValueInLabel(`\,\\\\,`)).Should(BeTrue())
		})
	})

	Context("Verify spec object", func() {
		It("Should return error if both endpoint and connectionStringReference are set", func() {
			configMapName := "test-configmap"
			connectionStringReference := "fakeSecret"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                  &EndpointName,
				ConnectionStringReference: &connectionStringReference,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec: both endpoint and connectionStringReference field are set"))
		})

		It("Should return error if configMapData key is set when type is default", func() {
			configMapName := "test-configmap"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
					ConfigMapData: &acpv1.ConfigMapDataOptions{
						Type: acpv1.Default,
						Key:  "testKey",
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData.key: key field is not allowed when type is default"))
		})

		It("Should return error if configMapData key is not set when type is not default", func() {
			configMapName := "test-configmap"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
					ConfigMapData: &acpv1.ConfigMapDataOptions{
						Type: acpv1.Json,
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData.key: key field is required when type is json, yaml or properties"))
		})

		It("Should return error if configMapData separator is set when type is default", func() {
			configMapName := "test-configmap"
			delimiter := "."
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
					ConfigMapData: &acpv1.ConfigMapDataOptions{
						Type:      acpv1.Default,
						Separator: &delimiter,
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData.separator: separator field is not allowed when type is default"))
		})

		It("Should return error if configMapData separator is set when type is properties", func() {
			configMapName := "test-configmap"
			delimiter := "."
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
					ConfigMapData: &acpv1.ConfigMapDataOptions{
						Type:      acpv1.Properties,
						Key:       "testKey",
						Separator: &delimiter,
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData.separator: separator field is not allowed when type is properties"))
		})

		It("Should return error if feature flag is set when data type is default", func() {
			configMapName := "test-configmap"
			testKey := "testKey"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter: &testKey,
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData: configMap data type must be json or yaml when FeatureFlag is set"))
		})

		It("Should return error if feature flag is set when data type is properties", func() {
			configMapName := "test-configmap"
			testKeyFilter := "testKeyFilter"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
					ConfigMapData: &acpv1.ConfigMapDataOptions{
						Type: acpv1.Properties,
						Key:  "testKey",
					},
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter: &testKeyFilter,
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData: configMap data type must be json or yaml when FeatureFlag is set"))
		})

		It("Should return error if feature flag selector is not set", func() {
			configMapName := "test-configmap"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
					ConfigMapData: &acpv1.ConfigMapDataOptions{
						Type: acpv1.Json,
						Key:  "testKey",
					},
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.featureFlag.selectors: featureFlag.selectors must be specified when FeatureFlag is set"))
		})

		It("Should return error if both endpoint and connectionStringReference are not set", func() {
			configMapName := "test-configmap"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec: one of endpoint and connectionStringReference field must be set"))
		})

		It("Should return error when both connectionStringReference and auth object are set", func() {
			configMapName := "test-configmap"
			connectionStringReference := "fakeSecret"
			uuid1 := "86c613ca-b977-11ed-afa1-0242ac120002"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				ConnectionStringReference: &connectionStringReference,
				Auth: &acpv1.AzureAppConfigurationProviderAuth{
					ManagedIdentityClientId: &uuid1,
				},
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.auth: auth field is not allowed when connectionStringReference field is set"))
		})

		It("Should return error when duplicated sentinel key are set", func() {
			configMapName := "test-configmap"
			connectionStringReference := "fakeSecret"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				ConnectionStringReference: &connectionStringReference,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Refresh: &acpv1.DynamicConfigurationRefreshParameters{
						Monitoring: &acpv1.RefreshMonitoring{
							Sentinels: []acpv1.Sentinel{
								{
									Key:   "testKey",
									Label: "testValue",
								},
								{
									Key:   "testKey",
									Label: "testValue1",
								},
								{
									Key:   "testKey",
									Label: "testValue",
								},
							},
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.configuration.refresh.monitoring.keyValues: monitoring duplicated key 'testKey'"))
		})

		It("Should return no error when all sentinel are unique", func() {
			configMapName := "test-configmap"
			connectionStringReference := "fakeSecret"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				ConnectionStringReference: &connectionStringReference,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Refresh: &acpv1.DynamicConfigurationRefreshParameters{
						Monitoring: &acpv1.RefreshMonitoring{
							Sentinels: []acpv1.Sentinel{
								{
									Key:   "testKey",
									Label: "\x00",
								},
								{
									Key:   "testKey",
									Label: "testValue1",
								},
								{
									Key:   "testKey2",
									Label: "\x00",
								},
								{
									Key:   "testKey2",
									Label: "",
								},
							},
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec)).Should(BeNil())
		})

		It("Should return error when incorrectly configure the selector", func() {
			configMapName := "test-configmap"
			connectionStringReference := "fakeSecret"
			testKey := "testKey"
			testSnapshot := "testSnapshot"
			testLabel := "testLabel"
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				ConnectionStringReference: &connectionStringReference,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter:    &testKey,
							SnapshotName: &testSnapshot,
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.configuration.selectors: set both 'keyFilter' and 'snapshotName' in one selector causes ambiguity, only one of them should be set"))

			configProviderSpec2 := acpv1.AzureAppConfigurationProviderSpec{
				ConnectionStringReference: &connectionStringReference,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Selectors: []acpv1.Selector{
						{
							SnapshotName: &testSnapshot,
							LabelFilter:  &testLabel,
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec2).Error()).Should(Equal("spec.configuration.selectors: 'labelFilter' is not allowed when 'snapshotName' is set"))

			configProviderSpec3 := acpv1.AzureAppConfigurationProviderSpec{
				ConnectionStringReference: &connectionStringReference,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Selectors: []acpv1.Selector{
						{
							LabelFilter: &testLabel,
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec3).Error()).Should(Equal("spec.configuration.selectors: a selector uses 'labelFilter' but misses the 'keyFilter', 'keyFilter' is required for key-label pair filtering"))
		})
	})

	Context("Verify auth object", func() {
		It("Should return no error if auth object is valid", func() {
			os.Setenv("WORKLOAD_IDENTITY_ENABLED", "true")

			uuid1 := "86c613ca-b977-11ed-afa1-0242ac120002"
			secretName := "fakeName1"
			configMapName := "fakeName2"
			serviceAccountName := "fakeName3"
			key := "fakeKey"
			authObj := &acpv1.AzureAppConfigurationProviderAuth{}
			authObj2 := &acpv1.AzureAppConfigurationProviderAuth{
				ManagedIdentityClientId: &uuid1,
			}
			authObj3 := &acpv1.AzureAppConfigurationProviderAuth{
				ServicePrincipalReference: &secretName,
			}
			authObj4 := &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
					ManagedIdentityClientId: &uuid1,
				},
			}
			authObj5 := &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
					ManagedIdentityClientIdReference: &acpv1.ManagedIdentityReferenceParameters{
						ConfigMap: configMapName,
						Key:       key,
					},
				},
			}
			authObj6 := &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
					ServiceAccountName: &serviceAccountName,
				},
			}
			Expect(verifyAuthObject(nil)).Should(BeNil())
			Expect(verifyAuthObject(authObj)).Should(BeNil())
			Expect(verifyAuthObject(authObj2)).Should(BeNil())
			Expect(verifyAuthObject(authObj3)).Should(BeNil())
			Expect(verifyAuthObject(authObj4)).Should(BeNil())
			Expect(verifyAuthObject(authObj5)).Should(BeNil())
			Expect(verifyAuthObject(authObj6)).Should(BeNil())
		})

		It("Should return error if auth object is not valid", func() {
			uuid1 := "not-a-uuid"
			uuid2 := "86c613ca-b977-11ed-afa1-0242ac120002"
			secretName := "fakeName1"
			configMapName := "fakeName2"
			key := "fakeKey"
			authObj := &acpv1.AzureAppConfigurationProviderAuth{
				ManagedIdentityClientId: &uuid1,
			}
			authObj2 := &acpv1.AzureAppConfigurationProviderAuth{
				ManagedIdentityClientId:   &uuid2,
				ServicePrincipalReference: &secretName,
			}
			authObj3 := &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
					ManagedIdentityClientId: &uuid2,
					ManagedIdentityClientIdReference: &acpv1.ManagedIdentityReferenceParameters{
						ConfigMap: configMapName,
						Key:       key,
					},
				},
			}
			authObj4 := &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{
					ManagedIdentityClientId: &uuid1,
				},
			}
			authObj5 := &acpv1.AzureAppConfigurationProviderAuth{
				WorkloadIdentity: &acpv1.WorkloadIdentityParameters{},
			}
			Expect(verifyAuthObject(authObj).Error()).Should(Equal("auth: ManagedIdentityClientId \"not-a-uuid\" in auth field is not a valid uuid"))
			Expect(verifyAuthObject(authObj2).Error()).Should(Equal("auth: more than one authentication methods are specified in 'auth' field"))
			Expect(verifyAuthObject(authObj3).Error()).Should(Equal("auth.workloadIdentity: setting only one of 'managedIdentityClientId', 'managedIdentityClientIdReference' or 'serviceAccountName' field is allowed"))
			Expect(verifyAuthObject(authObj4).Error()).Should(Equal("auth.workloadIdentity.managedIdentityClientId: managedIdentityClientId \"not-a-uuid\" in auth.workloadIdentity is not a valid uuid"))
			Expect(verifyAuthObject(authObj5).Error()).Should(Equal("auth.workloadIdentity: setting one of 'managedIdentityClientId', 'managedIdentityClientIdReference' or 'serviceAccountName' field is required"))
		})
	})

	Context("Verify the existing configMap", func() {
		It("Should return no error if existing configMap is valid", func() {
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "providerName",
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: "configMapName",
					},
				},
			}

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configMapName",
					Namespace: ProviderNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "AzureAppConfigurationProvider", Name: "providerName"},
					},
				},
			}

			configMap2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "anotherConfigMap",
					Namespace: ProviderNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "AzureAppConfigurationProvider", Name: "providerName"},
					},
				},
			}
			Expect(verifyExistingTargetObject(&corev1.ConfigMap{}, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(BeNil())
			Expect(verifyExistingTargetObject(configMap, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(BeNil())
			Expect(verifyExistingTargetObject(configMap2, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(BeNil())

		})

		It("Should return error if configMap is not valid", func() {
			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "providerName",
					Namespace: ProviderNamespace,
					Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: "configMapName",
					},
				},
			}

			configMap1 := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configMapName",
					Namespace: ProviderNamespace,
				},
			}
			configMap2 := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configMapName",
					Namespace: ProviderNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "AzureAppConfigurationProvider", Name: "anotherProvider"},
					},
				},
			}
			Expect(verifyExistingTargetObject(configMap1, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(MatchError("a ConfigMap with name 'configMapName' already exists in namespace 'default'"))
			Expect(verifyExistingTargetObject(configMap2, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(MatchError("a ConfigMap with name 'configMapName' already exists in namespace 'default'"))
		})
	})
})
