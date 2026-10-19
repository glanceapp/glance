package glance

import (
	"bytes"
	"os"
	"testing"
)

func TestParseConfigVariablesSkipsCommentedOutLines(t *testing.T) {
	os.Unsetenv("GLANCE_TEST_UNSET")

	// The variable is only referenced on a commented-out line, so an unset env
	// var must not cause an error and the content must be left untouched. See #474.
	input := []byte("server:\n  port: 8080\n# token: ${GLANCE_TEST_UNSET}\n")

	output, err := parseConfigVariables(input)
	if err != nil {
		t.Fatalf("expected no error for commented-out variable, got: %v", err)
	}
	if !bytes.Equal(output, input) {
		t.Fatalf("commented-out line was modified:\nwant %q\ngot  %q", input, output)
	}
}

func TestParseConfigVariablesErrorsOnActiveUnsetVariable(t *testing.T) {
	os.Unsetenv("GLANCE_TEST_UNSET")

	_, err := parseConfigVariables([]byte("token: ${GLANCE_TEST_UNSET}\n"))
	if err == nil {
		t.Fatal("expected an error for an unset env var on an active line")
	}
}

func TestParseConfigVariablesSubstitutesActiveVariable(t *testing.T) {
	os.Setenv("GLANCE_TEST_SET", "s3cr3t")
	defer os.Unsetenv("GLANCE_TEST_SET")

	output, err := parseConfigVariables([]byte("token: ${GLANCE_TEST_SET}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(output, []byte("token: s3cr3t")) {
		t.Fatalf("variable was not substituted, got: %q", output)
	}
}

func TestParseConfigVariablesKeepsEscapedVariable(t *testing.T) {
	output, err := parseConfigVariables([]byte(`token: \${NOT_A_VAR}` + "\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(output, []byte("token: ${NOT_A_VAR}")) {
		t.Fatalf("escaped variable was not preserved, got: %q", output)
	}
}
