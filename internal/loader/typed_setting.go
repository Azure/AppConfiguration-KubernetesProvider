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

func createTypedSettings(settings map[string]*string, isJsonContentTypeMap map[string]bool, dataOptions *acpv1.ConfigMapDataOptions) (map[string]string, error) {
	if settings == nil {
		return nil, errors.New("settings cannot be nil")
	}

	if isJsonContentTypeMap == nil {
		return nil, errors.New("isJsonContentTypeMap cannot be nil")
	}

	result := make(map[string]string)
	if len(settings) == 0 {
		return result, nil
	}

	if dataOptions == nil || dataOptions.Type == acpv1.Default || dataOptions.Type == acpv1.Properties {
		tmpSettings := make(map[string]string)
		for k, v := range settings {
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
	for k, v := range settings {
		var value interface{} = nil
		if v != nil {
			value = *v
			if isJsonContentTypeMap[k] {
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

	if dataOptions.Type == acpv1.Json {
		jsonStr, err := json.Marshal(parsedSettings)
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal key-values to json: %s", err.Error())
		}
		result[dataOptions.Key] = string(jsonStr)
	}

	if dataOptions.Type == acpv1.Yaml {
		yamlStr, err := yaml.Marshal(parsedSettings)
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal key-values to yaml: %s", err.Error())
		}
		result[dataOptions.Key] = string(yamlStr)
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
