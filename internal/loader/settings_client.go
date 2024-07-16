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
	selectors    []acpv1.Selector
	etags        map[acpv1.Selector][]*azcore.ETag
	etagChanged  bool
	sentinelUsed bool
}

type SettingsClient interface {
	GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, map[acpv1.Selector][]*azcore.ETag, error)
}

// GetSettings retrieves settings from the Azure App Configuration if the etag has changed, 
// When the etag has not changed, it will return empty settings and etags
func (s *SelectorSettingsClient) GetSettings(ctx context.Context, client *azappconfig.Client) ([]azappconfig.Setting, map[acpv1.Selector][]*azcore.ETag, error) {
	nullString := "\x00"
	settingsToReturn := make([]azappconfig.Setting, 0)
	refreshedEtags := make(map[acpv1.Selector][]*azcore.ETag)
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

	if s.etagChanged {
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

				if !s.sentinelUsed {
					refreshedEtags[filter] = latestEtags
				}
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

		if s.sentinelUsed {
			for filter := range s.etags {
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
						page, err := pager.NextPage(context.Background())
						if err != nil {
							return nil, nil, err
						}
						if page.ETag != nil {
							latestEtags = append(latestEtags, page.ETag)
						}
					}
					refreshedEtags[filter] = latestEtags
				}
			}
		}
	}

	return settingsToReturn, refreshedEtags, nil
}
