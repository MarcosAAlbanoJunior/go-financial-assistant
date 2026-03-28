package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func webhookSourceMiddleware(evolutionHost string, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			logger.Warn("webhook bloqueado: RemoteAddr inválido", "remote_addr", r.RemoteAddr)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		addrs, err := net.LookupHost(evolutionHost)
		if err != nil {
			logger.Warn("webhook bloqueado: não foi possível resolver host Evolution",
				"host", evolutionHost, "error", err)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		for _, addr := range addrs {
			if addr == ip {
				next.ServeHTTP(w, r)
				return
			}
		}

		logger.Warn("webhook bloqueado: IP não autorizado",
			"ip", ip, "evolution_host", evolutionHost, "resolved_addrs", addrs)
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

func adminRateLimitMiddleware(rl *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !rl.allow(ip) {
			http.Error(w, "too many requests, tente novamente em breve", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractBearerSecret(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string][]time.Time
	max      int
	window   time.Duration
}

func newIPRateLimiter(max int, window time.Duration) *ipRateLimiter {
	rl := &ipRateLimiter{
		visitors: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var valid []time.Time
	for _, t := range rl.visitors[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.max {
		rl.visitors[ip] = valid
		return false
	}

	rl.visitors[ip] = append(valid, now)
	return true
}

func (rl *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.window)
		for ip, reqs := range rl.visitors {
			var valid []time.Time
			for _, t := range reqs {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.visitors, ip)
			} else {
				rl.visitors[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}
