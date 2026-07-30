// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
)

// AppConfigurationClient abstracts the Azure App Configuration operations used by the provider.
type AppConfigurationClient interface {
	// Key-value configuration operations.
	NewListSettingsPager(selector azappconfig.SettingSelector, options *azappconfig.ListSettingsOptions) *runtime.Pager[azappconfig.ListSettingsPageResponse]
	GetSetting(ctx context.Context, key string, options *azappconfig.GetSettingOptions) (azappconfig.GetSettingResponse, error)
	GetSnapshot(ctx context.Context, snapshotName string, options *azappconfig.GetSnapshotOptions) (azappconfig.GetSnapshotResponse, error)
	NewListSettingsForSnapshotPager(snapshotName string, options *azappconfig.ListSettingsForSnapshotOptions) *runtime.Pager[azappconfig.ListSettingsForSnapshotResponse]

	// Feature flag operations served by the dedicated feature flag endpoint.
	NewListFeatureFlagsPager(selector azappconfig.FeatureFlagSelector, options *azappconfig.ListFeatureFlagsOptions) *runtime.Pager[azappconfig.ListFeatureFlagsPageResponse]
}

type appConfigurationClient struct {
	configurationClient *azappconfig.Client
	featureFlagClient   *azappconfig.FeatureFlagClient
}

func NewAppConfigurationClient(endpoint string, credential azcore.TokenCredential, options *azappconfig.ClientOptions) (AppConfigurationClient, error) {
	configurationClient, err := azappconfig.NewClient(endpoint, credential, options)
	if err != nil {
		return nil, err
	}

	featureFlagClient, err := azappconfig.NewFeatureFlagClient(endpoint, credential, featureFlagClientOptions(options))
	if err != nil {
		return nil, err
	}

	return &appConfigurationClient{
		configurationClient: configurationClient,
		featureFlagClient:   featureFlagClient,
	}, nil
}

func NewAppConfigurationClientFromConnectionString(connectionString string, options *azappconfig.ClientOptions) (AppConfigurationClient, error) {
	configurationClient, err := azappconfig.NewClientFromConnectionString(connectionString, options)
	if err != nil {
		return nil, err
	}

	featureFlagClient, err := azappconfig.NewFeatureFlagClientFromConnectionString(connectionString, featureFlagClientOptions(options))
	if err != nil {
		return nil, err
	}

	return &appConfigurationClient{
		configurationClient: configurationClient,
		featureFlagClient:   featureFlagClient,
	}, nil
}

// featureFlagClientOptions mirrors the configuration client options onto feature flag client options
func featureFlagClientOptions(options *azappconfig.ClientOptions) *azappconfig.FeatureFlagClientOptions {
	if options == nil {
		return nil
	}

	return &azappconfig.FeatureFlagClientOptions{ClientOptions: options.ClientOptions}
}

func (c *appConfigurationClient) NewListSettingsPager(selector azappconfig.SettingSelector, options *azappconfig.ListSettingsOptions) *runtime.Pager[azappconfig.ListSettingsPageResponse] {
	return c.configurationClient.NewListSettingsPager(selector, options)
}

func (c *appConfigurationClient) GetSetting(ctx context.Context, key string, options *azappconfig.GetSettingOptions) (azappconfig.GetSettingResponse, error) {
	return c.configurationClient.GetSetting(ctx, key, options)
}

func (c *appConfigurationClient) GetSnapshot(ctx context.Context, snapshotName string, options *azappconfig.GetSnapshotOptions) (azappconfig.GetSnapshotResponse, error) {
	return c.configurationClient.GetSnapshot(ctx, snapshotName, options)
}

func (c *appConfigurationClient) NewListSettingsForSnapshotPager(snapshotName string, options *azappconfig.ListSettingsForSnapshotOptions) *runtime.Pager[azappconfig.ListSettingsForSnapshotResponse] {
	return c.configurationClient.NewListSettingsForSnapshotPager(snapshotName, options)
}

func (c *appConfigurationClient) NewListFeatureFlagsPager(selector azappconfig.FeatureFlagSelector, options *azappconfig.ListFeatureFlagsOptions) *runtime.Pager[azappconfig.ListFeatureFlagsPageResponse] {
	return c.featureFlagClient.NewListFeatureFlagsPager(selector, options)
}
