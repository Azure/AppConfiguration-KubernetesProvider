// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"testing"
)

func TestReplaceColonsWithDoubleUnderscores(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "String with colons",
			input:    "Section:SecretName",
			expected: "Section__SecretName",
		},
		{
			name:     "String with multiple colons",
			input:    "Section:SubSection:SecretName",
			expected: "Section__SubSection__SecretName",
		},
		{
			name:     "String without colons",
			input:    "SecretName",
			expected: "SecretName",
		},
		{
			name:     "String with existing double underscores",
			input:    "Section__SecretName",
			expected: "Section__SecretName",
		},
		{
			name:     "Mixed string with colons and underscores",
			input:    "Section:Secret__Name",
			expected: "Section__Secret__Name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := replaceColonsWithDoubleUnderscores(test.input)
			if result != test.expected {
				t.Errorf("Expected %q but got %q", test.expected, result)
			}
		})
	}
}