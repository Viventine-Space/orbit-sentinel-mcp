package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postMCP(t *testing.T, url string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestRateLimitUnauthenticated(t *testing.T) {
	srv := httptest.NewServer(newHTTPHandler())
	defer srv.Close()

	var limited bool
	for i := 0; i < unauthBurst+5; i++ {
		if postMCP(t, srv.URL, map[string]string{"CF-Connecting-IP": "203.0.113.7"}) == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatalf("unauthenticated burst of %d requests was never rate limited", unauthBurst+5)
	}

	// A different client IP gets its own bucket.
	if code := postMCP(t, srv.URL, map[string]string{"CF-Connecting-IP": "203.0.113.8"}); code == http.StatusTooManyRequests {
		t.Fatal("distinct IP was limited by another IP's bucket")
	}
}

func TestRateLimitSkipsAuthenticated(t *testing.T) {
	srv := httptest.NewServer(newHTTPHandler())
	defer srv.Close()

	for i := 0; i < unauthBurst+5; i++ {
		code := postMCP(t, srv.URL, map[string]string{
			"CF-Connecting-IP": "203.0.113.9",
			"Authorization":    "Bearer some-key",
		})
		if code == http.StatusTooManyRequests {
			t.Fatalf("authenticated request %d was rate limited", i)
		}
	}
}
