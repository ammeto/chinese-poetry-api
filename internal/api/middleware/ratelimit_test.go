package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterAllowsThenBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(1, 2)
	defer rl.Stop()

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := make([]int, 3)
	for i := range codes {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		codes[i] = w.Code
	}

	// burst of 2 is consumed, the third request in the same instant is refused
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests}, codes)
}

// TestRateLimiterEvictsIdleClients covers the limiter map, which previously had
// no eviction at all: it kept one entry per client IP ever seen, and ClientIP
// trusts forwarding headers, so the key set was effectively caller-controlled.
func TestRateLimiterEvictsIdleClients(t *testing.T) {
	rl := NewRateLimiter(10, 10)
	defer rl.Stop()

	for _, ip := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"} {
		rl.getLimiter(ip)
	}
	require.Len(t, rl.limiters, 3)

	// Not yet idle for long enough: nothing is dropped.
	rl.sweep(time.Now().Add(idleTTL / 2))
	assert.Len(t, rl.limiters, 3)

	// One client stays active while the clock advances past the TTL.
	future := time.Now().Add(idleTTL + time.Minute)
	rl.limiters["192.0.2.2"].lastSeen = future

	rl.sweep(future)
	require.Len(t, rl.limiters, 1, "only the recently seen client should survive")
	_, ok := rl.limiters["192.0.2.2"]
	assert.True(t, ok)
}

func TestRateLimiterSeparatesClients(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	a := rl.getLimiter("192.0.2.1")
	b := rl.getLimiter("192.0.2.2")
	assert.NotSame(t, a, b, "each client gets its own limiter")
	assert.Same(t, a, rl.getLimiter("192.0.2.1"), "the same client reuses its limiter")

	assert.True(t, a.Allow())
	assert.False(t, a.Allow(), "first client exhausted its burst")
	assert.True(t, b.Allow(), "second client is unaffected")
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(1000, 1000)
	defer rl.Stop()

	// Run under -race: getLimiter and sweep both mutate the shared map.
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			for range 20 {
				rl.getLimiter(string(rune('a' + i%10))).Allow()
			}
		})
	}
	wg.Go(func() {
		for range 20 {
			rl.sweep(time.Now())
		}
	})
	wg.Wait()

	assert.LessOrEqual(t, len(rl.limiters), 10)
}

func TestRateLimiterStopIsIdempotent(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	rl.Stop()
	assert.NotPanics(t, rl.Stop)
}
