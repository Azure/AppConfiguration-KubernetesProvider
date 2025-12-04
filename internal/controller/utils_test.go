// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	acpv1 "azappconfig/provider/api/v1"
	"azappconfig/provider/internal/loader"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldCreateOrUpdateSecret(t *testing.T) {
	// Test case 1: Secret does not exist
	processor1 := createTestProcessor("test-secret")
	existingSecrets1 := make(map[string]corev1.Secret)

	result1 := shouldCreateOrUpdateSecret(processor1, "test-secret", existingSecrets1)
	assert.True(t, result1, "Should create secret when it doesn't exist")

	// Test case 2: Secret exists with different data (Default type)
	processor2 := createTestProcessor("test-secret")
	existingSecrets2 := make(map[string]corev1.Secret)
	existingSecrets2["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"key1": []byte("different-value"),
			"key2": []byte("value2"),
		},
	}

	result2 := shouldCreateOrUpdateSecret(processor2, "test-secret", existingSecrets2)
	assert.True(t, result2, "Should update secret when data is different (Default type)")

	// Test case 3: Secret exists with the same data (Default type)
	processor3 := createTestProcessor("test-secret")
	existingSecrets3 := make(map[string]corev1.Secret)
	existingSecrets3["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"key1": []byte("value1"),
			"key2": []byte("value2"),
		},
	}

	result3 := shouldCreateOrUpdateSecret(processor3, "test-secret", existingSecrets3)
	assert.False(t, result3, "Should not update secret when data is the same (Default type)")

	// Test case 4: Secret exists with different data (Properties type)
	processor4 := createTestProcessorWithDataType("test-secret", acpv1.Properties, "secrets.properties")
	existingSecrets4 := make(map[string]corev1.Secret)
	existingSecrets4["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"secrets.properties": []byte("key1=differentvalue\nkey2=value2"),
		},
	}

	result4 := shouldCreateOrUpdateSecret(processor4, "test-secret", existingSecrets4)
	assert.True(t, result4, "Should update secret when data is different (Properties type)")

	// Test case 5: Secret exists with the same data (JSON type)
	processor5 := createTestProcessorWithDataType("test-secret", acpv1.Json, "secrets.json")

	// Create processor's secret settings with JSON data
	jsonData := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	jsonBytes, _ := json.Marshal(jsonData)
	processor5.Settings.SecretSettings["test-secret"] = corev1.Secret{
		Data: map[string][]byte{
			"secrets.json": jsonBytes,
		},
	}

	// Create existing secrets with same JSON data
	existingSecrets5 := make(map[string]corev1.Secret)
	existingSecrets5["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"secrets.json": jsonBytes,
		},
	}

	result5 := shouldCreateOrUpdateSecret(processor5, "test-secret", existingSecrets5)
	assert.False(t, result5, "Should not update secret when JSON data is the same")

	// Test case 6: Secret exists with different data (JSON type)
	processor6 := createTestProcessorWithDataType("test-secret", acpv1.Json, "secrets.json")

	// Create processor's secret settings with JSON data
	jsonData6 := map[string]interface{}{
		"key1": "value1",
		"key2": "updated-value",
	}
	jsonBytes6, _ := json.Marshal(jsonData6)
	processor6.Settings.SecretSettings["test-secret"] = corev1.Secret{
		Data: map[string][]byte{
			"secrets.json": jsonBytes6,
		},
	}

	// Create existing secrets with different JSON data
	existingJsonData6 := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	existingJsonBytes6, _ := json.Marshal(existingJsonData6)
	existingSecrets6 := make(map[string]corev1.Secret)
	existingSecrets6["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"secrets.json": existingJsonBytes6,
		},
	}

	result6 := shouldCreateOrUpdateSecret(processor6, "test-secret", existingSecrets6)
	assert.True(t, result6, "Should update secret when JSON data is different")

	// Test case 7: Secret exists with the same data (YAML type)
	processor7 := createTestProcessorWithDataType("test-secret", acpv1.Yaml, "secrets.yaml")

	// Create processor's secret settings with YAML data
	yamlData := map[string]interface{}{
		"key1": "value1",
		"nested": map[string]interface{}{
			"key2": "value2",
		},
	}
	yamlBytes, _ := yaml.Marshal(yamlData)
	processor7.Settings.SecretSettings["test-secret"] = corev1.Secret{
		Data: map[string][]byte{
			"secrets.yaml": yamlBytes,
		},
	}

	// Create existing secrets with same YAML data
	existingSecrets7 := make(map[string]corev1.Secret)
	existingSecrets7["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"secrets.yaml": yamlBytes,
		},
	}

	result7 := shouldCreateOrUpdateSecret(processor7, "test-secret", existingSecrets7)
	assert.False(t, result7, "Should not update secret when YAML data is the same")

	// Test case 8: Secret exists with different data (YAML type)
	processor8 := createTestProcessorWithDataType("test-secret", acpv1.Yaml, "secrets.yaml")

	// Create processor's secret settings with YAML data
	yamlData8 := map[string]interface{}{
		"key1": "value1",
		"nested": map[string]interface{}{
			"key2": "updated-value",
		},
	}
	yamlBytes8, _ := yaml.Marshal(yamlData8)
	processor8.Settings.SecretSettings["test-secret"] = corev1.Secret{
		Data: map[string][]byte{
			"secrets.yaml": yamlBytes8,
		},
	}

	// Create existing secrets with different YAML data
	existingYamlData8 := map[string]interface{}{
		"key1": "value1",
		"nested": map[string]interface{}{
			"key2": "value2",
		},
	}
	existingYamlBytes8, _ := yaml.Marshal(existingYamlData8)
	existingSecrets8 := make(map[string]corev1.Secret)
	existingSecrets8["test-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret"},
		Data: map[string][]byte{
			"secrets.yaml": existingYamlBytes8,
		},
	}

	result8 := shouldCreateOrUpdateSecret(processor8, "test-secret", existingSecrets8)
	assert.True(t, result8, "Should update secret when YAML data is different")

	// Test case 9: Secret name doesn't match the one in the provider spec
	processor9 := createTestProcessor("configured-secret")
	existingSecrets9 := make(map[string]corev1.Secret)
	existingSecrets9["different-secret"] = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "different-secret"},
		Data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}

	result9 := shouldCreateOrUpdateSecret(processor9, "different-secret", existingSecrets9)
	assert.True(t, result9, "Should create secret when name doesn't match the provider spec")
}

