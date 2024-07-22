// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/syncmap"
)

func TestResolveNotValidSecretReference(t *testing.T) {
	valueToTest := "https://fake-vault/secrets/fake-secret/version"

	secretMetadata, err := parse(valueToTest)

	assert.Nil(t, secretMetadata)
	assert.Equal(t, "invalid character 'h' looking for beginning of value", err.Error())
}

func TestResolveNotValidSecretReferenceUri(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Nil(t, secretMetadata)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/\"}', not a valid item", err.Error())
}

func TestResolveNotValidSecretReferenceUri2(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets///fake-secret\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Nil(t, secretMetadata)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/secrets///fake-secret\"}', not a valid item", err.Error())
}

func TestResolveNotValidSecretReferenceUri3(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/test/test/test\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Nil(t, secretMetadata)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/test/test/test\"}', not a valid item", err.Error())
}

func TestResolveNotValidSecretReferenceUri4(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/test////test\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Nil(t, secretMetadata)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/test////test\"}', not a valid item", err.Error())
}

func TestResolveValidSecretReferenceUriEmptyVersion(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets/fake-secret\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Equal(t, "fake-vault", secretMetadata.HostName)
	assert.Equal(t, "fake-secret", secretMetadata.SecretName)
	assert.Equal(t, "", secretMetadata.SecretVersion)
	assert.Nil(t, err)
}

func TestResolveValidSecretReferenceUri(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets/fake-secret/testversion\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Equal(t, "fake-vault", secretMetadata.HostName)
	assert.Equal(t, "fake-secret", secretMetadata.SecretName)
	assert.Equal(t, "testversion", secretMetadata.SecretVersion)
	assert.Nil(t, err)
}

func TestResolveValidSecretReferenceUri2(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets/fake-secret/testversion/notvalid\"}"

	secretMetadata, err := parse(valueToTest)

	assert.Equal(t, "fake-vault", secretMetadata.HostName)
	assert.Equal(t, "fake-secret", secretMetadata.SecretName)
	assert.Equal(t, "testversion", secretMetadata.SecretVersion)
	assert.Nil(t, err)
}

func TestResolveSecretReferenceSetting(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://FAKE-VAULT/secrets/fake-secret/testversion/notvalid\"}"
	defaultCredential := mockTokeCredential{}
	clients := &syncmap.Map{}
	resolver := &KeyVaultConnector{
		DefaultTokenCredential: defaultCredential,
		Clients:                clients,
	}

	secretMetadata, _ := parse(valueToTest)
	resolvedSecret, err := resolver.Resolve(*secretMetadata, context.Background())

	assert.Contains(t, err.Error(), "fake-vault")
	length := 0
	clients.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	assert.Equal(t, 1, length)
	assert.Nil(t, resolvedSecret.Value)
}

type mockTokeCredential struct {
}

func (mocktc mockTokeCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	fakeToken := azcore.AccessToken{
		Token:     "fakeaccesstoke",
		ExpiresOn: time.Now(),
	}
	return fakeToken, nil
}
