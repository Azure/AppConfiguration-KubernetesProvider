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
	settingsChan := make(chan []azappconfig.Setting)
	errChan := make(chan error)
	go getConfigurationSettings(ctx, s.selectors, client, settingsChan, errChan)

	var configSettings []azappconfig.Setting
	for {
		select {
		case configSettings = <-settingsChan:
			if len(configSettings) == 0 {
				return settingsToReturn, nil
			} else {
				settingsToReturn = append(settingsToReturn, configSettings...)
			}
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
		}
	}
}
