// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package jsonc

import (
	"testing"
)

func TestStripComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    `{"name": "value"}`,
			expected: `{"name": "value"}`,
		},
		{
			name:     "line comment",
			input:    `{"name": "value"} // this is a comment`,
			expected: `{"name": "value"} `,
		},
		{
			name:     "block comment",
			input:    `{"name": /* comment */ "value"}`,
			expected: `{"name":               "value"}`,
		},
		{
			name:     "comment in string should be preserved",
			input:    `{"comment": "This // is not a comment"}`,
			expected: `{"comment": "This // is not a comment"}`,
		},
		{
			name:     "escaped quote in string",
			input:    `{"escaped": "She said \"Hello\""}`,
			expected: `{"escaped": "She said \"Hello\""}`,
		},
		{
			name: "multiline block comment",
			input: `{
				/* This is a
				   multiline comment */
				"name": "value"
			}`,
			expected: `{
				            
                           
				"name": "value"
			}`,
		},
		// Edge cases
		{
			name:     "empty string",
			input:    ``,
			expected: ``,
		},
		{
			name: "only whitespace",
			input: `   	 
   `,
			expected: `   	 
   `,
		},
		{
			name:     "only line comment",
			input:    `// just a comment`,
			expected: ``,
		},
		{
			name:     "only block comment",
			input:    `/* just a comment */`,
			expected: `                    `,
		},
		{
			name: "multiple line comments",
			input: `// comment 1
// comment 2
{"name": "value"} // comment 3`,
			expected: `

{"name": "value"} `,
		},
		{
			name:     "nested-like comments in strings",
			input:    `{"path": "/* not a comment */", "url": "http://example.com"}`,
			expected: `{"path": "/* not a comment */", "url": "http://example.com"}`,
		},
		{
			name:     "comment markers in strings with escapes",
			input:    `{"msg": "He said \"// not a comment\"", "note": "File: C:\\temp\\*test*.txt"}`,
			expected: `{"msg": "He said \"// not a comment\"", "note": "File: C:\\temp\\*test*.txt"}`,
		},
		{
			name: "block comment at start of line",
			input: `{
/* comment */
"name": "value"
}`,
			expected: `{
             
"name": "value"
}`,
		},
		{
			name: "block comment at end of line",
			input: `{
"name": "value" /* comment */
}`,
			expected: `{
"name": "value"              
}`,
		},
		{
			name:     "consecutive comments",
			input:    `{"name": "value"} // comment1 /* still comment */`,
			expected: `{"name": "value"} `,
		},
		{
			name:     "comment with forward slash in string",
			input:    `{"url": "https://example.com/path"} // Real comment`,
			expected: `{"url": "https://example.com/path"} `,
		},
		{
			name:     "incomplete block comment at end",
			input:    `{"name": "value"} /* incomplete comment`,
			expected: `{"name": "value"}                      `,
		},
		{
			name:     "incomplete line comment (just //)",
			input:    `{"name": "value"} //`,
			expected: `{"name": "value"} `,
		},
		{
			name:     "block comment with nested /* inside",
			input:    `{"name": /* outer /* inner */ "value"}`,
			expected: `{"name":                      "value"}`,
		},
		{
			name:     "multiple consecutive slashes",
			input:    `{"name": "value"} /// triple slash comment`,
			expected: `{"name": "value"} `,
		},
		{
			name:     "block comment with asterisks inside",
			input:    `{"name": /* comment with * asterisks ** */ "value"}`,
			expected: `{"name":                                   "value"}`,
		},
		{
			name:     "string with backslash escape sequences",
			input:    `{"path": "C:\\Users\\test\\file.txt", "newline": "line1\nline2"} // Comment`,
			expected: `{"path": "C:\\Users\\test\\file.txt", "newline": "line1\nline2"} `,
		},
		{
			name:     "comment immediately after string quote",
			input:    `{"name": "value"/* comment */, "other": "data"}`,
			expected: `{"name": "value"             , "other": "data"}`,
		},
		{
			name:     "comment between key and colon",
			input:    `{"name" /* comment */ : "value"}`,
			expected: `{"name"               : "value"}`,
		},
		{
			name:     "comment between colon and value",
			input:    `{"name": /* comment */ "value"}`,
			expected: `{"name":               "value"}`,
		},
		{
			name:     "unicode in comments and strings",
			input:    `{"emoji": "😀🎉", "note": "test"} // Comment with émojis 🚀`,
			expected: `{"emoji": "😀🎉", "note": "test"} `,
		},
		{
			name:     "windows line endings with comments",
			input:    "{\r\n\t\"name\": \"value\" // comment\r\n}",
			expected: "{\r\n\t\"name\": \"value\" \r\n}",
		},
		{
			name:     "mixed line endings",
			input:    "{\n\t\"name\": \"value\" // comment\r\n\t\"other\": \"data\"\r}",
			expected: "{\n\t\"name\": \"value\" \r\n\t\"other\": \"data\"\r}",
		},
		{
			name:     "comment at very end of input",
			input:    `{"name": "value"}//`,
			expected: `{"name": "value"}`,
		},
		{
			name:     "string with escaped backslash before quote",
			input:    `{"path": "C:\\test\\"} // Comment`,
			expected: `{"path": "C:\\test\\"} `,
		},
		{
			name:     "multiple escaped quotes in string",
			input:    `{"msg": "Say \"Hello\" and \"Goodbye\""} /* comment */`,
			expected: `{"msg": "Say \"Hello\" and \"Goodbye\""}              `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripComments([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("stripComments() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}
