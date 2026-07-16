package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runHTTP serves the MCP server over Streamable HTTP at addr. Every /mcp
// request must carry a Bearer token, which is forwarded per-request to the REST
// API; the MCP_API_KEY env var is not used as a fallback in this mode.
func runHTTP(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHTTPHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

func newHTTPHandler() http.Handler {
	return newHTTPHandlerWithVerifier(newOAuthVerifier())
}

// newHTTPHandlerWithVerifier builds the HTTP mux with an injectable OAuth
// verifier so tests can point validation at fixtures or a stub JWKS endpoint.
func newHTTPHandlerWithVerifier(v *oauthVerifier) http.Handler {
	client := NewAPIClient()
	client.APIKey = "" // HTTP callers authenticate per-request; no env-key fallback
	server := newServer(client)

	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle(prmPath, oauthMetadataHandler())
	mux.Handle("/mcp", v.middleware(rateLimitUnauth(handler)))

	return mux
}

// bearerToken extracts the token from an Authorization header. Accepts both
// "Bearer <token>" and a bare token — gateways like Smithery forward config
// values to upstream headers raw, with no way to add the scheme prefix.
func bearerToken(header string) string {
	const prefix = "bearer "
	if len(header) >= len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	header = strings.TrimSpace(header)
	if header == "" || strings.ContainsAny(header, " \t") {
		return ""
	}
	return header
}
