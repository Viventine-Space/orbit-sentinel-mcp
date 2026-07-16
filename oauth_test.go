package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giantswarm/mcp-oauth/providers/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const rsFixtures = "/private/tmp/claude-501/-Users-avc-projects-viventine/8957a054-9d3c-4ad2-bfdf-98ecf7305180/scratchpad/rs-fixtures"

func loadFixtureJWKS(t *testing.T) *jose.JSONWebKeySet {
	t.Helper()
	b, err := os.ReadFile(rsFixtures + "/jwks.json")
	if err != nil {
		t.Skipf("RS fixtures unavailable: %v", err)
	}
	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(b, &jwks); err != nil {
		t.Fatal(err)
	}
	return &jwks
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(rsFixtures + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// gateG2Cases drives every gate-G2 shape against a verifier: the string-or-array
// aud gotcha (both 200), wrong aud (403 insufficient_scope), expired (401
// invalid_token), and the dual-auth divergence from a hard-gate RS — a missing
// token stays open (200) so the unauthenticated MCP handshake keeps working.
func gateG2Cases(t *testing.T, v *oauthVerifier) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(prmPath, oauthMetadataHandler())
	mux.Handle("/mcp", v.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})))
	rs := httptest.NewServer(mux)
	defer rs.Close()

	call := func(token string) (int, string) {
		req, _ := http.NewRequest("GET", rs.URL+"/mcp", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode, resp.Header.Get("WWW-Authenticate")
	}

	cases := []struct {
		name      string
		token     string
		wantCode  int
		wantError string // expected error= token in WWW-Authenticate, "" = none
	}{
		{"valid scalar aud", readFixture(t, "valid_scalar_aud.jwt"), 200, ""},
		{"valid array aud", readFixture(t, "valid_array_aud.jwt"), 200, ""},
		{"wrong aud", readFixture(t, "wrong_aud.jwt"), 403, "insufficient_scope"},
		{"expired", readFixture(t, "expired.jwt"), 401, "invalid_token"},
		{"no token (open handshake)", "", 200, ""},
	}
	for _, c := range cases {
		code, wa := call(c.token)
		if code != c.wantCode {
			t.Fatalf("%-26s want %d got %d (WWW-Authenticate=%q)", c.name, c.wantCode, code, wa)
		}
		if c.wantError == "" {
			continue
		}
		if !strings.Contains(wa, `error="`+c.wantError+`"`) {
			t.Fatalf("%-26s WWW-Authenticate=%q, want error=%q", c.name, wa, c.wantError)
		}
		if !strings.Contains(wa, `resource_metadata="`+oauthResourceMetadata+`"`) {
			t.Fatalf("%-26s WWW-Authenticate=%q, missing resource_metadata pointer", c.name, wa)
		}
	}

	// PRM discovery (RFC 9728) — resource exact, AS listed.
	resp, err := http.Get(rs.URL + prmPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PRM discovery: got %d", resp.StatusCode)
	}
	var prm struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prm); err != nil {
		t.Fatal(err)
	}
	if prm.Resource != oauthResource {
		t.Fatalf("PRM resource = %q, want %q", prm.Resource, oauthResource)
	}
	if len(prm.AuthorizationServers) == 0 || prm.AuthorizationServers[0] != oauthIssuer {
		t.Fatalf("PRM authorization_servers = %v, want [%q]", prm.AuthorizationServers, oauthIssuer)
	}
}

// TestRSGateG2 ports the verified reference prototype into the repo, run against
// the static fixtures AND a live cached JWKS endpoint (the production path).
func TestRSGateG2(t *testing.T) {
	jwks := loadFixtureJWKS(t)

	t.Run("static fixtures", func(t *testing.T) {
		v := &oauthVerifier{fetch: func(context.Context) (*jose.JSONWebKeySet, error) {
			return jwks, nil
		}}
		gateG2Cases(t, v)
	})

	t.Run("live cached JWKS", func(t *testing.T) {
		var fetches int32
		jwksSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&fetches, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jwks)
		}))
		defer jwksSrv.Close()

		pool := x509.NewCertPool()
		pool.AddCert(jwksSrv.Certificate())
		jc := oidc.NewJWKSClientWithOptions(oidc.JWKSClientOptions{
			CacheTTL:       time.Minute,
			AllowPrivateIP: true, // stub runs on loopback
			RootCAs:        pool,
		})
		jwksURL := jwksSrv.URL + "/jwks"
		v := &oauthVerifier{fetch: func(ctx context.Context) (*jose.JSONWebKeySet, error) {
			return jc.FetchJWKS(ctx, jwksURL)
		}}
		gateG2Cases(t, v)

		if n := atomic.LoadInt32(&fetches); n != 1 {
			t.Fatalf("JWKS endpoint hit %d times, want 1 (cache should coalesce)", n)
		}
	})
}

// TestOAuthDualAuth proves the API-key path is intact alongside OAuth: an osk_
// key and a valid OAuth JWT both reach the REST API, each forwarded unchanged.
func TestOAuthDualAuth(t *testing.T) {
	jwks := loadFixtureJWKS(t)
	v := &oauthVerifier{fetch: func(context.Context) (*jose.JSONWebKeySet, error) {
		return jwks, nil
	}}

	var gotAuth string
	restAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":42}`))
	}))
	defer restAPI.Close()
	t.Setenv("MCP_API_URL", restAPI.URL)
	t.Setenv("MCP_API_KEY", "env-key-must-not-be-used")

	mcpSrv := httptest.NewServer(newHTTPHandlerWithVerifier(v))
	defer mcpSrv.Close()

	callWith := func(t *testing.T, key string) {
		transport := &mcp.StreamableClientTransport{
			Endpoint:             mcpSrv.URL + "/mcp",
			HTTPClient:           &http.Client{Transport: bearerRoundTripper{key: key, base: http.DefaultTransport}},
			DisableStandaloneSSE: true,
		}
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		session, err := client.Connect(context.Background(), transport, nil)
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		defer session.Close()
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "search_filings",
			Arguments: map[string]any{"count_only": true},
		})
		if err != nil {
			t.Fatalf("tools/call: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error: %+v", res.Content)
		}
	}

	t.Run("osk_ api key passes through", func(t *testing.T) {
		callWith(t, "osk_live_key_123")
		if gotAuth != "Bearer osk_live_key_123" {
			t.Fatalf("REST Authorization = %q, want the osk_ key forwarded", gotAuth)
		}
	})

	t.Run("valid oauth jwt forwarded unchanged", func(t *testing.T) {
		jwt := readFixture(t, "valid_scalar_aud.jwt")
		callWith(t, jwt)
		if gotAuth != "Bearer "+jwt {
			t.Fatalf("REST Authorization = %q, want the validated JWT forwarded unchanged", gotAuth)
		}
	})
}
