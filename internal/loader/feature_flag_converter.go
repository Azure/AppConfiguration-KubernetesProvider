// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"encoding/json"

	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
)

// convertToMicrosoftSchema converts an enhanced FeatureFlag returned by new feature flag
// endpoint into the Microsoft Feature Management schema object (snake_case) used within the
// `feature_management.feature_flags` array.
func convertToMicrosoftSchema(featureFlag azappconfig.FeatureFlag) map[string]interface{} {
	result := make(map[string]interface{})

	if featureFlag.Name != nil {
		result["id"] = *featureFlag.Name
	}

	if featureFlag.Enabled != nil {
		result["enabled"] = *featureFlag.Enabled
	} else {
		result["enabled"] = false
	}

	if featureFlag.Description != nil {
		result["description"] = *featureFlag.Description
	}

	// conditions: filters -> client_filters, requirementType -> requirement_type
	conditions := make(map[string]interface{})
	clientFilters := make([]interface{}, 0)
	if featureFlag.Conditions != nil {
		for _, filter := range featureFlag.Conditions.Filters {
			clientFilter := make(map[string]interface{})
			if filter.Name != nil {
				clientFilter["name"] = *filter.Name
			}
			if filter.Parameters != nil {
				parameters := make(map[string]interface{}, len(filter.Parameters))
				for key, value := range filter.Parameters {
					parameters[key] = parseFeatureFlagValue(value)
				}
				clientFilter["parameters"] = parameters
			}
			clientFilters = append(clientFilters, clientFilter)
		}
	}
	conditions["client_filters"] = clientFilters
	if featureFlag.Conditions != nil && featureFlag.Conditions.RequirementType != nil {
		conditions["requirement_type"] = string(*featureFlag.Conditions.RequirementType)
	}
	result["conditions"] = conditions

	// variants: value -> configuration_value, statusOverride -> status_override
	if featureFlag.Variants != nil {
		variants := make([]interface{}, 0, len(featureFlag.Variants))
		for _, variant := range featureFlag.Variants {
			variantMap := make(map[string]interface{})
			if variant.Name != nil {
				variantMap["name"] = *variant.Name
			}
			if variant.Value != nil {
				variantMap["configuration_value"] = parseFeatureFlagValue(variant.Value)
			}
			if variant.StatusOverride != nil {
				variantMap["status_override"] = string(*variant.StatusOverride)
			}
			variants = append(variants, variantMap)
		}
		result["variants"] = variants
	}

	// allocation: camelCase -> snake_case
	if featureFlag.Allocation != nil {
		allocation := make(map[string]interface{})
		source := featureFlag.Allocation
		if source.DefaultWhenDisabled != nil {
			allocation["default_when_disabled"] = *source.DefaultWhenDisabled
		}
		if source.DefaultWhenEnabled != nil {
			allocation["default_when_enabled"] = *source.DefaultWhenEnabled
		}
		if source.Percentile != nil {
			percentiles := make([]interface{}, 0, len(source.Percentile))
			for _, percentile := range source.Percentile {
				percentileMap := make(map[string]interface{})
				if percentile.Variant != nil {
					percentileMap["variant"] = *percentile.Variant
				}
				if percentile.From != nil {
					percentileMap["from"] = *percentile.From
				}
				if percentile.To != nil {
					percentileMap["to"] = *percentile.To
				}
				percentiles = append(percentiles, percentileMap)
			}
			allocation["percentile"] = percentiles
		}
		if source.Group != nil {
			groups := make([]interface{}, 0, len(source.Group))
			for _, group := range source.Group {
				groupMap := make(map[string]interface{})
				if group.Variant != nil {
					groupMap["variant"] = *group.Variant
				}
				if group.Groups != nil {
					groupMap["groups"] = toInterfaceSlice(group.Groups)
				}
				groups = append(groups, groupMap)
			}
			allocation["group"] = groups
		}
		if source.User != nil {
			users := make([]interface{}, 0, len(source.User))
			for _, user := range source.User {
				userMap := make(map[string]interface{})
				if user.Variant != nil {
					userMap["variant"] = *user.Variant
				}
				if user.Users != nil {
					userMap["users"] = toInterfaceSlice(user.Users)
				}
				users = append(users, userMap)
			}
			allocation["user"] = users
		}
		if source.Seed != nil {
			allocation["seed"] = *source.Seed
		}
		result["allocation"] = allocation
	}

	// telemetry: metadata is (re)populated later by populateTelemetryMetadata with ETag/FeatureFlagReference
	if featureFlag.Telemetry != nil {
		telemetry := make(map[string]interface{})
		if featureFlag.Telemetry.Enabled != nil {
			telemetry["enabled"] = *featureFlag.Telemetry.Enabled
		} else {
			telemetry["enabled"] = false
		}
		if featureFlag.Telemetry.Metadata != nil {
			metadata := make(map[string]interface{}, len(featureFlag.Telemetry.Metadata))
			for key, value := range featureFlag.Telemetry.Metadata {
				if value != nil {
					metadata[key] = *value
				}
			}
			telemetry["metadata"] = metadata
		}
		result["telemetry"] = telemetry
	}

	return result
}

// Attempting to parse the string as JSON recovers booleans, numbers, and nested objects; non-JSON strings are returned as-is.
func parseFeatureFlagValue(raw *string) interface{} {
	if raw == nil {
		return nil
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(*raw), &parsed); err == nil {
		return parsed
	}

	return *raw
}

// toInterfaceSlice converts a slice of strings into a slice of interface{} for inclusion in the
// generic map that is marshaled into the feature management schema.
func toInterfaceSlice(values []string) []interface{} {
	result := make([]interface{}, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}
