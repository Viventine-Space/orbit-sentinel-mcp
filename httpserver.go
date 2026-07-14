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
	mux.Handle("/mcp", withBearerAuth(handler))

	return mux
}

// withBearerAuth rejects requests lacking a Bearer token and stamps the token
// onto the request context so tool handlers forward the caller's own key.
func withBearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := bearerToken(r.Header.Get("Authorization"))
		if key == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing or invalid Authorization header"}`))
			return
		}
		next.ServeHTTP(w, r.WithContext(WithAPIKey(r.Context(), key)))
	})
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header,
// returning "" if the scheme is absent or the token is empty.
func bearerToken(header string) string {
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