// Helper function to create a test processor with default data type
func createTestProcessor(secretName string) *AppConfigurationProviderProcessor {
	processor := &AppConfigurationProviderProcessor{
		Provider: &acpv1.AzureAppConfigurationProvider{
			Spec: acpv1.AzureAppConfigurationProviderSpec{
				Secret: &acpv1.SecretReference{
					Target: acpv1.SecretGenerationParameters{
						SecretName: secretName,
					},
				},
			},
		},
		Settings: &loader.TargetKeyValueSettings{
			SecretSettings: map[string]corev1.Secret{
				secretName: {
					Data: map[string][]byte{
						"key1": []byte("value1"),
						"key2": []byte("value2"),
					},
				},
			},
		},
	}
	return processor
}

// Helper function to create a test processor with specified data type
func createTestProcessorWithDataType(secretName string, dataType acpv1.DataType, key string) *AppConfigurationProviderProcessor {
	processor := &AppConfigurationProviderProcessor{
		Provider: &acpv1.AzureAppConfigurationProvider{
			Spec: acpv1.AzureAppConfigurationProviderSpec{
				Secret: &acpv1.SecretReference{
					Target: acpv1.SecretGenerationParameters{
						SecretName: secretName,
						SecretData: &acpv1.DataOptions{
							Type: dataType,
							Key:  key,
						},
					},
				},
			},
		},
		Settings: &loader.TargetKeyValueSettings{
			SecretSettings: map[string]corev1.Secret{
				secretName: {
					Data: map[string][]byte{},
				},
			},
		},
	}
	return processor
}

