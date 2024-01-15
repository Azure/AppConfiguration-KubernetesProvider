// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

func createTypedSettings(unformattedSettings *UnformattedSettings, dataOptions *acpv1.ConfigMapDataOptions) (map[string]string, error) {
	if unformattedSettings.KeyValueSettings == nil {
		return nil, errors.New("settings cannot be nil")
	}

	if unformattedSettings.IsJsonContentTypeMap == nil {
		return nil, errors.New("isJsonContentTypeMap cannot be nil")
	}

	result := make(map[string]string)
	if len(unformattedSettings.KeyValueSettings) == 0 {
		return result, nil
	}

	if dataOptions == nil || dataOptions.Type == acpv1.Default || dataOptions.Type == acpv1.Properties {
		tmpSettings := make(map[string]string)
		for k, v := range unformattedSettings.KeyValueSettings {
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
	for k, v := range unformattedSettings.KeyValueSettings {
		var value interface{} = nil
		if v != nil {
			value = *v
			if unformattedSettings.IsJsonContentTypeMap[k] {
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

	if unformattedSettings.FeatureFlagSettings != nil {
		if typedFeatureFlagSettings, err := unmarshalFeatureFlagSettings(unformattedSettings.FeatureFlagSettings); err != nil {
			return nil, err
		} else {
			parsedSettings[FeatureManagementSectionName] = typedFeatureFlagSettings
		}
	}

	if typedStr, err := createJsonYamlTypeData(parsedSettings, dataOptions); err != nil {
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

func UnmarshalConfigMap(existingConfigMapSetting *map[string]string, dataOptions *acpv1.ConfigMapDataOptions) (interface{}, map[string]interface{}, error) {
	allSettings := make(map[string]interface{})
	existingConfigMapData := (*existingConfigMapSetting)[dataOptions.Key]
	var typedFeatureFlags interface{}
	var err error

	switch dataOptions.Type {
	case acpv1.Yaml:
		err = yaml.Unmarshal([]byte(existingConfigMapData), &allSettings)
		if err != nil {
			return nil, nil, err
		}
	case acpv1.Json:
		err = json.Unmarshal([]byte(existingConfigMapData), &allSettings)
		if err != nil {
			return nil, nil, err
		}
	}

	if _, ok := allSettings[FeatureManagementSectionName]; ok {
		typedFeatureFlags = allSettings[FeatureManagementSectionName]
		delete(allSettings, FeatureManagementSectionName)
	}

	return typedFeatureFlags, allSettings, nil
}

func createJsonYamlTypeData(settings map[string]interface{}, dataOptions *acpv1.ConfigMapDataOptions) (string, error) {
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

func unmarshalFeatureFlagSettings(featureFlagSettings interface{}) (interface{}, error) {
	if featureFlagSettings == nil {
		return nil, nil
	}

	switch featureFlagSettings.(type) {
	// feature flag settings from existing configMap
	case map[string]interface{}:
		return featureFlagSettings, nil
	// feature flag settings from appconfig store
	case []*string:
		featureFlagSlice := featureFlagSettings.([]*string)
		var typedSlice []interface{}
		for _, v := range featureFlagSlice {
			if v != nil {
				var out interface{}
				err := json.Unmarshal([]byte(*v), &out)
				if err != nil {
					return nil, fmt.Errorf("Failed to unmarshal feature flag settings: %s", err.Error())
				}
				typedSlice = append(typedSlice, out)
			}
		}
		return map[string]interface{}{
			FeatureFlagSectionName: typedSlice,
		}, nil
	default:
		return nil, fmt.Errorf("Unsupported feature flag settings type: %T", featureFlagSettings)
	}
}
