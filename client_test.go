package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewAPIClientDefaults(t *testing.T) {
	t.Setenv("MCP_API_URL", "")
	t.Setenv("MCP_API_KEY", "")
	c := NewAPIClient()
	if c.BaseURL != "https://orbit-sentinel.viventine.com" {
		t.Errorf("BaseURL = %q, want https://orbit-sentinel.viventine.com", c.BaseURL)
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

// fakeAPI answers each request with the next of statuses (the last repeats),
// a 429 carrying code and Retry-After retryAfter, and counts the requests.
func fakeAPI(t *testing.T, code, retryAfter string, statuses ...int) (*APIClient, *int32, *[]string) {
	t.Helper()
	var n int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(atomic.AddInt32(&n, 1)) - 1
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		st := statuses[min(i, len(statuses)-1)]
		w.Header().Set("Content-Type", "application/json")
		if st == http.StatusTooManyRequests {
			if retryAfter != "" {
				w.Header().Set("Retry-After", retryAfter)
			}
			w.WriteHeader(st)
			_, _ = fmt.Fprintf(w, `{"code":%q,"error":"slow down"}`+"\n", code)
			return
		}
		w.WriteHeader(st)
		_, _ = w.Write([]byte(`{"total":7}`))
	}))
	t.Cleanup(srv.Close)
	return &APIClient{BaseURL: srv.URL, HTTPClient: srv.Client()}, &n, &bodies
}

func TestRetriesRateLimitedThenSucceeds(t *testing.T) {
	c, n, _ := fakeAPI(t, "rate_limited", "0", 429, 200)
	data, err := c.SearchFilings(context.Background(), map[string]string{"q": "x"})
	if err != nil {
		t.Fatalf("want success after one retry, got %v", err)
	}
	if string(data) != `{"total":7}` || *n != 2 {
		t.Fatalf("data %s after %d requests, want the 200 body after 2", data, *n)
	}
}

func TestRetriesConcurrencyExceededPOSTBodyIntact(t *testing.T) {
	c, n, bodies := fakeAPI(t, "concurrency_exceeded", "", 429, 200)
	if _, err := c.SearchSemantic(context.Background(), "spectrum sharing", 5, "", 0); err != nil {
		t.Fatalf("want success after one retry, got %v", err)
	}
	if *n != 2 || (*bodies)[0] != (*bodies)[1] || !strings.Contains((*bodies)[1], "spectrum sharing") {
		t.Fatalf("requests %d, bodies %q: the retry must resend the whole body", *n, *bodies)
	}
}

func TestRetryGivesUpAfterTwoRetries(t *testing.T) {
	c, n, _ := fakeAPI(t, "rate_limited", "0", 429)
	_, err := c.SearchFilings(context.Background(), nil)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 429 {
		t.Fatalf("want the 429 APIError, got %v", err)
	}
	if *n != 3 {
		t.Fatalf("requests = %d, want 3 (one try, two retries)", *n)
	}
}

func TestNoRetryForOtherRejections(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
	}{{"daily_budget_exceeded", 429}, {"auth_throttled", 429}, {"rate_limited", 503}, {"endpoint_denied", 403}} {
		c, n, _ := fakeAPI(t, tc.code, "0", tc.status)
		if _, err := c.SearchFilings(context.Background(), nil); err == nil {
			t.Fatalf("%s: want an error", tc.code)
		}
		if *n != 1 {
			t.Errorf("%s %d: requests = %d, want 1 (no retry)", tc.code, tc.status, *n)
		}
	}
}

func TestRetryHonorsRetryAfterWithCap(t *testing.T) {
	var waits []time.Duration
	orig := sleepCtx
	sleepCtx = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	t.Cleanup(func() { sleepCtx = orig })

	for _, tc := range []struct {
		retryAfter string
		lo         time.Duration
	}{{"1", time.Second}, {"6", maxRetryWait}, {"", time.Second}, {"soon", time.Second}} {
		waits = nil
		c, _, _ := fakeAPI(t, "rate_limited", tc.retryAfter, 429, 429, 200)
		if _, err := c.SearchFilings(context.Background(), nil); err != nil {
			t.Fatalf("Retry-After %q: %v", tc.retryAfter, err)
		}
		if len(waits) != 2 {
			t.Fatalf("Retry-After %q: %d waits, want 2", tc.retryAfter, len(waits))
		}
		for _, w := range waits {
			if w < tc.lo || w >= tc.lo+retryJitter {
				t.Errorf("Retry-After %q: wait %v, want [%v, %v)", tc.retryAfter, w, tc.lo, tc.lo+retryJitter)
			}
		}
	}
}

func TestRetryWaitStopsOnCancel(t *testing.T) {
	c, n, _ := fakeAPI(t, "rate_limited", "2", 429, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.SearchFilings(ctx, nil); err == nil {
		t.Fatal("want the 429 error when the context ends during the wait")
	}
	if el := time.Since(start); el > 500*time.Millisecond || *n != 1 {
		t.Fatalf("returned after %v and %d requests, want promptly after 1", el, *n)
	}
}
