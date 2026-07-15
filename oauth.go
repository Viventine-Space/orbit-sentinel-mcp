package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/giantswarm/mcp-oauth/providers/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// OAuth resource-server (gate G2) parameters. The MCP binary is the RS: it
// validates RFC 9068 access tokens the Orbit Sentinel AS issues, offline,
// against the AS JWKS. The API-key path (osk_ / bare tokens) is untouched — a
// request bearing an OAuth JWT is validated as such, everything else passes
// through as before.
const (
	oauthIssuer           = "https://orbit-sentinel.viventine.com"
	oauthResource         = "https://orbit-sentinel.viventine.com/mcp"
	oauthJWKSURI          = "https://orbit-sentinel.viventine.com/.well-known/jwks.json"
	prmPath               = "/.well-known/oauth-protected-resource/mcp"
	oauthResourceMetadata = oauthIssuer + prmPath

	// jwksCacheTTL bounds how long a fetched JWKS is reused. One hour is safe
	// because key rotation is still picked up out of band: the JWKS client
	// refetches on an unknown kid (bounded by its own backoff), so a freshly
	// rotated signing key resolves without waiting out the TTL.
	jwksCacheTTL = time.Hour

	// clockSkew tolerates minor clock drift between AS and RS on exp/nbf.
	clockSkew = 5 * time.Second
)

// jwtAlgs is the closed set of asymmetric algorithms accepted for access-token
// signatures. HMAC and "none" are absent by design (algorithm-confusion).
var jwtAlgs = []jose.SignatureAlgorithm{jose.ES256, jose.RS256}

// oauthVerifier validates access tokens against the AS JWKS. fetch is injectable
// so tests can point at static fixtures or a stub endpoint; production wires the
// cached, SSRF-safe oidc.JWKSClient against the live AS.
type oauthVerifier struct {
	fetch func(ctx context.Context) (*jose.JSONWebKeySet, error)
}

func newOAuthVerifier() *oauthVerifier {
	jc := oidc.NewJWKSClient(nil, jwksCacheTTL, slog.Default())
	return &oauthVerifier{
		fetch: func(ctx context.Context) (*jose.JSONWebKeySet, error) {
			return jc.FetchJWKS(ctx, oauthJWKSURI)
		},
	}
}

// tokenError carries the RFC 6750 status + error code for the
// WWW-Authenticate header.
type tokenError struct {
	status int
	code   string // invalid_token | insufficient_scope
	desc   string
}

// verify performs the full gate-G2 check: signature against the JWKS (by kid),
// RFC 9068 typ, exp/nbf, iss, and aud — accepting a scalar string OR an array
// form via the library extractor. Returns nil on success.
func (v *oauthVerifier) verify(ctx context.Context, bearer string) *tokenError {
	jws, err := jose.ParseSigned(bearer, jwtAlgs)
	if err != nil {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "malformed JWT"}
	}
	sig := jws.Signatures[0]
	if typ, _ := sig.Header.ExtraHeaders[jose.HeaderKey("typ")].(string); typ != "at+jwt" {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "typ must be at+jwt (RFC 9068)"}
	}
	jwks, err := v.fetch(ctx)
	if err != nil {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "jwks unavailable"}
	}
	keys := jwks.Key(sig.Header.KeyID)
	if len(keys) == 0 {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "unknown kid"}
	}
	payload, err := jws.Verify(keys[0])
	if err != nil {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "bad signature"}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "bad claims"}
	}
	now := time.Now()
	if exp, ok := claims["exp"].(float64); ok && now.After(time.Unix(int64(exp), 0).Add(clockSkew)) {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "token expired"}
	}
	if nbf, ok := claims["nbf"].(float64); ok && now.Add(clockSkew).Before(time.Unix(int64(nbf), 0)) {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "token not yet valid"}
	}
	if iss, _ := claims["iss"].(string); iss != oauthIssuer {
		return &tokenError{http.StatusUnauthorized, "invalid_token", "issuer mismatch"}
	}
	// Audience — the load-bearing check: go-jose serializes a single aud as a
	// bare string, an array otherwise; GetAudienceFromClaims normalizes both.
	for _, a := range oidc.GetAudienceFromClaims(claims) {
		if a == oauthResource {
			return nil
		}
	}
	return &tokenError{http.StatusForbidden, "insufficient_scope",
		"token audience does not include " + oauthResource}
}

// middleware validates an OAuth bearer when one is present and stamps the caller
// credential onto the request context. It preserves the existing behavior:
//   - no token: the MCP handshake and tool listing stay open (scanners probe
//     them; the REST layer gates any call that needs data).
//   - osk_ / bare token: pass-through, forwarded to the REST API as today.
//   - JWT: validated as an OAuth access token; on failure the request is
//     rejected with an RFC 6750 WWW-Authenticate carrying the PRM pointer, on
//     success the JWT is forwarded unchanged (the API's dual-auth resolves it).
func (v *oauthVerifier) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r.Header.Get("Authorization"))
		switch {
		case tok == "":
			// unauthenticated: leave context untouched
		case looksLikeJWT(tok):
			if te := v.verify(r.Context(), tok); te != nil {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(
					`Bearer resource_metadata=%q, error=%q, error_description=%q`,
					oauthResourceMetadata, te.code, te.desc))
				http.Error(w, te.code+": "+te.desc, te.status)
				return
			}
			r = r.WithContext(WithAPIKey(r.Context(), tok))
		default:
			r = r.WithContext(WithAPIKey(r.Context(), tok))
		}
		next.ServeHTTP(w, r)
	})
}

// looksLikeJWT reports whether a bearer should be validated as an OAuth access
// token: an osk_ API key is never a JWT, and a compact JWS has exactly two dots.
func looksLikeJWT(tok string) bool {
	return !strings.HasPrefix(tok, "osk_") && strings.Count(tok, ".") == 2
}

// oauthMetadataHandler serves RFC 9728 Protected Resource Metadata so a client
// that gets a 401/403 can discover the authorization server.
func oauthMetadataHandler() http.Handler {
	return auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               oauthResource,
		AuthorizationServers:   []string{oauthIssuer},
		ScopesSupported:        []string{"orbit:read"},
		BearerMethodsSupported: []string{"header"},
	})
}
