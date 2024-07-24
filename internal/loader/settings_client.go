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
	"k8s.io/klog"
)

//go:generate mockgen -destination=mocks/mock_settings_client.go -package mocks . SettingsClient

type EtagSettingsClient struct {
	etags       map[acpv1.Selector][]*azcore.ETag
	etagChanged bool
}

type SentinelSettingsClient struct {
	sentinel        acpv1.Sentinel
	etag            *azcore.ETag
	refreshInterval string
}
type SelectorSettingsClient struct {
	selectors []acpv1.Selector
}

type SettingsClient interface {
	GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, map[acpv1.Selector][]*azcore.ETag, error)
}

func (s *EtagSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, map[acpv1.Selector][]*azcore.ETag, error) {
	nullString := "\x00"
	for filter, pageEtags := range s.etags {
		if filter.KeyFilter != nil {
			if filter.LabelFilter == nil {
				filter.LabelFilter = &nullString // NUL is escaped to \x00 in golang
			}
			selector := azappconfig.SettingSelector{
				KeyFilter:   filter.KeyFilter,
				LabelFilter: filter.LabelFilter,
				Fields:      azappconfig.AllSettingFields(),
			}

			conditions := make([]azcore.MatchConditions, 0)
			for _, etag := range pageEtags {
				conditions = append(conditions, azcore.MatchConditions{IfNoneMatch: etag})
			}

			pager := client.NewListSettingsPager(selector, &azappconfig.ListSettingsOptions{
				MatchConditions: conditions,
			})

			pageCount := 0
			for pager.More() {
				pageCount++
				page, err := pager.NextPage(context.Background())
				if err != nil {
					return nil, nil, err
				}
				if page.ETag != nil {
					s.etagChanged = true
					break
				}
			}

			if !s.etagChanged && pageCount != len(pageEtags) {
				s.etagChanged = true
			}

			if s.etagChanged {
				break
			}
		}
	}

	return nil, nil, nil
}

func (s *SentinelSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, map[acpv1.Selector][]*azcore.ETag, error) {
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
				return nil, nil, nil
			case 304:
				klog.V(3).Infof("There's no change to the sentinel key '%s' with %s label , just exit and revisit the sentinel after %s", s.sentinel.Key, label, s.refreshInterval)
				return []azappconfig.Setting{sentinelSetting.Setting}, nil, nil
			}
		}

		return nil, nil, err
	}

	return []azappconfig.Setting{sentinelSetting.Setting}, nil, nil
}

func (s *SelectorSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, map[acpv1.Selector][]*azcore.ETag, error) {
	nullString := "\x00"
	settingsToReturn := make([]azappconfig.Setting, 0)
	refreshedEtags := make(map[acpv1.Selector][]*azcore.ETag)

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
			latestEtags := make([]*azcore.ETag, 0)

			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return nil, nil, err
				} else if len(page.Settings) > 0 {
					settingsToReturn = append(settingsToReturn, page.Settings...)
					latestEtags = append(latestEtags, page.ETag)
				}
			}
			// update the etags for the filter
			refreshedEtags[filter] = latestEtags
		} else {
			snapshot, err := client.GetSnapshot(ctx, *filter.SnapshotName, nil)
			if err != nil {
				return nil, nil, err
			}

			if *snapshot.CompositionType != azappconfig.CompositionTypeKey {
				return nil, nil, fmt.Errorf("compositionType for the selected snapshot '%s' must be 'key', found '%s'", *filter.SnapshotName, *snapshot.CompositionType)
			}

			pager := client.NewListSettingsForSnapshotPager(*filter.SnapshotName, nil)

			for pager.More() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return nil, nil, err
				} else if len(page.Settings) > 0 {
					settingsToReturn = append(settingsToReturn, page.Settings...)
				}
			}
		}
	}

	return settingsToReturn, refreshedEtags, nil
}
