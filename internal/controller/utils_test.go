// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	acpv1 "azappconfig/provider/api/v1"
	"azappconfig/provider/internal/loader"
	"encoding/json"
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
