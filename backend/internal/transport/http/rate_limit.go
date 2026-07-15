package httptransport

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"discerne/backend/internal/config"
)

type mutationRateLimiter struct {
	mu       sync.Mutex
	clients  map[string]rateLimitWindow
	requests int
	window   time.Duration
	now      func() time.Time
	checks   int
}

type rateLimitWindow struct {
	count   int
	resetAt time.Time
}

func newMutationRateLimiter(cfg config.Config) *mutationRateLimiter {
	requests := cfg.MutationRateLimit.Requests
	if requests <= 0 {
		requests = config.DefaultMutationRateLimitRequests
	}

	window := cfg.MutationRateLimit.Window
	if window <= 0 {
		window = config.DefaultMutationRateLimitWindow
	}

	return &mutationRateLimiter{
		clients:  make(map[string]rateLimitWindow),
		requests: requests,
		window:   window,
		now:      time.Now,
	}
}

func (limiter *mutationRateLimiter) middleware(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isRateLimitedMutation(r) {
			next.ServeHTTP(w, r)
			return
		}

		key := rateLimitKey(r, cfg.DeviceCookieName)
		allowed, retryAfter := limiter.allow(key)
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
			respondError(w, http.StatusTooManyRequests, "rate_limited")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (limiter *mutationRateLimiter) allow(key string) (bool, time.Duration) {
	now := limiter.now()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.checks++
	if limiter.checks%128 == 0 {
		limiter.deleteExpired(now)
	}

	window, ok := limiter.clients[key]
	if !ok || !now.Before(window.resetAt) {
		limiter.clients[key] = rateLimitWindow{
			count:   1,
			resetAt: now.Add(limiter.window),
		}
		return true, 0
	}

	if window.count >= limiter.requests {
		return false, window.resetAt.Sub(now)
	}

	window.count++
	limiter.clients[key] = window
	return true, 0
}

func (limiter *mutationRateLimiter) deleteExpired(now time.Time) {
	for key, window := range limiter.clients {
		if !now.Before(window.resetAt) {
			delete(limiter.clients, key)
		}
	}
}

func isRateLimitedMutation(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}

	if r.URL.Path == "/api/v1/quizzes/today/attempt" {
		return true
	}

	return strings.HasPrefix(r.URL.Path, "/api/v1/attempts/") &&
		strings.HasSuffix(r.URL.Path, "/answers")
}

func rateLimitKey(r *http.Request, cookieName string) string {
	if cookieName != "" {
		if cookie, err := r.Cookie(cookieName); err == nil {
			if value := strings.TrimSpace(cookie.Value); value != "" {
				return "device:" + value
			}
		}
	}

	return "ip:" + clientIP(r)
}

func clientIP(r *http.Request) string {
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	forwardedFor := r.Header.Get("X-Forwarded-For")
	for _, item := range strings.Split(forwardedFor, ",") {
		if ip := strings.TrimSpace(item); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}

	return "unknown"
}

func retryAfterSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 1
	}

	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}
