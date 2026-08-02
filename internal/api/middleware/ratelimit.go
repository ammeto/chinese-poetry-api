package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	// idleTTL is how long an unused per-client limiter is kept. It only needs
	// to outlive the burst it is tracking: once a client has been quiet for
	// this long its limiter is back at full tokens, so dropping it and creating
	// a fresh one on the next request is indistinguishable from keeping it.
	idleTTL = 10 * time.Minute

	// sweepInterval is how often expired limiters are collected.
	sweepInterval = time.Minute
)

// clientLimiter is a per-client limiter plus the time it was last used.
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter holds rate limiting configuration
type RateLimiter struct {
	limiters map[string]*clientLimiter
	mu       sync.Mutex
	rps      rate.Limit
	burst    int

	stop     chan struct{}
	stopOnce sync.Once
}

// NewRateLimiter creates a new rate limiter and starts a goroutine that evicts
// limiters idle for longer than idleTTL. Call Stop to release it.
//
// Without eviction the map only ever grows: it holds one entry per client IP
// ever seen, and ClientIP trusts forwarding headers, so the set of keys is
// effectively attacker-controlled.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*clientLimiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		stop:     make(chan struct{}),
	}

	go rl.sweepLoop()

	return rl
}

// Stop ends the eviction goroutine. Safe to call more than once.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

func (rl *RateLimiter) sweepLoop() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case now := <-ticker.C:
			rl.sweep(now)
		}
	}
}

// sweep drops limiters that have been idle for longer than idleTTL.
func (rl *RateLimiter) sweep(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key, cl := range rl.limiters {
		if now.Sub(cl.lastSeen) > idleTTL {
			delete(rl.limiters, key)
		}
	}
}

// getLimiter returns a rate limiter for the given key (IP address), recording
// the access so an idle client's limiter can later be swept.
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	now := time.Now()

	// A plain Mutex rather than RWLock: every call writes lastSeen, so the
	// read-only fast path a RWMutex would buy no longer exists.
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if cl, exists := rl.limiters[key]; exists {
		cl.lastSeen = now
		return cl.limiter
	}

	limiter := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[key] = &clientLimiter{limiter: limiter, lastSeen: now}

	return limiter
}

// Middleware returns a Gin middleware function for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "too many requests, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
