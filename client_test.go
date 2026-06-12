package main

import "testing"

func TestNewAPIClientDefaults(t *testing.T) {
	t.Setenv("MCP_API_URL", "")
	t.Setenv("MCP_API_KEY", "")
	c := NewAPIClient()
	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want http://localhost:8080", c.BaseURL)
	}
	if c.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", c.APIKey)
	}
	if c.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
}

func TestNewAPIClientFromEnv(t *testing.T) {
	t.Setenv("MCP_API_URL", "https://orbit-sentinel.viventine.com///")
	t.Setenv("MCP_API_KEY", "osk_testkey")
	c := NewAPIClient()
	if c.BaseURL != "https://orbit-sentinel.viventine.com" {
		t.Errorf("BaseURL = %q, want trailing slashes trimmed", c.BaseURL)
	}
	if c.APIKey != "osk_testkey" {
		t.Errorf("APIKey = %q, want osk_testkey", c.APIKey)
	}
}