func TestShouldCreateOrUpdateConfigMap(t *testing.T) {
	// Test case 1: ConfigMap is nil
	existingConfigMap1 := (*corev1.ConfigMap)(nil)
	latestConfigMapSettings1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	dataOptions1 := (*acpv1.DataOptions)(nil)

	result1 := shouldCreateOrUpdateConfigMap(existingConfigMap1, latestConfigMapSettings1, dataOptions1)
	assert.True(t, result1, "Should create ConfigMap when it doesn't exist")

	// Test case 2: ConfigMap Data is nil
	existingConfigMap2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data:       nil,
	}
	latestConfigMapSettings2 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	dataOptions2 := (*acpv1.DataOptions)(nil)

	result2 := shouldCreateOrUpdateConfigMap(existingConfigMap2, latestConfigMapSettings2, dataOptions2)
	assert.True(t, result2, "Should update ConfigMap when existing data is nil")

	// Test case 3: ConfigMap exists with different data (Default type)
	existingConfigMap3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"key1": "different-value",
			"key2": "value2",
		},
	}
	latestConfigMapSettings3 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	dataOptions3 := (*acpv1.DataOptions)(nil)

	result3 := shouldCreateOrUpdateConfigMap(existingConfigMap3, latestConfigMapSettings3, dataOptions3)
	assert.True(t, result3, "Should update ConfigMap when data is different (Default type)")

	// Test case 4: ConfigMap exists with the same data (Default type)
	existingConfigMap4 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	latestConfigMapSettings4 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	dataOptions4 := (*acpv1.DataOptions)(nil)

	result4 := shouldCreateOrUpdateConfigMap(existingConfigMap4, latestConfigMapSettings4, dataOptions4)
	assert.False(t, result4, "Should not update ConfigMap when data is the same (Default type)")

	// Test case 5: ConfigMap exists with different data (Properties type)
	existingConfigMap5 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"config.properties": "key1=differentvalue\nkey2=value2",
		},
	}
	latestConfigMapSettings5 := map[string]string{
		"config.properties": "key1=value1\nkey2=value2",
	}
	dataOptions5 := &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "config.properties",
	}

	result5 := shouldCreateOrUpdateConfigMap(existingConfigMap5, latestConfigMapSettings5, dataOptions5)
	assert.True(t, result5, "Should update ConfigMap when data is different (Properties type)")

	// Test case 6: ConfigMap exists with the same data (Properties type)
	existingConfigMap6 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"config.properties": "key1=value1\nkey2=value2",
		},
	}
	latestConfigMapSettings6 := map[string]string{
		"config.properties": "key1=value1\nkey2=value2",
	}
	dataOptions6 := &acpv1.DataOptions{
		Type: acpv1.Properties,
		Key:  "config.properties",
	}

	result6 := shouldCreateOrUpdateConfigMap(existingConfigMap6, latestConfigMapSettings6, dataOptions6)
	assert.False(t, result6, "Should not update ConfigMap when data is the same (Properties type)")

	// Test case 7: Key changed in dataOptions
	existingConfigMap7 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"old-key.json": "{\"key1\":\"value1\",\"key2\":\"value2\"}",
		},
	}
	latestConfigMapSettings7 := map[string]string{
		"new-key.json": "{\"key1\":\"value1\",\"key2\":\"value2\"}",
	}
	dataOptions7 := &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "new-key.json", // Changed key
	}

	result7 := shouldCreateOrUpdateConfigMap(existingConfigMap7, latestConfigMapSettings7, dataOptions7)
	assert.True(t, result7, "Should update ConfigMap when key has changed")

	// Test case 8: ConfigMap exists with the same data (JSON type)
	jsonData8 := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]string{
			"subkey1": "subvalue1",
		},
	}
	jsonString8, _ := json.Marshal(jsonData8)

	existingConfigMap8 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"config.json": string(jsonString8),
		},
	}
	latestConfigMapSettings8 := map[string]string{
		"config.json": string(jsonString8),
	}
	dataOptions8 := &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "config.json",
	}

	result8 := shouldCreateOrUpdateConfigMap(existingConfigMap8, latestConfigMapSettings8, dataOptions8)
	assert.False(t, result8, "Should not update ConfigMap when JSON data is the same")

	// Test case 9: ConfigMap exists with different data (JSON type)
	jsonData9a := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]string{
			"subkey1": "subvalue1",
		},
	}
	jsonString9a, _ := json.Marshal(jsonData9a)

	jsonData9b := map[string]interface{}{
		"key1": "value1",
		"key2": "updated-value", // Changed value
		"nested": map[string]string{
			"subkey1": "subvalue1",
		},
	}
	jsonString9b, _ := json.Marshal(jsonData9b)

	existingConfigMap9 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"config.json": string(jsonString9a),
		},
	}
	latestConfigMapSettings9 := map[string]string{
		"config.json": string(jsonString9b),
	}
	dataOptions9 := &acpv1.DataOptions{
		Type: acpv1.Json,
		Key:  "config.json",
	}

	result9 := shouldCreateOrUpdateConfigMap(existingConfigMap9, latestConfigMapSettings9, dataOptions9)
	assert.True(t, result9, "Should update ConfigMap when JSON data is different")

	// Test case 10: ConfigMap exists with the same data (YAML type)
	yamlData10 := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]interface{}{
			"subkey1": "subvalue1",
		},
	}
	yamlBytes10, _ := yaml.Marshal(yamlData10)

	existingConfigMap10 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"config.yaml": string(yamlBytes10),
		},
	}
	latestConfigMapSettings10 := map[string]string{
		"config.yaml": string(yamlBytes10),
	}
	dataOptions10 := &acpv1.DataOptions{
		Type: acpv1.Yaml,
		Key:  "config.yaml",
	}

	result10 := shouldCreateOrUpdateConfigMap(existingConfigMap10, latestConfigMapSettings10, dataOptions10)
	assert.False(t, result10, "Should not update ConfigMap when YAML data is the same")

	// Test case 11: ConfigMap exists with different data (YAML type)
	yamlData11a := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]interface{}{
			"subkey1": "subvalue1",
		},
	}
	yamlBytes11a, _ := yaml.Marshal(yamlData11a)

	yamlData11b := map[string]interface{}{
		"key1": "value1",
		"key2": "updated-value", // Changed value
		"nested": map[string]interface{}{
			"subkey1": "subvalue1",
		},
	}
	yamlBytes11b, _ := yaml.Marshal(yamlData11b)

	existingConfigMap11 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap"},
		Data: map[string]string{
			"config.yaml": string(yamlBytes11a),
		},
	}
	latestConfigMapSettings11 := map[string]string{
		"config.yaml": string(yamlBytes11b),
	}
	dataOptions11 := &acpv1.DataOptions{
		Type: acpv1.Yaml,
		Key:  "config.yaml",
	}

	result11 := shouldCreateOrUpdateConfigMap(existingConfigMap11, latestConfigMapSettings11, dataOptions11)
	assert.True(t, result11, "Should update ConfigMap when YAML data is different")
}

