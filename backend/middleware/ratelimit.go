package middleware

import (
	"fmt"
	"net"
	"net/http"
	"solana_paywall/backend/database"
	"strings"
	"time"
)

// Rate Limit configuration
const WindowSize = 1 * time.Minute // In this window time range

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

func rateLimitPolicy(path string) (string, int64) {
	switch {
	case strings.HasPrefix(path, "/api/checkout/detect"):
		return "checkout_detect", 30
	case strings.HasPrefix(path, "/api/checkout/recheck"):
		return "checkout_recheck", 12
	case strings.HasPrefix(path, "/api/checkout/manual-verify"):
		return "checkout_manual_verify", 10
	default:
		return "default", 10
	}
}

// Middleware RateLimit uses Redis for distributed and process-safe limiting.
func RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		bucket := windowBucket(time.Now())
		tag, limit := rateLimitPolicy(r.URL.Path)
		key := fmt.Sprintf("ratelimit:%s:%s:%d", tag, ip, bucket)

		count, err := database.RDB.Incr(database.Ctx, key).Result()
		if err != nil {
			http.Error(w, "Rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}

		if count == 1 {
			_ = database.RDB.Expire(database.Ctx, key, WindowSize).Err()
		}

		if count > limit {
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
