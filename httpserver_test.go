package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerRoundTripper struct {
	key  string
	base http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if b.key != "" {
		req.Header.Set("Authorization", "Bearer "+b.key)
	}
	return b.base.RoundTrip(req)
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header, want string
	}{
		{"Bearer abc123", "abc123"},
		{"bearer abc123", "abc123"},
		{"Bearer  abc123 ", "abc123"},
		{"abc123", "abc123"},   // bare token: gateways forward config values raw
		{" abc123 ", "abc123"}, // bare token, padded
		{"Basic dXNlcjpwYXNz", ""},
		{"Bearer ", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := bearerToken(c.header); got != c.want {
			t.Errorf("bearerToken(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestHTTPHealthz(t *testing.T) {
	srv := httptest.NewServer(newHTTPHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestHTTPUnauthenticatedHandshakeAndGatedCalls(t *testing.T) {
	restAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":42}`))
	}))
	defer restAPI.Close()

	t.Setenv("MCP_API_URL", restAPI.URL)
	t.Setenv("MCP_API_KEY", "env-key-must-not-be-used")

	mcpSrv := httptest.NewServer(newHTTPHandler())
	defer mcpSrv.Close()

	ctx := context.Background()
	transport := &mcp.StreamableClientTransport{
		Endpoint:             mcpSrv.URL + "/mcp",
		DisableStandaloneSSE: true,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("unauthenticated initialize should succeed (scanners probe it): %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("unauthenticated tools/list should succeed: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_filings",
		Arguments: map[string]any{"count_only": true},
	})
	if err != nil {
		t.Fatalf("tools/call transport error: %v", err)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "401") && !strings.Contains(strings.ToLower(text), "error") {
		t.Fatalf("unauthenticated tools/call should surface the REST 401, got %q", text)
	}
}

func TestHTTPForwardsCallerKey(t *testing.T) {
	var gotAuth string
	restAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":42}`))
	}))
	defer restAPI.Close()

	t.Setenv("MCP_API_URL", restAPI.URL)
	t.Setenv("MCP_API_KEY", "env-key-must-not-be-used")

	mcpSrv := httptest.NewServer(newHTTPHandler())
	defer mcpSrv.Close()

	ctx := context.Background()
	transport := &mcp.StreamableClientTransport{
		Endpoint:             mcpSrv.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: bearerRoundTripper{key: "caller-key-123", base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect/initialize: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var found bool
	for _, tool := range tools.Tools {
		if tool.Name == "search_filings" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("tools/list did not include search_filings")
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_filings",
		Arguments: map[string]any{"count_only": true},
	})
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "42") {
		t.Fatalf("result text = %q, want it to contain 42", text)
	}

	if gotAuth != "Bearer caller-key-123" {
		t.Fatalf("REST Authorization = %q, want %q", gotAuth, "Bearer caller-key-123")
	}
}

func TestHTTPConcurrentKeysIsolated(t *testing.T) {
	restAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer key-")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":` + n + `}`))
	}))
	defer restAPI.Close()

	t.Setenv("MCP_API_URL", restAPI.URL)
	t.Setenv("MCP_API_KEY", "env-key-must-not-be-used")

	mcpSrv := httptest.NewServer(newHTTPHandler())
	defer mcpSrv.Close()

	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			want := strconv.Itoa(700001 + i)
			transport := &mcp.StreamableClientTransport{
				Endpoint:             mcpSrv.URL + "/mcp",
				HTTPClient:           &http.Client{Transport: bearerRoundTripper{key: "key-" + want, base: http.DefaultTransport}},
				DisableStandaloneSSE: true,
			}
			client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
			session, err := client.Connect(context.Background(), transport, nil)
			if err != nil {
				t.Errorf("key-%s connect: %v", want, err)
				return
			}
			defer session.Close()
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "search_filings",
				Arguments: map[string]any{"count_only": true},
			})
			if err != nil {
				t.Errorf("key-%s tools/call: %v", want, err)
				return
			}
			if res.IsError {
				t.Errorf("key-%s tool error: %+v", want, res.Content)
				return
			}
			var text string
			for _, c := range res.Content {
				if tc, ok := c.(*mcp.TextContent); ok {
					text += tc.Text
				}
			}
			if !strings.Contains(text, want) {
				t.Errorf("key-%s: result %q does not reflect this caller's key", want, text)
			}
		}()
	}
	wg.Wait()
}