func TestVerifyObject(t *testing.T) {
	EndpointName := "https://fake-endpoint"

	t.Run("Should return error if both endpoint and connectionStringReference are set", func(t *testing.T) {
		configMapName := "test-configmap"
		connectionStringReference := "fakeSecret"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint:                  &EndpointName,
			ConnectionStringReference: &connectionStringReference,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec: both endpoint and connectionStringReference field are set", err.Error())
	})

	t.Run("Should return error if configMapData key is set when type is default", func(t *testing.T) {
		configMapName := "test-configmap"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
				ConfigMapData: &acpv1.DataOptions{
					Type: acpv1.Default,
					Key:  "testKey",
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.target.configMapData.key: key field is not allowed when type is default", err.Error())
	})

	t.Run("Should return error if configMapData key is not set when type is not default", func(t *testing.T) {
		configMapName := "test-configmap"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
				ConfigMapData: &acpv1.DataOptions{
					Type: acpv1.Json,
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.target.configMapData.key: key field is required when type is json, yaml or properties", err.Error())
	})

	t.Run("Should return error if configMapData separator is set when type is default", func(t *testing.T) {
		configMapName := "test-configmap"
		delimiter := "."
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
				ConfigMapData: &acpv1.DataOptions{
					Type:      acpv1.Default,
					Separator: &delimiter,
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.target.configMapData.separator: separator field is not allowed when type is default", err.Error())
	})

	t.Run("Should return error if configMapData separator is set when type is properties", func(t *testing.T) {
		configMapName := "test-configmap"
		delimiter := "."
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
				ConfigMapData: &acpv1.DataOptions{
					Type:      acpv1.Properties,
					Key:       "testKey",
					Separator: &delimiter,
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.target.configMapData.separator: separator field is not allowed when type is properties", err.Error())
	})

	t.Run("Should return error if selector only uses labelFilter", func(t *testing.T) {
		configMapName := "test-configmap"
		testLabelFilter := "testLabelFilter"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Selectors: []acpv1.Selector{
					{
						LabelFilter: &testLabelFilter,
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.configuration.selectors: a selector must specify either 'keyFilter' or 'snapshotName'", err.Error())
	})

	t.Run("Should return error set both 'labelFilter' and 'snapshotName' in one selector", func(t *testing.T) {
		configMapName := "test-configmap"
		testLabelFilter := "testLabelFilter"
		testSnapshotName := "testSnapshotName"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Selectors: []acpv1.Selector{
					{
						LabelFilter:  &testLabelFilter,
						SnapshotName: &testSnapshotName,
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.configuration.selectors: 'labelFilter' is not allowed when 'snapshotName' is set", err.Error())
	})

	t.Run("Should return error if both endpoint and connectionStringReference are not set", func(t *testing.T) {
		configMapName := "test-configmap"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec: one of endpoint and connectionStringReference field must be set", err.Error())
	})

	t.Run("Should return error when configuration.refresh.interval is less than 1 second", func(t *testing.T) {
		configMapName := "test-configmap"
		testKey := "testKey"
		testLabel := "testLabel"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Refresh: &acpv1.DynamicConfigurationRefreshParameters{
					Interval: "500ms",
					Enabled:  true,
					Monitoring: &acpv1.RefreshMonitoring{
						Sentinels: []acpv1.Sentinel{
							{Key: testKey, Label: &testLabel},
						},
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "configuration.refresh.interval: configuration.refresh.interval can not be shorter than 1s", err.Error())
	})

	t.Run("Should return error when there's non escaped value in labelFilter", func(t *testing.T) {
		configMapName := "test-configmap"
		testLabelFilter := ","
		testKeyFilter := "testKeyFilter"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Selectors: []acpv1.Selector{
					{
						LabelFilter: &testLabelFilter,
						KeyFilter:   &testKeyFilter,
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.configuration.selectors: non-escaped reserved wildcard character '*' and multiple labels separator ',' are not supported in label filters", err.Error())
	})

	t.Run("Should return error if feature flag is set when data type is default", func(t *testing.T) {
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

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.target.configMapData: configMap data type must be json or yaml when FeatureFlag is set", err.Error())
	})

	t.Run("Should return error if feature flag selector is not set", func(t *testing.T) {
		configMapName := "test-configmap"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
				ConfigMapData: &acpv1.DataOptions{
					Type: acpv1.Json,
					Key:  "testKey",
				},
			},
			FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.featureFlag.selectors: featureFlag.selectors must be specified when FeatureFlag is set", err.Error())
	})

	t.Run("Should return error when secret.refresh.interval is less than 1 second", func(t *testing.T) {
		configMapName := "test-configmap"
		connectionStringReference := "fakeSecret"
		testKey := "testKey"
		testLabelOne := "testValue"
		testLabelTwo := "testValue1"
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
								Key:   testKey,
								Label: &testLabelOne,
							},
							{
								Key:   testKey,
								Label: &testLabelTwo,
							},
							{
								Key:   testKey,
								Label: &testLabelOne,
							},
						},
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.configuration.refresh.monitoring.keyValues: monitoring duplicated key 'testKey'", err.Error())
	})

	t.Run("Should return no error when all sentinel are unique", func(t *testing.T) {
		configMapName := "test-configmap"
		connectionStringReference := "fakeSecret"
		testKey := "testKey"
		testKey2 := "testKey2"
		testKey3 := "testKey3"
		testKey4 := "testKey4"
		testLabel := "testValue"
		emptyLabel := ""
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
								Key:   testKey,
								Label: nil,
							},
							{
								Key:   testKey2,
								Label: &testLabel,
							},
							{
								Key:   testKey3,
								Label: nil,
							},
							{
								Key:   testKey4,
								Label: &emptyLabel,
							},
						},
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Nil(t, err)
	})

	t.Run("Should return error when incorrectly configure the selector", func(t *testing.T) {
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

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "spec.configuration.selectors: set both 'keyFilter' and 'snapshotName' in one selector causes ambiguity, only one of them should be set", err.Error())

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

		err = verifyObject(configProviderSpec2)
		assert.Equal(t, "spec.configuration.selectors: 'labelFilter' is not allowed when 'snapshotName' is set", err.Error())

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

		err = verifyObject(configProviderSpec3)
		assert.Equal(t, "spec.configuration.selectors: a selector must specify either 'keyFilter' or 'snapshotName'", err.Error())
	})

	t.Run("Should return error when secret.refresh.interval is less than 1 second", func(t *testing.T) {
		configMapName := "test-configmap"
		testKey := "testKey"
		testLabel := "testLabel"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Refresh: &acpv1.DynamicConfigurationRefreshParameters{
					Interval: "500ms",
					Enabled:  true,
					Monitoring: &acpv1.RefreshMonitoring{
						Sentinels: []acpv1.Sentinel{
							{Key: testKey, Label: &testLabel},
						},
					},
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "configuration.refresh.interval: configuration.refresh.interval can not be shorter than 1s", err.Error())
	})

	t.Run("Should return error when secret.refresh.interval is less than 1 minute", func(t *testing.T) {
		configMapName := "test-configmap"
		secretName := "test"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
			},
			Secret: &acpv1.SecretReference{
				Target: acpv1.SecretGenerationParameters{
					SecretName: secretName,
				},
				Refresh: &acpv1.RefreshSettings{
					Interval: "1s",
					Enabled:  true,
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "secret.refresh.interval: secret.refresh.interval can not be shorter than 1m0s", err.Error())
	})

	t.Run("Should return error when featureFlag.refresh.interval is less than 1 second", func(t *testing.T) {
		configMapName := "test-configmap"
		wildcard := "*"
		configProviderSpec := acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: configMapName,
				ConfigMapData: &acpv1.DataOptions{
					Type: acpv1.Json,
					Key:  "testKey",
				},
			},
			FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
				Selectors: []acpv1.Selector{
					{
						KeyFilter: &wildcard,
					},
				},
				Refresh: &acpv1.FeatureFlagRefreshSettings{
					Interval: "500ms",
					Enabled:  true,
				},
			},
		}

		err := verifyObject(configProviderSpec)
		assert.Equal(t, "featureFlag.refresh.interval: featureFlag.refresh.interval can not be shorter than 1s", err.Error())
	})
}

func TestVerifyAuthObject(t *testing.T) {
	// Store original env
	originalWIEnabled := os.Getenv("WORKLOAD_IDENTITY_ENABLED")

	// Cleanup after tests
	defer func() {
		err := os.Setenv("WORKLOAD_IDENTITY_ENABLED", originalWIEnabled)
		if err != nil {
			t.Errorf("Failed to restore environment variable: %v", err)
		}
	}()

	t.Run("Should return no error if auth object is valid", func(t *testing.T) {
		err := os.Setenv("WORKLOAD_IDENTITY_ENABLED", "true")
		if err != nil {
			t.Errorf("Failed to set environment variable: %v", err)
		}

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

		assert.Nil(t, verifyAuthObject(nil))
		assert.Nil(t, verifyAuthObject(authObj))
		assert.Nil(t, verifyAuthObject(authObj2))
		assert.Nil(t, verifyAuthObject(authObj3))
		assert.Nil(t, verifyAuthObject(authObj4))
		assert.Nil(t, verifyAuthObject(authObj5))
		assert.Nil(t, verifyAuthObject(authObj6))
	})

	t.Run("Should return error if auth object is not valid", func(t *testing.T) {
		if err := os.Setenv("WORKLOAD_IDENTITY_ENABLED", "true"); err != nil {
			t.Errorf("Failed to set environment variable: %v", err)
		}

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

		assert.Equal(t, "auth: ManagedIdentityClientId \"not-a-uuid\" in auth field is not a valid uuid", verifyAuthObject(authObj).Error())
		assert.Equal(t, "auth: more than one authentication methods are specified in 'auth' field", verifyAuthObject(authObj2).Error())
		assert.Equal(t, "auth.workloadIdentity: setting only one of 'managedIdentityClientId', 'managedIdentityClientIdReference' or 'serviceAccountName' field is allowed", verifyAuthObject(authObj3).Error())
		assert.Equal(t, "auth.workloadIdentity.managedIdentityClientId: managedIdentityClientId \"not-a-uuid\" in auth.workloadIdentity is not a valid uuid", verifyAuthObject(authObj4).Error())
		assert.Equal(t, "auth.workloadIdentity: setting one of 'managedIdentityClientId', 'managedIdentityClientIdReference' or 'serviceAccountName' field is required", verifyAuthObject(authObj5).Error())
	})
}

func TestVerifyExistingTargetObject(t *testing.T) {
	providerNamespace := "default"

	t.Run("Should return no error if existing configMap is valid", func(t *testing.T) {
		providerName := "providerName"
		configMapName := "configMapName"

		// Case 1: ConfigMap owned by the provider
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: providerNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "AzureAppConfigurationProvider", Name: providerName},
				},
			},
		}

		// Case 2: ConfigMap with different name
		configMap2 := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "anotherConfigMap",
				Namespace: providerNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "AzureAppConfigurationProvider", Name: providerName},
				},
			},
		}

		// Case 3: Empty ConfigMap
		emptyConfigMap := &corev1.ConfigMap{}

		assert.Nil(t, verifyExistingTargetObject(emptyConfigMap, configMapName, providerName))
		assert.Nil(t, verifyExistingTargetObject(configMap, configMapName, providerName))
		assert.Nil(t, verifyExistingTargetObject(configMap2, configMapName, providerName))
	})

	t.Run("Should return error if configMap is not valid", func(t *testing.T) {
		providerName := "providerName"
		configMapName := "configMapName"

		// Case 1: ConfigMap exists but is not owned by the provider
		configMap1 := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: providerNamespace,
			},
		}

		// Case 2: ConfigMap owned by another provider
		configMap2 := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: providerNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "AzureAppConfigurationProvider", Name: "anotherProvider"},
				},
			},
		}

		err1 := verifyExistingTargetObject(configMap1, configMapName, providerName)
		assert.Equal(t, "a ConfigMap with name 'configMapName' already exists in namespace 'default'", err1.Error())

		err2 := verifyExistingTargetObject(configMap2, configMapName, providerName)
		assert.Equal(t, "a ConfigMap with name 'configMapName' already exists in namespace 'default'", err2.Error())
	})
}

