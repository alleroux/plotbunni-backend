package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	max        float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens = min(b.max, b.tokens+now.Sub(b.lastRefill).Seconds()*b.refillRate)
	b.lastRefill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64
	burst   float64
}

func newRateLimiter(perSecond, burst float64) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    perSecond,
		burst:   burst,
	}
	go func() {
		for range time.Tick(5 * time.Minute) {
			rl.mu.Lock()
			for ip, b := range rl.buckets {
				b.mu.Lock()
				stale := time.Since(b.lastRefill) > 10*time.Minute
				b.mu.Unlock()
				if stale {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{tokens: rl.burst, max: rl.burst, refillRate: rl.rate, lastRefill: time.Now()}
		rl.buckets[ip] = b
	}
	rl.mu.Unlock()
	return b.allow()
}

func (rl *rateLimiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the real client IP, trusting X-Real-IP set by nginx.
// X-Forwarded-For is NOT trusted directly because clients can spoof it.
// In production the backend must not be reachable without going through the proxy.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[:i]
	}
	return addr
}
