// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
)

// fakeAppConfigurationClient is a test double for AppConfigurationClient that serves the provided
// pages from in-memory slices. Only the list operations exercised by the feature flag loading path
// are backed by data; the remaining methods return empty results.
type fakeAppConfigurationClient struct {
	keyValuePages          [][]azappconfig.Setting
	featureFlagPages       [][]azappconfig.FeatureFlag
	featureFlagListOptions *azappconfig.ListFeatureFlagsOptions
}

func (c *fakeAppConfigurationClient) NewListSettingsPager(_ azappconfig.SettingSelector, _ *azappconfig.ListSettingsOptions) *runtime.Pager[azappconfig.ListSettingsPageResponse] {
	pages := c.keyValuePages
	if len(pages) == 0 {
		pages = [][]azappconfig.Setting{{}}
	}
	index := 0
	return runtime.NewPager(runtime.PagingHandler[azappconfig.ListSettingsPageResponse]{
		More: func(azappconfig.ListSettingsPageResponse) bool { return index < len(pages) },
		Fetcher: func(context.Context, *azappconfig.ListSettingsPageResponse) (azappconfig.ListSettingsPageResponse, error) {
			page := pages[index]
			index++
			etag := azcore.ETag(fmt.Sprintf("kv-page-%d", index))
			return azappconfig.ListSettingsPageResponse{Settings: page, ETag: &etag}, nil
		},
	})
}

func (c *fakeAppConfigurationClient) GetSetting(context.Context, string, *azappconfig.GetSettingOptions) (azappconfig.GetSettingResponse, error) {
	return azappconfig.GetSettingResponse{}, nil
}

func (c *fakeAppConfigurationClient) GetSnapshot(context.Context, string, *azappconfig.GetSnapshotOptions) (azappconfig.GetSnapshotResponse, error) {
	return azappconfig.GetSnapshotResponse{}, nil
}

func (c *fakeAppConfigurationClient) NewListSettingsForSnapshotPager(_ string, _ *azappconfig.ListSettingsForSnapshotOptions) *runtime.Pager[azappconfig.ListSettingsForSnapshotResponse] {
	return runtime.NewPager(runtime.PagingHandler[azappconfig.ListSettingsForSnapshotResponse]{
		More: func(azappconfig.ListSettingsForSnapshotResponse) bool { return false },
		Fetcher: func(context.Context, *azappconfig.ListSettingsForSnapshotResponse) (azappconfig.ListSettingsForSnapshotResponse, error) {
			return azappconfig.ListSettingsForSnapshotResponse{}, nil
		},
	})
}

func (c *fakeAppConfigurationClient) NewListFeatureFlagsPager(_ azappconfig.FeatureFlagSelector, options *azappconfig.ListFeatureFlagsOptions) *runtime.Pager[azappconfig.ListFeatureFlagsPageResponse] {
	c.featureFlagListOptions = options
	pages := c.featureFlagPages
	if len(pages) == 0 {
		pages = [][]azappconfig.FeatureFlag{{}}
	}
	index := 0
	return runtime.NewPager(runtime.PagingHandler[azappconfig.ListFeatureFlagsPageResponse]{
		More: func(azappconfig.ListFeatureFlagsPageResponse) bool { return index < len(pages) },
		Fetcher: func(context.Context, *azappconfig.ListFeatureFlagsPageResponse) (azappconfig.ListFeatureFlagsPageResponse, error) {
			page := pages[index]
			index++
			etag := azcore.ETag(fmt.Sprintf("ff-page-%d", index))
			if options != nil && index <= len(options.MatchConditions) {
				condition := options.MatchConditions[index-1]
				if condition.IfNoneMatch != nil && *condition.IfNoneMatch == etag {
					return azappconfig.ListFeatureFlagsPageResponse{}, nil
				}
			}
			return azappconfig.ListFeatureFlagsPageResponse{FeatureFlags: page, ETag: &etag}, nil
		},
	})
}

func newTypedFeatureFlag(name string, enabled bool) azappconfig.FeatureFlag {
	etag := azcore.ETag("etag-" + name)
	return azappconfig.FeatureFlag{
		Name:       &name,
		Enabled:    &enabled,
		ETag:       &etag,
		Conditions: &azappconfig.FeatureFlagConditions{Filters: []azappconfig.FeatureFlagFilter{}},
	}
}

