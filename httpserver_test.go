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

	// An anonymous tools/call is refused at the HTTP layer with the RFC 6750
	// challenge (B-10), so an OAuth-capable client discovers the flow instead
	// of reading the REST API's 401 as tool text.
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_filings",
		Arguments: map[string]any{"count_only": true},
	})
	if err == nil {
		t.Fatal("unauthenticated tools/call should be refused")
	}
	raw, _ := http.NewRequest(http.MethodPost, mcpSrv.URL+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"search_filings","arguments":{}}}`))
	raw.Header.Set("Content-Type", "application/json")
	raw.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(raw)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous tools/call status = %d, want 401", resp.StatusCode)
	}
	if h := resp.Header.Get("WWW-Authenticate"); !strings.Contains(h, "resource_metadata=") || !strings.Contains(h, prmPath) {
		t.Fatalf("WWW-Authenticate must carry the PRM pointer, got %q", h)
	}
}

func TestGatedMethodPeeksAndRestoresBody(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`
	r, _ := http.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	if got := gatedMethod(r); got != "tools/call" {
		t.Fatalf("gatedMethod = %q", got)
	}
	rest, _ := io.ReadAll(r.Body)
	if string(rest) != body {
		t.Fatalf("body not restored: %q", rest)
	}
	for _, open := range []string{"initialize", "tools/list", "resources/list", "prompts/list", "notifications/initialized"} {
		r, _ := http.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+open+`"}`))
		if got := gatedMethod(r); got != "" {
			t.Errorf("%s should stay open, got %q", open, got)
		}
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
