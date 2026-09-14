package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/sachinkaru123/gochin/pkg/router"
)

// RateLimitConfig configures the token-bucket limiter.
type RateLimitConfig struct {
	// RequestsPerMinute is the sustained budget per client; 0 disables.
	RequestsPerMinute int
	// Burst is the bucket capacity, defaulting to RequestsPerMinute.
	Burst int
	// KeyFunc identifies the client; defaults to the client IP.
	KeyFunc func(*router.Context) string
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	burst    float64
	stopOnce sync.Once
}

// RateLimit throttles clients with an in-process token bucket.
//
// The budget is per process, so N instances allow N times the limit; a shared
// store is the multi-instance answer.
func RateLimit(cfg RateLimitConfig) router.Middleware {
	if cfg.RequestsPerMinute <= 0 {
		return func(next router.Handler) router.Handler { return next }
	}
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.RequestsPerMinute
	}
	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = func(c *router.Context) string { return c.ClientIP() }
	}

	l := &limiter{
		buckets: map[string]*bucket{},
		rate:    float64(cfg.RequestsPerMinute) / 60.0,
		burst:   float64(cfg.Burst),
	}
	l.startJanitor()

	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			key := keyFunc(c)
			allowed, remaining := l.allow(key)

			c.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerMinute))
			c.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				retry := int(1/l.rate) + 1
				c.Header().Set("Retry-After", strconv.Itoa(retry))
				return router.Errorf(429, "rate limit exceeded; retry in %ds", retry)
			}
			return next(c)
		}
	}
}

func (l *limiter) allow(key string) (bool, int) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}

	b.tokens += now.Sub(b.lastSeen).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false, 0
	}
	b.tokens--
	return true, int(b.tokens)
}

// startJanitor evicts idle buckets. Without it the map grows without bound,
// keyed by an attacker-controlled value — a denial of service in itself.
func (l *limiter) startJanitor() {
	l.stopOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cutoff := time.Now().Add(-10 * time.Minute)
				l.mu.Lock()
				for k, b := range l.buckets {
					if b.lastSeen.Before(cutoff) {
						delete(l.buckets, k)
					}
				}
				l.mu.Unlock()
			}
		}()
	})
}
