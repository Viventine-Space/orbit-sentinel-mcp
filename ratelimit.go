package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Unauthenticated /mcp traffic (handshake, tool listing) has no per-key limit
// at the REST layer, so throttle it per client IP. Authenticated requests are
// rate-limited per key by the API and pass through untouched.
const (
	unauthRPM   = 30
	unauthBurst = 10
)

type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimitUnauth throttles requests that carry no API key, keyed by client
// IP. Must wrap the handler inside withCallerKey so the context reflects the
// caller's auth state. Stale limiters are evicted to bound memory.
func rateLimitUnauth(next http.Handler) http.Handler {
	var mu sync.Mutex
	limiters := make(map[string]*ipLimiterEntry)

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			cutoff := time.Now().Add(-1 * time.Hour)
			for k, v := range limiters {
				if v.lastSeen.Before(cutoff) {
					delete(limiters, k)
				}
			}
			mu.Unlock()
		}
	}()

	// Shared fallback bucket used when the map is full (an attacker cycling
	// many real IPs) — degrades to one collective limit instead of growing
	// memory without bound inside the container's 128M cap.
	overflow := rate.NewLimiter(rate.Limit(float64(unauthRPM)/60.0), unauthBurst)
	const maxTrackedIPs = 50000

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if e, ok := limiters[ip]; ok {
			e.lastSeen = time.Now()
			return e.limiter
		}
		if len(limiters) >= maxTrackedIPs {
			cutoff := time.Now().Add(-10 * time.Minute)
			for k, v := range limiters {
				if v.lastSeen.Before(cutoff) {
					delete(limiters, k)
				}
			}
			if len(limiters) >= maxTrackedIPs {
				return overflow
			}
		}
		lim := rate.NewLimiter(rate.Limit(float64(unauthRPM)/60.0), unauthBurst)
		limiters[ip] = &ipLimiterEntry{limiter: lim, lastSeen: time.Now()}
		return lim
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKeyFromContext(r.Context()) != "" {
			next.ServeHTTP(w, r)
			return
		}
		if !getLimiter(clientIP(r)).Allow() {
			w.Header().Set("Retry-After", "2")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded for unauthenticated requests"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP prefers CF-Connecting-IP — spoof-proof in this topology because
// only Cloudflare can reach the load balancer — falling back to the socket
// peer for internal (WireGuard / LB) traffic.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
