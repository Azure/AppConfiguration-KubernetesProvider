// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"azappconfig/provider/internal/loader"
	"context"
	"fmt"
	"time"

	acpv1 "azappconfig/provider/api/v1"

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
					Endpoint: &EndpointName,
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
			mapResult := make(map[string][]byte)
			mapResult["testSecretKey"] = []byte("testValue")
			mapResult["testSecretKey2"] = []byte("testValue2")
			mapResult["testSecretKey3"] = []byte("testValue3")

			allSettings := &loader.TargetKeyValueSettings{
				SecretSettings: mapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-3"
			configMapName := "configmap-to-be-created-3"
			secretName := "secret-to-be-created-3"
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
					Secret: &acpv1.AzureKeyVaultReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, configProvider)).Should(Succeed())
			secretLookupKey := types.NamespacedName{Name: secretName, Namespace: ProviderNamespace}
			secret := &corev1.Secret{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secretLookupKey, secret)
				if err != nil {
					fmt.Print(err.Error())
				}
				return err == nil
			}, time.Second*20, interval).Should(BeTrue())

			Expect(secret.Namespace).Should(Equal(ProviderNamespace))
			Expect(string(secret.Data["testSecretKey"])).Should(Equal("testValue"))
			Expect(string(secret.Data["testSecretKey2"])).Should(Equal("testValue2"))
			Expect(string(secret.Data["testSecretKey3"])).Should(Equal("testValue3"))
			Expect(secret.Type).Should(Equal(corev1.SecretType("Opaque")))
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

			allSettings := &loader.TargetKeyValueSettings{
				SecretSettings:    secretResult,
				ConfigMapSettings: configMapResult,
			}

			mockConfigurationSettings.EXPECT().CreateTargetSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-5"
			configMapName := "configmap-to-be-created-5"
			secretName := "secret-to-be-created-5"
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
					Secret: &acpv1.AzureKeyVaultReference{
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

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec: Both endpoint and connectionStringReference field are set"))
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
			configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: configMapName,
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.KeyLabelSelector{
						{
							KeyFilter: "testKey",
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData: target.configMapData.type must be json or yaml when FeatureFlag is set"))
		})

		It("Should return error if feature flag is set when data type is properties", func() {
			configMapName := "test-configmap"
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
					Selectors: []acpv1.KeyLabelSelector{
						{
							KeyFilter: "testKeyFilter",
						},
					},
				},
			}

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec.target.configMapData: target.configMapData.type must be json or yaml when FeatureFlag is set"))
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

			Expect(verifyObject(configProviderSpec).Error()).Should(Equal("spec: One of endpoint and connectionStringReference field must be set"))
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
	})

	Context("Verify auth object", func() {
		It("Should return no error if auth object is valid", func() {
			uuid1 := "86c613ca-b977-11ed-afa1-0242ac120002"
			secretName := "fakeName1"
			configMapName := "fakeName2"
			key := "fakeKey"
			authObj := &acpv1.AzureAppConfigurationProviderAuth{}
			authObj2 := &acpv1.AzureAppConfigurationProviderAuth{
				ManagedIdentityClientId: &uuid1,
			}
			authObj3 := &acpv1.AzureAppConfigurationProviderAuth{
				ServicePrincipalReference: &secretName,
			}
			autoObj4 := &acpv1.AzureAppConfigurationProviderAuth{
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
			Expect(verifyAuthObject(nil)).Should(BeNil())
			Expect(verifyAuthObject(authObj)).Should(BeNil())
			Expect(verifyAuthObject(authObj2)).Should(BeNil())
			Expect(verifyAuthObject(authObj3)).Should(BeNil())
			Expect(verifyAuthObject(autoObj4)).Should(BeNil())
			Expect(verifyAuthObject(authObj5)).Should(BeNil())
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
			Expect(verifyAuthObject(authObj3).Error()).Should(Equal("auth.workloadIdentity: only one of managedIdentityClientId and managedIdentityClientIdReference is allowed"))
			Expect(verifyAuthObject(authObj4).Error()).Should(Equal("auth.workloadIdentity.managedIdentityClientId: managedIdentityClientId \"not-a-uuid\" in auth.workloadIdentity is not a valid uuid"))
			Expect(verifyAuthObject(authObj5).Error()).Should(Equal("auth.workloadIdentity: one of managedIdentityClientId and managedIdentityClientIdReference is required"))
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
			Expect(verifyExistingTargetObject(configMap1, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(MatchError("A ConfigMap with name 'configMapName' already exists in namespace 'default'"))
			Expect(verifyExistingTargetObject(configMap2, configProvider.Spec.Target.ConfigMapName, configProvider.Name)).Should(MatchError("A ConfigMap with name 'configMapName' already exists in namespace 'default'"))
		})
	})
})
