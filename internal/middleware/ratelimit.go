package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var (
	visitors = make(map[string]*visitor)
	mu       sync.Mutex
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimitMiddleware ограничивает количество запросов
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		if _, ok := visitors[ip]; !ok {
			// 5 запросов в минуту для логина
			visitors[ip] = &visitor{
				limiter:  rate.NewLimiter(rate.Every(time.Minute), 5),
				lastSeen: time.Now(),
			}
		} else {
			visitors[ip].lastSeen = time.Now()
		}
		mu.Unlock()

		// Очищаем старых посетителей
		go cleanupVisitors()

		if !visitors[ip].limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Слишком много запросов. Попробуйте через минуту.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func cleanupVisitors() {
	mu.Lock()
	defer mu.Unlock()

	for ip, v := range visitors {
		if time.Since(v.lastSeen) > 10*time.Minute {
			delete(visitors, ip)
		}
	}
}
