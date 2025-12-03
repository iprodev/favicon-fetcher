// Package ratelimit provides request rate limiting using the token bucket algorithm.
// It supports both global rate limiting and per-IP rate limiting.
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"

	"faviconsvc/pkg/metrics"
)

// Limiter provides rate limiting functionality using token bucket algorithm.
type Limiter struct {
	globalBucket  *TokenBucket
	ipBuckets     sync.Map // IP address -> *TokenBucket
	ipRate        int      // requests per second per IP
	ipBurst       int      // burst capacity per IP
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// TokenBucket implements the token bucket algorithm for rate limiting.
type TokenBucket struct {
	rate       float64   // tokens per second
	capacity   float64   // maximum tokens
	tokens     float64   // current tokens
	lastUpdate time.Time // last token update
	mu         sync.Mutex
}

// NewLimiter creates a new rate limiter with the specified limits.
// globalRate: global requests per second (0 = unlimited)
// globalBurst: global burst capacity
// ipRate: requests per second per IP (0 = unlimited)
// ipBurst: burst capacity per IP
// Returns nil if both rates are 0 (completely unlimited).
func NewLimiter(globalRate, globalBurst, ipRate, ipBurst int) *Limiter {
	// If both rates are 0, no limiting needed
	if globalRate == 0 && ipRate == 0 {
		return nil
	}

	l := &Limiter{
		ipRate:      ipRate,
		ipBurst:     ipBurst,
		stopCleanup: make(chan struct{}),
	}

	if globalRate > 0 {
		l.globalBucket = newTokenBucket(float64(globalRate), float64(globalBurst))
	}

	// Cleanup old IP buckets every 5 minutes
	l.cleanupTicker = time.NewTicker(5 * time.Minute)
	go l.cleanupLoop()

	return l
}

// Stop stops the cleanup goroutine.
func (l *Limiter) Stop() {
	close(l.stopCleanup)
	l.cleanupTicker.Stop()
}

// Allow checks if a request from the given IP should be allowed.
// Returns true if allowed, false if rate limited.
func (l *Limiter) Allow(ip string) bool {
	// Check global limit first
	if l.globalBucket != nil && !l.globalBucket.allow() {
		metrics.Get().IncError("rate_limit_global")
		return false
	}

	// Check IP-specific limit
	if l.ipRate > 0 {
		bucket := l.getOrCreateIPBucket(ip)
		if !bucket.allow() {
			metrics.Get().IncError("rate_limit_ip")
			return false
		}
	}

	return true
}

func (l *Limiter) getOrCreateIPBucket(ip string) *TokenBucket {
	val, ok := l.ipBuckets.Load(ip)
	if ok {
		return val.(*TokenBucket)
	}

	bucket := newTokenBucket(float64(l.ipRate), float64(l.ipBurst))
	actual, _ := l.ipBuckets.LoadOrStore(ip, bucket)
	return actual.(*TokenBucket)
}

func (l *Limiter) cleanupLoop() {
	for {
		select {
		case <-l.stopCleanup:
			return
		case <-l.cleanupTicker.C:
			l.cleanup()
		}
	}
}

func (l *Limiter) cleanup() {
	// Remove IP buckets that haven't been used in 10 minutes
	cutoff := time.Now().Add(-10 * time.Minute)
	l.ipBuckets.Range(func(key, value interface{}) bool {
		bucket := value.(*TokenBucket)
		bucket.mu.Lock()
		if bucket.lastUpdate.Before(cutoff) {
			l.ipBuckets.Delete(key)
		}
		bucket.mu.Unlock()
		return true
	})
}

func newTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity,
		lastUpdate: time.Now(),
	}
}

func (b *TokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.lastUpdate = now

	// Add tokens based on elapsed time
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	// Check if we have at least 1 token
	if b.tokens >= 1.0 {
		b.tokens--
		return true
	}

	return false
}

// Middleware returns an HTTP middleware that applies rate limiting.
func Middleware(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract IP address
			ip := getClientIP(r)

			// Check rate limit
			if !limiter.Allow(ip) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request.
// It checks X-Forwarded-For and X-Real-IP headers first,
// then falls back to RemoteAddr.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		if ip := parseIP(xff); ip != "" {
			return ip
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		if ip := parseIP(xri); ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func parseIP(s string) string {
	// Handle comma-separated list (X-Forwarded-For)
	for idx := 0; idx < len(s); idx++ {
		if s[idx] == ',' {
			s = s[:idx]
			break
		}
	}

	// Trim whitespace
	s = trimSpace(s)

	// Validate IP
	if net.ParseIP(s) != nil {
		return s
	}

	return ""
}

func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}
