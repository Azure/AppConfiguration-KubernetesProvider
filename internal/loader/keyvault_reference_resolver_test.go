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

	secretUriSegment, err := parse(valueToTest)

	assert.Nil(t, secretUriSegment)
	assert.Equal(t, "invalid character 'h' looking for beginning of value", err.Error())
}

func TestResolveNotValidSecretReferenceUri(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Nil(t, secretUriSegment)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/\"}', not a valid item", err.Error())
}

func TestResolveNotValidSecretReferenceUri2(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets///fake-secret\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Nil(t, secretUriSegment)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/secrets///fake-secret\"}', not a valid item", err.Error())
}

func TestResolveNotValidSecretReferenceUri3(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/test/test/test\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Nil(t, secretUriSegment)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/test/test/test\"}', not a valid item", err.Error())
}

func TestResolveNotValidSecretReferenceUri4(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/test////test\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Nil(t, secretUriSegment)
	assert.Equal(t, "not a valid url in Key Vault reference type setting '{ \"uri\":\"https://fake-vault/test////test\"}', not a valid item", err.Error())
}

func TestResolveValidSecretReferenceUriEmptyVersion(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets/fake-secret\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Equal(t, "fake-vault", secretUriSegment.HostName)
	assert.Equal(t, "fake-secret", secretUriSegment.SecretName)
	assert.Equal(t, "", secretUriSegment.SecretVersion)
	assert.Nil(t, err)
}

func TestResolveValidSecretReferenceUri(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets/fake-secret/testversion\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Equal(t, "fake-vault", secretUriSegment.HostName)
	assert.Equal(t, "fake-secret", secretUriSegment.SecretName)
	assert.Equal(t, "testversion", secretUriSegment.SecretVersion)
	assert.Nil(t, err)
}

func TestResolveValidSecretReferenceUri2(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://fake-vault/secrets/fake-secret/testversion/notvalid\"}"

	secretUriSegment, err := parse(valueToTest)

	assert.Equal(t, "fake-vault", secretUriSegment.HostName)
	assert.Equal(t, "fake-secret", secretUriSegment.SecretName)
	assert.Equal(t, "testversion", secretUriSegment.SecretVersion)
	assert.Nil(t, err)
}

func TestResolveSecretReferenceSetting(t *testing.T) {
	valueToTest := "{ \"uri\":\"https://FAKE-VAULT/secrets/fake-secret/testversion/notvalid\"}"
	defaultCredential := mockTokeCredential{}
	clients := &syncmap.Map{}
	resolver := &KeyVaultReferenceResolver{
		DefaultTokenCredential: defaultCredential,
		Clients:                clients,
	}

	secretUriSegment, _ := parse(valueToTest)
	resolvedValue, err := resolver.Resolve(*secretUriSegment, context.Background())

	assert.Contains(t, err.Error(), "fake-vault")
	length := 0
	clients.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	assert.Equal(t, 1, length)
	assert.Nil(t, resolvedValue)
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
