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
					return nil, fmt.Errorf("Failed to unmarshal json value for key '%s': %s", k, err.Error())
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
		// FeatureManagementSection = {"FeatureManagement": { "featureFlags" : [{...}, {...}]}
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
	if settings != nil && len(settings) > 0 {
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
	keyValueSettings := make(map[string]interface{})
	var typedFeatureFlags map[string]interface{}

	if existingConfigMapData, ok := (*existingConfigMapSetting)[dataOptions.Key]; !ok {
		return nil, nil, fmt.Errorf("Failed to get data for key '%s' from configMap", dataOptions.Key)
	} else {
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
			return nil, nil, fmt.Errorf("Unsupported data type '%s', only supports json/yaml for now", dataOptions.Type)
		}

		if featureFlagSection, ok := keyValueSettings[FeatureManagementSectionName].(map[string]interface{}); !ok {
			return nil, nil, fmt.Errorf("Failed to get feature flags from configMap data '%s'", existingConfigMapData)
		} else {
			typedFeatureFlags = featureFlagSection
			delete(keyValueSettings, FeatureManagementSectionName)
		}

		return typedFeatureFlags, keyValueSettings, nil
	}
}

func marshalJsonYaml(settings map[string]interface{}, dataOptions *acpv1.ConfigMapDataOptions) (string, error) {
	switch dataOptions.Type {
	case acpv1.Yaml:
		yamlStr, err := yaml.Marshal(settings)
		if err != nil {
			return "", fmt.Errorf("Failed to marshal key-values to yaml: %s", err.Error())
		}
		return string(yamlStr), nil
	case acpv1.Json:
		jsonStr, err := json.Marshal(settings)
		if err != nil {
			return "", fmt.Errorf("Failed to marshal key-values to json: %s", err.Error())
		}
		return string(jsonStr), nil
	}

	return "", nil
}
