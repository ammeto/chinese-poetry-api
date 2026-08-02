package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	// idleTTL 是闲置的单客户端限流器的保留时长。它只需长于所跟踪的突发窗口即可：
	// 客户端静默这么久之后，其令牌桶已经回满，此时丢弃它、待下次请求再新建一个，
	// 与一直保留在效果上没有区别。
	idleTTL = 10 * time.Minute

	// sweepInterval 是回收过期限流器的间隔。
	sweepInterval = time.Minute
)

// clientLimiter 是单个客户端的限流器及其最近一次使用时间。
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter 保存限流配置及各客户端的限流器。
type RateLimiter struct {
	limiters map[string]*clientLimiter
	mu       sync.Mutex
	rps      rate.Limit
	burst    int

	stop     chan struct{}
	stopOnce sync.Once
}

// NewRateLimiter 创建限流器，并启动一个协程回收闲置超过 idleTTL 的限流器，
// 用完需调用 Stop 释放。
//
// 若不回收，这个 map 只增不减：每个出现过的客户端 IP 都会占一个条目，
// 而 ClientIP 会信任转发头，等于让攻击者控制 key 的取值范围。
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

// Stop 结束回收协程，可安全重复调用。
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

// sweep 清理闲置超过 idleTTL 的限流器。
func (rl *RateLimiter) sweep(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key, cl := range rl.limiters {
		if now.Sub(cl.lastSeen) > idleTTL {
			delete(rl.limiters, key)
		}
	}
}

// getLimiter 返回指定 key（客户端 IP）对应的限流器，
// 同时记录访问时间，以便后续回收闲置客户端的限流器。
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	now := time.Now()

	// 这里用普通 Mutex 而非 RWMutex：每次调用都要写 lastSeen，
	// RWMutex 所能带来的只读快路径已不复存在。
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

// Middleware 返回用于限流的 Gin 中间件。
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
