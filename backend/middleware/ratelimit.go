package middleware

import (
	"fmt"
	"net"
	"net/http"
	"solana_paywall/backend/database"
	"time"
)

// Rate Limit configuration
const (
	RequestLimit = 5               // Maximum hit request
	WindowSize   = 1 * time.Minute // In this window time range
)

func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || ip == "" {
		return r.RemoteAddr
	}
	return ip
}

func windowBucket(now time.Time) int64 {
	return now.Unix() / int64(WindowSize.Seconds())
}

// Middleware RateLimit uses Redis for distributed and process-safe limiting.
func RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		bucket := windowBucket(time.Now())
		key := fmt.Sprintf("ratelimit:recheck:%s:%d", ip, bucket)

		count, err := database.RDB.Incr(database.Ctx, key).Result()
		if err != nil {
			http.Error(w, "Rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}

		if count == 1 {
			_ = database.RDB.Expire(database.Ctx, key, WindowSize).Err()
		}

		if count > RequestLimit {
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
