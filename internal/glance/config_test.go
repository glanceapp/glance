package glance

import (
	"os"
	"testing"
)

func TestParseConfigVariablesIgnoresComments(t *testing.T) {
	// Set up an environment variable that should be resolved
	os.Setenv("TEST_API_KEY", "my-secret-value")
	defer os.Unsetenv("TEST_API_KEY")

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "variable in comment is not expanded",
			input:    "api-key: ${TEST_API_KEY} # Use secrets with ${secret:my-token}",
			expected: "api-key: my-secret-value # Use secrets with ${secret:my-token}",
		},
		{
			name:     "variable before comment is expanded",
			input:    "key: ${TEST_API_KEY} # this is a comment",
			expected: "key: my-secret-value # this is a comment",
		},
		{
			name:     "no comment, variable is expanded",
			input:    "key: ${TEST_API_KEY}",
			expected: "key: my-secret-value",
		},
		{
			name:     "hash inside double quotes is not a comment",
			input:    `key: "${TEST_API_KEY} # not a comment ${TEST_API_KEY}"`,
			expected: `key: "my-secret-value # not a comment my-secret-value"`,
		},
		{
			name:     "hash inside single quotes is not a comment",
			input:    `key: '${TEST_API_KEY} # not a comment ${TEST_API_KEY}'`,
			expected: `key: 'my-secret-value # not a comment my-secret-value'`,
		},
		{
			name:     "comment-only line is not expanded",
			input:    "# ${secret:some-secret}",
			expected: "# ${secret:some-secret}",
		},
		{
			name:     "multiple lines with mixed comments",
			input:    "key1: ${TEST_API_KEY}\n# comment with ${secret:x}\nkey2: ${TEST_API_KEY} # ${secret:y}",
			expected: "key1: my-secret-value\n# comment with ${secret:x}\nkey2: my-secret-value # ${secret:y}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseConfigVariables([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConfigVariables() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(result) != tt.expected {
				t.Errorf("parseConfigVariables()\ngot:  %q\nwant: %q", string(result), tt.expected)
			}
		})
	}
}

func TestFindYAMLCommentStart(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"no comment here", -1},
		{"# full line comment", 0},
		{"key: value # comment", 11},
		{`key: "value # not comment"`, -1},
		{`key: 'value # not comment'`, -1},
		{`key: "quoted" # comment`, 14},
		{"key: value#no-space-not-comment", -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := findYAMLCommentStart([]byte(tt.input))
			if got != tt.expected {
				t.Errorf("findYAMLCommentStart(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
