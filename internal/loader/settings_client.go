// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"k8s.io/klog/v2"
)

//go:generate mockgen -destination=mocks/mock_settings_client.go -package mocks . SettingsClient

type SelectorSettingsClient struct {
	selectors []acpv1.Selector
}

type SentinelSettingsClient struct {
	sentinel        acpv1.Sentinel
	etag            *azcore.ETag
	refreshInterval string
}

type SettingsClient interface {
	GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, error)
}

func (s *SentinelSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, error) {
	sentinelSetting, err := client.GetSetting(ctx, s.sentinel.Key, &azappconfig.GetSettingOptions{Label: &s.sentinel.Label, OnlyIfChanged: s.etag})
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			var label string
			if s.sentinel.Label == "\x00" { // NUL is escaped to \x00 in golang
				label = "no"
			} else {
				label = fmt.Sprintf("'%s'", s.sentinel.Label)
			}
			switch respErr.StatusCode {
			case 404:
				klog.Warningf("Sentinel key '%s' with %s label does not exists, revisit the sentinel after %s", s.sentinel.Key, label, s.refreshInterval)
				return nil, nil
			case 304:
				klog.V(3).Infof("There's no change to the sentinel key '%s' with %s label , just exit and revisit the sentinel after %s", s.sentinel.Key, label, s.refreshInterval)
				return []azappconfig.Setting{sentinelSetting.Setting}, nil
			}
		}

		return nil, err
	}

	return []azappconfig.Setting{sentinelSetting.Setting}, nil
}

func (s *SelectorSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, error) {
	settingsToReturn := make([]azappconfig.Setting, 0)
	nullString := "\x00"

	for _, filter := range s.selectors {
		if filter.KeyFilter != nil {
			if filter.LabelFilter == nil {
				filter.LabelFilter = &nullString // NUL is escaped to \x00 in golang
			}
			selector := azappconfig.SettingSelector{
				KeyFilter:   filter.KeyFilter,
				LabelFilter: filter.LabelFilter,
				Fields:      azappconfig.AllSettingFields(),
			}
			pager := client.NewListSettingsPager(selector, nil)

			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return nil, err
				} else if len(page.Settings) > 0 {
					settingsToReturn = append(settingsToReturn, page.Settings...)
				}
			}
		} else {
			snapshot, err := client.GetSnapshot(ctx, *filter.SnapshotName, nil)
			if err != nil {
				return nil, err
			}

			if *snapshot.CompositionType != azappconfig.CompositionTypeKey {
				return nil, fmt.Errorf("compositionType for the selected snapshot '%s' must be 'key', found '%s'", *filter.SnapshotName, *snapshot.CompositionType)
			}

			pager := client.NewListSettingsForSnapshotPager(*filter.SnapshotName, nil)

			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return nil, err
				} else if len(page.Settings) > 0 {
					settingsToReturn = append(settingsToReturn, page.Settings...)
				}
			}
		}
	}

	return settingsToReturn, nil
}