func TestHasNonEscapedValueInLabel(t *testing.T) {
	t.Run("Should return false if all character is escaped", func(t *testing.T) {
		assert.False(t, hasNonEscapedValueInLabel(`some\,valid\,label`))
		assert.False(t, hasNonEscapedValueInLabel(`somevalidlabel`))
		assert.False(t, hasNonEscapedValueInLabel(""))
		assert.False(t, hasNonEscapedValueInLabel(`some\*`))
		assert.False(t, hasNonEscapedValueInLabel(`\\some\,\*\valid\,\label\*`))
		assert.False(t, hasNonEscapedValueInLabel(`\,`))
		assert.False(t, hasNonEscapedValueInLabel(`\\`))
		assert.False(t, hasNonEscapedValueInLabel(`\`))
		assert.False(t, hasNonEscapedValueInLabel(`'\`))
		assert.False(t, hasNonEscapedValueInLabel(`\\\,`))
		assert.False(t, hasNonEscapedValueInLabel(`\a\\\,`))
		assert.False(t, hasNonEscapedValueInLabel(`\\\\\\\,`))
	})

	t.Run("Should return true if any character is not escaped", func(t *testing.T) {
		assert.True(t, hasNonEscapedValueInLabel(`some\,invalid,label`))
		assert.True(t, hasNonEscapedValueInLabel(`,`))
		assert.True(t, hasNonEscapedValueInLabel(`some,,value\\`))
		assert.True(t, hasNonEscapedValueInLabel(`some\,,value`))
		assert.True(t, hasNonEscapedValueInLabel(`some\,*value`))
		assert.True(t, hasNonEscapedValueInLabel(`\\,`))
		assert.True(t, hasNonEscapedValueInLabel(`\x\,y\\,z`))
		assert.True(t, hasNonEscapedValueInLabel(`\x\,\y*`))
		assert.True(t, hasNonEscapedValueInLabel(`\x\*\\\*some\\*value`))
		assert.True(t, hasNonEscapedValueInLabel(`\,\\\\,`))
	})
}
