package main

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorResultIsFlagged(t *testing.T) {
	r := errorResult("Error: API returned 404: not found")
	if !r.IsError {
		t.Fatal("errorResult must set IsError")
	}
	if textResult("ok").IsError {
		t.Fatal("textResult must not set IsError")
	}
}

func TestClassifyErrorText(t *testing.T) {
	cases := map[string]string{
		"Error searching filings: API returned 404: {}": "api_4xx",
		"Error: API returned 500: boom":                 "api_5xx",
		"Error: question is required":                   "invalid_args",
		"Error: request GET /x failed: dial tcp":        "transport",
	}
	for text, want := range cases {
		if got := classifyErrorText(text); got != want {
			t.Errorf("classify(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestCallerKind(t *testing.T) {
	if k, p := callerKind(""); k != "none" || p != "" {
		t.Errorf("none: %s %s", k, p)
	}
	if k, p := callerKind("osk_2b3d4f6eabcdef0123"); k != "key" || p != "2b3d4f6e" {
		t.Errorf("key: %s %s", k, p)
	}
	if k, _ := callerKind("aaa.bbb.ccc"); k != "jwt" {
		t.Errorf("jwt: %s", k)
	}
}

func TestExpandPhrase(t *testing.T) {
	cases := []struct{ q, term, want string }{
		{"What has AST SpaceMobile filed about direct-to-device spectrum sharing?", "ast", "AST SpaceMobile"},
		{"Recent SES filings", "ses", "ses"},
		{"anything about ses lately", "ses", "ses"},
		{"Compare Starlink and Kuiper", "starlink", "starlink"},
		{"Space Exploration Holdings, LLC bonds", "space exploration", "space exploration"},
	}
	for _, c := range cases {
		if got := expandPhrase(c.q, c.term); got != c.want {
			t.Errorf("expandPhrase(%q, %q) = %q, want %q", c.q, c.term, got, c.want)
		}
	}
	names := extractEntityNames("What has AST SpaceMobile filed about direct-to-device spectrum sharing?")
	if !strings.Contains(names, "AST SpaceMobile") {
		t.Errorf("extractEntityNames = %q", names)
	}
}

func TestAPIErrorText(t *testing.T) {
	err := error(&APIError{Status: 404, Body: "nf"})
	var ae *APIError
	if !errors.As(err, &ae) || err.Error() != "API returned 404: nf" {
		t.Fatalf("APIError shape changed: %v", err)
	}
}
