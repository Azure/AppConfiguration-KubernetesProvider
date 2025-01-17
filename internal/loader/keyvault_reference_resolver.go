// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"golang.org/x/sync/syncmap"
)

type KeyVaultConnector struct {
	DefaultTokenCredential azcore.TokenCredential
	Clients                *syncmap.Map //map[string]*azsecrets.Client
}

type SecretReference struct {
	Uri string `json:"uri,omitempty"`
}

type KeyVaultSecretMetadata struct {
	HostName      string
	SecretName    string
	SecretVersion string
}

type SecretReferenceResolver interface {
	Resolve(secretMetadata KeyVaultSecretMetadata, ctx context.Context) (azsecrets.GetSecretResponse, error)
}

func (resolver *KeyVaultConnector) Resolve(
	secretMetadata KeyVaultSecretMetadata,
	ctx context.Context) (azsecrets.GetSecretResponse, error) {
	var secretClient any
	var ok bool
	if secretClient, ok = resolver.Clients.Load(secretMetadata.HostName); !ok {
		newSecretClient, err := azsecrets.NewClient("https://"+secretMetadata.HostName, resolver.DefaultTokenCredential, nil)
		if err != nil {
			return azsecrets.GetSecretResponse{}, err
		}
		secretClient = newSecretClient
		resolver.Clients.Store(secretMetadata.HostName, newSecretClient)
	}

	return secretClient.(*azsecrets.Client).GetSecret(ctx, secretMetadata.SecretName, secretMetadata.SecretVersion, nil)
}

func parse(settingValue string) (*KeyVaultSecretMetadata, error) {
	var secretRef SecretReference
	//
	// Valid Key Vault Reference setting value to parse
	// {
	// 	"uri":"https://{keyVaultName}.vaule.azure.net/secrets/{secretName}/{secretVersion}"
	// }
	if err := json.Unmarshal([]byte(settingValue), &secretRef); err != nil {
		return nil, err
	}
	secretUrl, err := url.Parse(secretRef.Uri)
	if err != nil {
		return nil, err
	}

	trimmedPath := strings.TrimPrefix(secretUrl.Path, "/")
	segments := strings.Split(trimmedPath, "/")
	if len(segments) < 2 || strings.ToLower(segments[0]) != "secrets" || segments[1] == "" {
		return nil, fmt.Errorf("not a valid url in Key Vault reference type setting '%s', not a valid item", settingValue)
	}

	var secretVersion string
	if len(segments) == 2 { // no version be specified
		secretVersion = ""
	} else {
		secretVersion = segments[2]
	}
	secretName := segments[1]
	hostName := strings.ToLower(secretUrl.Host)

	result := &KeyVaultSecretMetadata{
		HostName:      hostName,
		SecretName:    secretName,
		SecretVersion: secretVersion,
	}

	return result, nil
}