func TestConvertFeatureFlagToMap(t *testing.T) {
	name := "Variant"
	enabled := true
	filterName := "Microsoft.TimeWindow"
	startParam := "Mon, 01 Jan 2024 00:00:00 GMT"
	requirementType := azappconfig.RequirementTypeAll
	offName, offValue := "Off", "false"
	onName, onValue := "On", "true"
	statusOverride := azappconfig.StatusOverrideDisabled
	defaultVariant := "Off"
	percentileVariant := "On"
	from, to := 0.0, 50.0
	seed := "seed-value"
	telemetryEnabled := true

	featureFlag := azappconfig.FeatureFlag{
		Name:    &name,
		Enabled: &enabled,
		Conditions: &azappconfig.FeatureFlagConditions{
			RequirementType: &requirementType,
			Filters: []azappconfig.FeatureFlagFilter{
				{Name: &filterName, Parameters: map[string]*string{"Start": &startParam}},
			},
		},
		Variants: []azappconfig.FeatureFlagVariantDefinition{
			{Name: &offName, Value: &offValue, StatusOverride: &statusOverride},
			{Name: &onName, Value: &onValue},
		},
		Allocation: &azappconfig.FeatureFlagAllocation{
			DefaultWhenEnabled:  &defaultVariant,
			DefaultWhenDisabled: &defaultVariant,
			Percentile:          []azappconfig.PercentileAllocation{{Variant: &percentileVariant, From: &from, To: &to}},
			Seed:                &seed,
		},
		Telemetry: &azappconfig.FeatureFlagTelemetryConfiguration{Enabled: &telemetryEnabled},
	}

	actual, err := json.Marshal(convertToMicrosoftSchema(featureFlag))
	if err != nil {
		t.Fatalf("failed to marshal converted feature flag: %s", err)
	}

	expected := `{"allocation":{"default_when_disabled":"Off","default_when_enabled":"Off","percentile":[{"from":0,"to":50,"variant":"On"}],"seed":"seed-value"},"conditions":{"client_filters":[{"name":"Microsoft.TimeWindow","parameters":{"Start":"Mon, 01 Jan 2024 00:00:00 GMT"}}],"requirement_type":"All"},"enabled":true,"id":"Variant","telemetry":{"enabled":true},"variants":[{"configuration_value":false,"name":"Off","status_override":"Disabled"},{"configuration_value":true,"name":"On"}]}`
	if string(actual) != expected {
		t.Errorf("unexpected converted feature flag.\n got: %s\nwant: %s", actual, expected)
	}
}

func TestEnhancedFeatureFlagSettingsClientLoadsEnhancedFlags(t *testing.T) {
	endpointNameFilter := "*"
	nullLabel := "\x00"

	client := &fakeAppConfigurationClient{
		featureFlagPages: [][]azappconfig.FeatureFlag{{
			newTypedFeatureFlag("Shared", false),
			newTypedFeatureFlag("EnhancedOnly", true),
		}},
	}

	settingsClient := &EnhancedFeatureFlagSettingsClient{
		enhancedFeatureFlagSelectors: []acpv1.Selector{{KeyFilter: &endpointNameFilter, LabelFilter: &nullLabel}},
	}

	response, err := settingsClient.GetSettings(context.Background(), client)
	if err != nil {
		t.Fatalf("GetSettings returned error: %s", err)
	}

	// The enhanced client only loads flags from the dedicated feature flag endpoint; merging with
	// classic feature flags happens later in ProcessFeatureFlags.
	if len(response.EnhancedFeatureFlags) != 2 {
		t.Fatalf("expected 2 enhanced feature flags, got %d", len(response.EnhancedFeatureFlags))
	}
	if response.EnhancedFeatureFlags[0].Name == nil || *response.EnhancedFeatureFlags[0].Name != "Shared" {
		t.Errorf("expected first enhanced flag to be 'Shared', got %v", response.EnhancedFeatureFlags[0].Name)
	}
	if len(response.Settings) != 0 {
		t.Errorf("expected no classic settings from the enhanced client, got %d", len(response.Settings))
	}
}

func TestEnhancedFeatureFlagEtagsClientUsesConditionalRequests(t *testing.T) {
	nameFilter := "*"
	nullLabel := "\x00"
	comparable := acpv1.MakeComparable(acpv1.Selector{KeyFilter: &nameFilter, LabelFilter: &nullLabel})

	client := &fakeAppConfigurationClient{
		featureFlagPages: [][]azappconfig.FeatureFlag{{newTypedFeatureFlag("Beta", true)}},
	}

	// The fake client assigns the first page the ETag "ff-page-1".
	unchangedETag := azcore.ETag("ff-page-1")
	unchangedClient := &EnhancedFeatureFlagEtagsClient{
		etags: map[acpv1.ComparableSelector][]*azcore.ETag{comparable: {&unchangedETag}},
	}
	unchangedResponse, err := unchangedClient.GetSettings(context.Background(), client)
	if err != nil {
		t.Fatalf("GetSettings returned error: %s", err)
	}
	if unchangedResponse.Etags != nil {
		t.Errorf("expected no change to be detected when page ETags match")
	}
	if client.featureFlagListOptions == nil || len(client.featureFlagListOptions.MatchConditions) != 1 {
		t.Fatalf("expected one match condition, got %#v", client.featureFlagListOptions)
	}
	if client.featureFlagListOptions.MatchConditions[0].IfNoneMatch == nil ||
		*client.featureFlagListOptions.MatchConditions[0].IfNoneMatch != unchangedETag {
		t.Errorf("expected If-None-Match %q", unchangedETag)
	}

	staleETag := azcore.ETag("stale-etag")
	changedClient := &EnhancedFeatureFlagEtagsClient{
		etags: map[acpv1.ComparableSelector][]*azcore.ETag{comparable: {&staleETag}},
	}
	changedResponse, err := changedClient.GetSettings(context.Background(), client)
	if err != nil {
		t.Fatalf("GetSettings returned error: %s", err)
	}
	if changedResponse.Etags == nil {
		t.Errorf("expected a change to be detected when page ETags differ")
	}

	missingPageClient := &EnhancedFeatureFlagEtagsClient{
		etags: map[acpv1.ComparableSelector][]*azcore.ETag{
			comparable: {&unchangedETag, &staleETag},
		},
	}
	missingPageResponse, err := missingPageClient.GetSettings(context.Background(), client)
	if err != nil {
		t.Fatalf("GetSettings returned error: %s", err)
	}
	if missingPageResponse.Etags == nil {
		t.Errorf("expected a change to be detected when the page count differs")
	}
}
