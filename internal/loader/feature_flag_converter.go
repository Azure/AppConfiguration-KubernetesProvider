// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
)

const (
	featureFlagDescriptionKey         = "description"
	featureFlagConditionsKey          = "conditions"
	featureFlagClientFiltersKey       = "client_filters"
	featureFlagRequirementTypeKey     = "requirement_type"
	featureFlagNameKey                = "name"
	featureFlagParametersKey          = "parameters"
	featureFlagVariantsKey            = "variants"
	featureFlagConfigurationValueKey  = "configuration_value"
	featureFlagStatusOverrideKey      = "status_override"
	featureFlagAllocationKey          = "allocation"
	featureFlagDefaultWhenDisabledKey = "default_when_disabled"
	featureFlagDefaultWhenEnabledKey  = "default_when_enabled"
	featureFlagPercentileKey          = "percentile"
	featureFlagVariantKey             = "variant"
	featureFlagFromKey                = "from"
	featureFlagToKey                  = "to"
	featureFlagGroupKey               = "group"
	featureFlagGroupsKey              = "groups"
	featureFlagUserKey                = "user"
	featureFlagUsersKey               = "users"
	featureFlagSeedKey                = "seed"
)

// convertToMicrosoftSchema converts an enhanced FeatureFlag returned by new feature flag
// endpoint into the Microsoft Feature Management schema object (snake_case) used within the
// `feature_management.feature_flags` array.
func convertToMicrosoftSchema(featureFlag azappconfig.FeatureFlag) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	if featureFlag.Name != nil {
		result[FeatureFlagIdKey] = *featureFlag.Name
	}

	if featureFlag.Enabled != nil {
		result[EnabledKey] = *featureFlag.Enabled
	} else {
		result[EnabledKey] = false
	}

	if featureFlag.Description != nil {
		result[featureFlagDescriptionKey] = *featureFlag.Description
	}

	// conditions: filters -> client_filters, requirementType -> requirement_type
	conditions := make(map[string]interface{})
	clientFilters := make([]interface{}, 0)
	if featureFlag.Conditions != nil {
		for _, filter := range featureFlag.Conditions.Filters {
			clientFilter := make(map[string]interface{})
			if filter.Name != nil {
				clientFilter[featureFlagNameKey] = *filter.Name
			}
			if filter.Parameters != nil {
				parameters := make(map[string]interface{}, len(filter.Parameters))
				for key, value := range filter.Parameters {
					parameters[key] = parseFeatureFlagParameterValue(value)
				}
				clientFilter[featureFlagParametersKey] = parameters
			}
			clientFilters = append(clientFilters, clientFilter)
		}
	}
	conditions[featureFlagClientFiltersKey] = clientFilters
	if featureFlag.Conditions != nil && featureFlag.Conditions.RequirementType != nil {
		conditions[featureFlagRequirementTypeKey] = string(*featureFlag.Conditions.RequirementType)
	}
	result[featureFlagConditionsKey] = conditions

	// variants: value -> configuration_value, statusOverride -> status_override
	if featureFlag.Variants != nil {
		variants := make([]interface{}, 0, len(featureFlag.Variants))
		for _, variant := range featureFlag.Variants {
			variantMap := make(map[string]interface{})
			if variant.Name != nil {
				variantMap[featureFlagNameKey] = *variant.Name
			}
			if variant.Value != nil {
				if isJsonContentType(variant.ContentType) {
					parsedValue, err := parseFeatureFlagVariantValue(variant.Value)
					if err != nil {
						variantName := ""
						if variant.Name != nil {
							variantName = *variant.Name
						}
						return nil, fmt.Errorf("failed to parse variant %q value: %w", variantName, err)
					}
					variantMap[featureFlagConfigurationValueKey] = parsedValue
				} else {
					variantMap[featureFlagConfigurationValueKey] = *variant.Value
				}
			}
			if variant.StatusOverride != nil {
				variantMap[featureFlagStatusOverrideKey] = string(*variant.StatusOverride)
			}
			variants = append(variants, variantMap)
		}
		result[featureFlagVariantsKey] = variants
	}

	// allocation: camelCase -> snake_case
	if featureFlag.Allocation != nil {
		allocation := make(map[string]interface{})
		source := featureFlag.Allocation
		if source.DefaultWhenDisabled != nil {
			allocation[featureFlagDefaultWhenDisabledKey] = *source.DefaultWhenDisabled
		}
		if source.DefaultWhenEnabled != nil {
			allocation[featureFlagDefaultWhenEnabledKey] = *source.DefaultWhenEnabled
		}
		if source.Percentile != nil {
			percentiles := make([]interface{}, 0, len(source.Percentile))
			for _, percentile := range source.Percentile {
				percentileMap := make(map[string]interface{})
				if percentile.Variant != nil {
					percentileMap[featureFlagVariantKey] = *percentile.Variant
				}
				if percentile.From != nil {
					percentileMap[featureFlagFromKey] = *percentile.From
				}
				if percentile.To != nil {
					percentileMap[featureFlagToKey] = *percentile.To
				}
				percentiles = append(percentiles, percentileMap)
			}
			allocation[featureFlagPercentileKey] = percentiles
		}
		if source.Group != nil {
			groups := make([]interface{}, 0, len(source.Group))
			for _, group := range source.Group {
				groupMap := make(map[string]interface{})
				if group.Variant != nil {
					groupMap[featureFlagVariantKey] = *group.Variant
				}
				if group.Groups != nil {
					groupMap[featureFlagGroupsKey] = toInterfaceSlice(group.Groups)
				}
				groups = append(groups, groupMap)
			}
			allocation[featureFlagGroupKey] = groups
		}
		if source.User != nil {
			users := make([]interface{}, 0, len(source.User))
			for _, user := range source.User {
				userMap := make(map[string]interface{})
				if user.Variant != nil {
					userMap[featureFlagVariantKey] = *user.Variant
				}
				if user.Users != nil {
					userMap[featureFlagUsersKey] = toInterfaceSlice(user.Users)
				}
				users = append(users, userMap)
			}
			allocation[featureFlagUserKey] = users
		}
		if source.Seed != nil {
			allocation[featureFlagSeedKey] = *source.Seed
		}
		result[featureFlagAllocationKey] = allocation
	}

	// telemetry: metadata is (re)populated later by populateTelemetryMetadata with ETag/FeatureFlagReference
	if featureFlag.Telemetry != nil {
		telemetry := make(map[string]interface{})
		if featureFlag.Telemetry.Enabled != nil {
			telemetry[EnabledKey] = *featureFlag.Telemetry.Enabled
		} else {
			telemetry[EnabledKey] = false
		}
		if featureFlag.Telemetry.Metadata != nil {
			metadata := make(map[string]interface{}, len(featureFlag.Telemetry.Metadata))
			for key, value := range featureFlag.Telemetry.Metadata {
				if value != nil {
					metadata[key] = *value
				}
			}
			telemetry[MetadataKey] = metadata
		}
		result[TelemetryKey] = telemetry
	}

	return result, nil
}

// parseFeatureFlagParameterValue parses object and array parameters as JSON. 
// Other values and malformed JSON are preserved as literal strings.
func parseFeatureFlagParameterValue(raw *string) interface{} {
	if raw == nil {
		return nil
	}

	trimmed := strings.TrimLeftFunc(*raw, unicode.IsSpace)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var parsed interface{}
		if err := json.Unmarshal([]byte(*raw), &parsed); err == nil {
			return parsed
		}
	}

	return *raw
}

// parseFeatureFlagVariantValue parses the JSON-encoded value returned by the enhanced feature flag endpoint.
func parseFeatureFlagVariantValue(raw *string) (interface{}, error) {
	if raw == nil {
		return nil, nil
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(*raw), &parsed); err != nil {
		return nil, err
	}

	return parsed, nil
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
