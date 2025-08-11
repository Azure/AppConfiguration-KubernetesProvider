// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package jsonc provides support for parsing JSON with comments (JSONC).
// This package extends standard JSON support to handle line comments (//) and block comments (/* */).
package jsonc

// StripComments removes comments from JSONC while preserving string literals.
func StripComments(data []byte) []byte {
	var result []byte
	var inString bool
	var escaped bool

	for i := 0; i < len(data); i++ {
		char := data[i]

		if escaped {
			// Previous character was a backslash, so this character is escaped
			result = append(result, char)
			escaped = false
			continue
		}

		if inString {
			result = append(result, char)
			if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}

		// Not in a string
		if char == '"' {
			inString = true
			result = append(result, char)
			continue
		}
		// Check for line comment
		if char == '/' && i+1 < len(data) && data[i+1] == '/' {
			// Skip until end of line
			i++ // skip the second '/'
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			// Include the newline character to preserve line numbers, but step back one
			// because the for loop will increment i again
			if i < len(data) {
				result = append(result, data[i])
			}
			continue
		}

		// Check for block comment
		if char == '/' && i+1 < len(data) && data[i+1] == '*' {
			// Replace /* with spaces
			result = append(result, ' ', ' ')
			i += 2 // skip /*

			// Find closing */ and replace content with spaces, preserving newlines
			for i < len(data) {
				if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
					// Replace */ with spaces
					result = append(result, ' ', ' ')
					i += 2 // skip */
					break
				}
				// Preserve newlines and carriage returns to maintain line numbers
				if data[i] == '\n' || data[i] == '\r' {
					result = append(result, data[i])
				} else {
					// Replace other characters with spaces to preserve column positions
					result = append(result, ' ')
				}
				i++
			}
			// Step back one because the for loop will increment i again
			i--
			continue
		}

		// Regular character, not in string, not a comment
		result = append(result, char)
	}

	return result
}
