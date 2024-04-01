// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type ArgumentError struct {
	Field string
	Err   error
}

func (r *ArgumentError) Error() string {
	return fmt.Sprintf("%s: %v", r.Field, r.Err)
}

func NewArgumentError(field string, err error) *ArgumentError {
	return &ArgumentError{
		Field: field,
		Err:   err,
	}
}

func IsFailoverable(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(net.Error); ok {
		return true
	}

	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) &&
		(respErr.StatusCode == http.StatusTooManyRequests ||
			respErr.StatusCode == http.StatusRequestTimeout ||
			respErr.StatusCode >= http.StatusInternalServerError) {
		return true
	}

	return false
}
