// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import "fmt"

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
