package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Limiter interface defines the rate limiting operations
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetTime time.Time, err error)
}

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// WindowSize is the time window for rate limiting (e.g., 1 minute)
	WindowSize time.Duration
	// MaxRequests is the maximum number of requests allowed in the window
	MaxRequests int
	// KeyFunc extracts the key from the request (e.g., IP address, user ID)
	KeyFunc func(*http.Request) string
	// SkipFunc determines if rate limiting should be skipped for this request
	SkipFunc func(*http.Request) bool
	// OnLimitExceeded is called when rate limit is exceeded
	OnLimitExceeded func(http.ResponseWriter, *http.Request, string)
}

// RateLimitMiddleware creates a new rate limiting middleware
func RateLimitMiddleware(limiter Limiter, config RateLimitConfig) func(http.Handler) http.Handler {
	// Set default values
	if config.WindowSize == 0 {
		config.WindowSize = time.Minute
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 100
	}
	if config.KeyFunc == nil {
		config.KeyFunc = defaultKeyFunc
	}
	if config.OnLimitExceeded == nil {
		config.OnLimitExceeded = defaultOnLimitExceeded
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting if configured
			if config.SkipFunc != nil && config.SkipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract key from request
			key := config.KeyFunc(r)
			if key == "" {
				// If no key can be extracted, allow the request
				next.ServeHTTP(w, r)
				return
			}

			// Create rate limit key with prefix
			rateLimitKey := fmt.Sprintf("rate_limit:%s", key)

			// Check rate limit
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			allowed, remaining, resetTime, err := limiter.Allow(ctx, rateLimitKey, config.MaxRequests, config.WindowSize)
			if err != nil {
				// Log error but don't block request
				// In production, you might want to handle this differently
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.MaxRequests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

			if !allowed {
				// Rate limit exceeded
				w.Header().Set("Retry-After", strconv.FormatInt(int64(time.Until(resetTime).Seconds()), 10))
				config.OnLimitExceeded(w, r, key)
				return
			}

			// Request is allowed, proceed
			next.ServeHTTP(w, r)
		})
	}
}

// defaultKeyFunc extracts IP address from request
func defaultKeyFunc(r *http.Request) string {
	// Check for X-Forwarded-For header (proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP from the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check for X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// defaultOnLimitExceeded handles rate limit exceeded cases
func defaultOnLimitExceeded(w http.ResponseWriter, r *http.Request, key string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	
	response := fmt.Sprintf(`{
		"error": "Rate Limit Exceeded",
		"message": "Too many requests. Please try again later.",
		"code": %d,
		"timestamp": "%s"
	}`, http.StatusTooManyRequests, time.Now().UTC().Format(time.RFC3339))
	
	w.Write([]byte(response))
}

// Predefined key functions for common use cases

// IPKeyFunc extracts client IP address
func IPKeyFunc(r *http.Request) string {
	return defaultKeyFunc(r)
}

// UserKeyFunc extracts user ID from request context or header
func UserKeyFunc(r *http.Request) string {
	// Try to get user ID from context (set by authentication middleware)
	if userID := r.Context().Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			return fmt.Sprintf("user:%s", id)
		}
	}

	// Try to get user ID from header
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return fmt.Sprintf("user:%s", userID)
	}

	// Fall back to IP-based limiting
	return fmt.Sprintf("ip:%s", defaultKeyFunc(r))
}

// APIKeyFunc extracts API key from request
func APIKeyFunc(r *http.Request) string {
	// Try Authorization header
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return fmt.Sprintf("api_key:%s", auth[7:])
		}
	}

	// Try X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return fmt.Sprintf("api_key:%s", apiKey)
	}

	// Fall back to IP-based limiting
	return fmt.Sprintf("ip:%s", defaultKeyFunc(r))
}

// CompositeKeyFunc combines multiple key sources
func CompositeKeyFunc(funcs ...func(*http.Request) string) func(*http.Request) string {
	return func(r *http.Request) string {
		var parts []string
		for _, f := range funcs {
			if key := f(r); key != "" {
				parts = append(parts, key)
			}
		}
		return strings.Join(parts, ":")
	}
}

// Predefined skip functions

// SkipHealthChecks skips rate limiting for health check endpoints
func SkipHealthChecks(r *http.Request) bool {
	path := r.URL.Path
	return path == "/health" || path == "/ready"
}

// SkipInternalIPs skips rate limiting for internal IP addresses
func SkipInternalIPs(r *http.Request) bool {
	ip := defaultKeyFunc(r)
	
	// Check for localhost
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}
	
	// Check for private IP ranges
	if strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "172.16.") ||
		strings.HasPrefix(ip, "172.17.") ||
		strings.HasPrefix(ip, "172.18.") ||
		strings.HasPrefix(ip, "172.19.") ||
		strings.HasPrefix(ip, "172.20.") ||
		strings.HasPrefix(ip, "172.21.") ||
		strings.HasPrefix(ip, "172.22.") ||
		strings.HasPrefix(ip, "172.23.") ||
		strings.HasPrefix(ip, "172.24.") ||
		strings.HasPrefix(ip, "172.25.") ||
		strings.HasPrefix(ip, "172.26.") ||
		strings.HasPrefix(ip, "172.27.") ||
		strings.HasPrefix(ip, "172.28.") ||
		strings.HasPrefix(ip, "172.29.") ||
		strings.HasPrefix(ip, "172.30.") ||
		strings.HasPrefix(ip, "172.31.") {
		return true
	}
	
	return false
}

// Convenience functions for common configurations

// NewIPRateLimiter creates a simple IP-based rate limiter
func NewIPRateLimiter(limiter Limiter, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return RateLimitMiddleware(limiter, RateLimitConfig{
		WindowSize:  window,
		MaxRequests: maxRequests,
		KeyFunc:     IPKeyFunc,
		SkipFunc:    SkipHealthChecks,
	})
}

// NewUserRateLimiter creates a user-based rate limiter
func NewUserRateLimiter(limiter Limiter, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return RateLimitMiddleware(limiter, RateLimitConfig{
		WindowSize:  window,
		MaxRequests: maxRequests,
		KeyFunc:     UserKeyFunc,
		SkipFunc:    SkipHealthChecks,
	})
}

// NewAPIKeyRateLimiter creates an API key-based rate limiter
func NewAPIKeyRateLimiter(limiter Limiter, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return RateLimitMiddleware(limiter, RateLimitConfig{
		WindowSize:  window,
		MaxRequests: maxRequests,
		KeyFunc:     APIKeyFunc,
		SkipFunc:    SkipHealthChecks,
	})
}