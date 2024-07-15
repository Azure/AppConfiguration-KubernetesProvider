// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
)

//go:generate mockgen -destination=mocks/mock_settings_client.go -package mocks . SettingsClient

type SelectorSettingsClient struct {
	selectors []acpv1.Selector
}

type SentinelSettingsClient struct {
	etagChanged bool
	etags       map[acpv1.Selector][]*azcore.ETag
}

type SettingsClient interface {
	GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, error)
}

func (s *SentinelSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, error) {
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

			refreshedEtags := make([]*azcore.ETag, 0)
			for pager.More() {
				page, err := pager.NextPage(context.Background())
				if err != nil {
					return nil, err
				}
				if page.Settings != nil {
					s.etagChanged = true
				}
				refreshedEtags = append(refreshedEtags, page.ETag)
			}

			if len(refreshedEtags) != len(pageEtags) {
				s.etagChanged = true
			}
			s.etags[filter] = refreshedEtags
		}
	}

	return []azappconfig.Setting{}, nil
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
