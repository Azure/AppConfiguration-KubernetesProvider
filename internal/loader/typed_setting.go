// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

func createTypedSettings(rawSettings *RawSettings, dataOptions *acpv1.ConfigMapDataOptions) (map[string]string, error) {
	result := make(map[string]string)
	if len(rawSettings.KeyValueSettings) == 0 && rawSettings.FeatureFlagSettings == nil {
		return result, nil
	}

	if dataOptions == nil || dataOptions.Type == acpv1.Default || dataOptions.Type == acpv1.Properties {
		tmpSettings := make(map[string]string)
		for k, v := range rawSettings.KeyValueSettings {
			if v != nil {
				tmpSettings[k] = *v
			} else {
				klog.Warningf("value of the setting '%s' is null, just ignore it", k)
			}
		}

		if dataOptions == nil || dataOptions.Type == acpv1.Default {
			return tmpSettings, nil
		}

		result[dataOptions.Key] = marshalProperties(tmpSettings)
		return result, nil
	}
	// dataOptions.Type = json or yaml
	root := &Tree{}
	parsedSettings := make(map[string]interface{})
	for k, v := range rawSettings.KeyValueSettings {
		var value interface{} = nil
		if v != nil {
			value = *v
			if rawSettings.IsJsonContentTypeMap[k] {
				var out interface{}
				err := json.Unmarshal([]byte(*v), &out)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal json value for key '%s': %s", k, err.Error())
				}
				value = out
			}
		}

		if dataOptions.Separator != nil {
			root.insert(strings.Split(k, string(*dataOptions.Separator)), value)
		} else {
			parsedSettings[k] = value
		}
	}

	if dataOptions.Separator != nil {
		parsedSettings = root.build()
	}

	if rawSettings.FeatureFlagSettings != nil {
		// FeatureManagementSection = {"feature_management": { "feature_flags" : [{...}, {...}]}
		parsedSettings[FeatureManagementSectionName] = rawSettings.FeatureFlagSettings
	}

	if typedStr, err := marshalJsonYaml(parsedSettings, dataOptions); err != nil {
		return nil, err
	} else {
		result[dataOptions.Key] = typedStr
	}

	return result, nil
}

func marshalProperties(settings map[string]string) string {
	stringBuilder := strings.Builder{}
	separator := "\n"
	if settings != nil {
		i := 0
		for k, v := range settings {
			stringBuilder.WriteString(fmt.Sprintf("%s=%s", k, v))
			if i < len(settings)-1 {
				stringBuilder.WriteString(separator)
			}
			i++
		}
	}

	return stringBuilder.String()
}

func isJsonContentType(contentType *string) bool {
	if contentType == nil {
		return false
	}
	contentTypeStr := strings.ToLower(strings.Trim(*contentType, " "))
	matched, _ := regexp.MatchString("^application\\/(?:[^\\/]+\\+)?json(;.*)?$", contentTypeStr)
	return matched
}

// Used for scenarios related to feature flag refresh.
// Currently, only supports feature flag settings in JSON/YAML formats.
func unmarshalConfigMap(existingConfigMapSetting *map[string]string, dataOptions *acpv1.ConfigMapDataOptions) (map[string]interface{}, map[string]interface{}, error) {
	var existingConfigMapData string
	var ok bool
	if existingConfigMapData, ok = (*existingConfigMapSetting)[dataOptions.Key]; !ok {
		return nil, nil, fmt.Errorf("failed to get data for key '%s' from configMap", dataOptions.Key)
	}

	keyValueSettings := make(map[string]interface{})
	var featureFlagSection map[string]interface{}
	switch dataOptions.Type {
	case acpv1.Yaml:
		err := yaml.Unmarshal([]byte(existingConfigMapData), &keyValueSettings)
		if err != nil {
			return nil, nil, err
		}
	case acpv1.Json:
		err := json.Unmarshal([]byte(existingConfigMapData), &keyValueSettings)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("unsupported data type '%s', only supports json/yaml for now", dataOptions.Type)
	}

	if featureFlagSection, ok = keyValueSettings[FeatureManagementSectionName].(map[string]interface{}); !ok {
		return nil, nil, fmt.Errorf("failed to get feature flags from configMap data '%s'", existingConfigMapData)
	}
	delete(keyValueSettings, FeatureManagementSectionName)
	return featureFlagSection, keyValueSettings, nil
}

func marshalJsonYaml(settings map[string]interface{}, dataOptions *acpv1.ConfigMapDataOptions) (string, error) {
	switch dataOptions.Type {
	case acpv1.Yaml:
		yamlStr, err := yaml.Marshal(settings)
		if err != nil {
			return "", fmt.Errorf("failed to marshal key-values to yaml: %s", err.Error())
		}
		return string(yamlStr), nil
	case acpv1.Json:
		jsonStr, err := json.Marshal(settings)
		if err != nil {
			return "", fmt.Errorf("failed to marshal key-values to json: %s", err.Error())
		}
		return string(jsonStr), nil
	}

	return "", nil
}

func isAIConfigurationContentType(contentType *string) bool {
	if !isJsonContentType(contentType) {
		return false
	}

	return hasProfile(*contentType, AIMimeProfileKey)
}

func isAIChatCompletionContentType(contentType *string) bool {
	if !isJsonContentType(contentType) {
		return false
	}

	return hasProfile(*contentType, AIChatCompletionMimeProfileKey)
}

// hasProfile checks if a content type contains a specific profile parameter
func hasProfile(contentType, profileValue string) bool {
	// Split by semicolons to get content type parts
	parts := strings.Split(contentType, ";")

	// Check each part after the content type for profile parameter
	for i := 1; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])

		// Look for profile="value" pattern
		if strings.HasPrefix(part, "profile=") {
			// Extract the profile value (handling quoted values)
			profile := part[len("profile="):]
			profile = strings.Trim(profile, "\"'")

			if profile == profileValue {
				return true
			}
		}
	}

	return false
}
