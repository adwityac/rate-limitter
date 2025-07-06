package utils

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response represents a standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// RateLimitResponse represents a rate limit specific response
type RateLimitResponse struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message,omitempty"`
	Allowed     bool      `json:"allowed"`
	Limit       int       `json:"limit"`
	Remaining   int       `json:"remaining"`
	ResetTime   time.Time `json:"reset_time"`
	RetryAfter  int       `json:"retry_after,omitempty"`
	WindowStart time.Time `json:"window_start,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
}

// WriteJSON writes a JSON response to the HTTP response writer
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
	}
}

// WriteSuccess writes a successful JSON response
func WriteSuccess(w http.ResponseWriter, message string, data interface{}) {
	response := Response{
		Success: true,
		Message: message,
		Data:    data,
	}
	WriteJSON(w, http.StatusOK, response)
}

// WriteError writes an error JSON response
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Success: false,
		Error:   message,
		Code:    statusCode,
	}
	WriteJSON(w, statusCode, response)
}

// WriteRateLimitAllowed writes a rate limit allowed response
func WriteRateLimitAllowed(w http.ResponseWriter, limit, remaining int, resetTime time.Time) {
	response := RateLimitResponse{
		Success:     true,
		Message:     "Request allowed",
		Allowed:     true,
		Limit:       limit,
		Remaining:   remaining,
		ResetTime:   resetTime,
		WindowStart: resetTime.Add(-time.Hour), // Assuming 1-hour window
	}
	
	// Add rate limit headers
	w.Header().Set("X-RateLimit-Limit", string(rune(limit)))
	w.Header().Set("X-RateLimit-Remaining", string(rune(remaining)))
	w.Header().Set("X-RateLimit-Reset", string(rune(resetTime.Unix())))
	
	WriteJSON(w, http.StatusOK, response)
}

// WriteRateLimitExceeded writes a rate limit exceeded response
func WriteRateLimitExceeded(w http.ResponseWriter, limit, remaining int, resetTime time.Time, retryAfter int) {
	response := RateLimitResponse{
		Success:     false,
		Message:     "Rate limit exceeded",
		Allowed:     false,
		Limit:       limit,
		Remaining:   remaining,
		ResetTime:   resetTime,
		RetryAfter:  retryAfter,
		WindowStart: resetTime.Add(-time.Hour), // Assuming 1-hour window
	}
	
	// Add rate limit headers
	w.Header().Set("X-RateLimit-Limit", string(rune(limit)))
	w.Header().Set("X-RateLimit-Remaining", string(rune(remaining)))
	w.Header().Set("X-RateLimit-Reset", string(rune(resetTime.Unix())))
	w.Header().Set("Retry-After", string(rune(retryAfter)))
	
	WriteJSON(w, http.StatusTooManyRequests, response)
}

// WriteHealthCheck writes a health check response
func WriteHealthCheck(w http.ResponseWriter, healthy bool, services map[string]bool) {
	status := "healthy"
	statusCode := http.StatusOK
	
	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}
	
	response := Response{
		Success: healthy,
		Message: status,
		Data: map[string]interface{}{
			"status":   status,
			"services": services,
			"time":     time.Now().UTC(),
		},
	}
	
	WriteJSON(w, statusCode, response)
}

// WritePing writes a simple ping response
func WritePing(w http.ResponseWriter) {
	response := Response{
		Success: true,
		Message: "pong",
		Data: map[string]interface{}{
			"timestamp": time.Now().UTC(),
			"service":   "rate-limiter",
		},
	}
	WriteJSON(w, http.StatusOK, response)
}

// WriteInternalError writes a generic internal server error response
func WriteInternalError(w http.ResponseWriter, err error) {
	message := "Internal server error"
	if err != nil {
		// In production, you might want to log the actual error
		// but not expose it to the client
		message = "Internal server error occurred"
	}
	WriteError(w, http.StatusInternalServerError, message)
}

// WriteBadRequest writes a bad request error response
func WriteBadRequest(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Bad request"
	}
	WriteError(w, http.StatusBadRequest, message)
}

// WriteUnauthorized writes an unauthorized error response
func WriteUnauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Unauthorized"
	}
	WriteError(w, http.StatusUnauthorized, message)
}

// WriteForbidden writes a forbidden error response
func WriteForbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Forbidden"
	}
	WriteError(w, http.StatusForbidden, message)
}

// WriteNotFound writes a not found error response
func WriteNotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Not found"
	}
	WriteError(w, http.StatusNotFound, message)
}