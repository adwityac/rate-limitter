package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPHandler struct {
	mux *http.ServeMux
}

// NewHTTPHandler creates a new HTTP handler with routes
func NewHTTPHandler() *HTTPHandler {
	h := &HTTPHandler{
		mux: http.NewServeMux(),
	}
	
	h.setupRoutes()
	return h
}

// setupRoutes configures all the routes for the handler
func (h *HTTPHandler) setupRoutes() {
	// Health check endpoint
	h.mux.HandleFunc("/ping", h.handlePing)
	
	// Rate limit test endpoint
	h.mux.HandleFunc("/api/test", h.handleTest)
	
	// Rate limit status endpoint
	h.mux.HandleFunc("/api/status", h.handleStatus)
	
	// Generic protected endpoint
	h.mux.HandleFunc("/api/protected", h.handleProtected)
}

// GetMux returns the configured mux
func (h *HTTPHandler) GetMux() *http.ServeMux {
	return h.mux
}

// ServeHTTP implements http.Handler interface
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// writeJSONResponse writes a JSON response with the given status code
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// handlePing handles the /ping endpoint for health checks
func (h *HTTPHandler) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.HandleMethodNotAllowed(w, r)
		return
	}
	
	response := map[string]interface{}{
		"message":   "pong",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "healthy",
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// handleTest handles the /api/test endpoint for rate limit testing
func (h *HTTPHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		h.HandleMethodNotAllowed(w, r)
		return
	}
	
	clientIP := getClientIP(r)
	
	response := map[string]interface{}{
		"message":    "Rate limit test endpoint",
		"method":     r.Method,
		"client_ip":  clientIP,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"user_agent": r.UserAgent(),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// handleStatus handles the /api/status endpoint to check rate limit status
func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.HandleMethodNotAllowed(w, r)
		return
	}
	
	clientIP := getClientIP(r)
	
	response := map[string]interface{}{
		"message":   "Rate limiter status",
		"client_ip": clientIP,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"note":      "Rate limit information would be retrieved from Redis here",
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// handleProtected handles protected endpoints that require rate limiting
func (h *HTTPHandler) handleProtected(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	
	response := map[string]interface{}{
		"message":   "Protected resource accessed successfully",
		"method":    r.Method,
		"client_ip": clientIP,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      "This is protected content",
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// HandleRateLimitExceeded handles requests that exceed rate limits
func (h *HTTPHandler) HandleRateLimitExceeded(w http.ResponseWriter, r *http.Request, limit int, windowSeconds int, retryAfter int) {
	clientIP := getClientIP(r)
	
	// Set rate limit headers
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Window", fmt.Sprintf("%d", windowSeconds))
	w.Header().Set("X-RateLimit-Retry-After", fmt.Sprintf("%d", retryAfter))
	
	response := map[string]interface{}{
		"error":        "Rate limit exceeded",
		"message":      fmt.Sprintf("Too many requests. Limit: %d per %d seconds", limit, windowSeconds),
		"client_ip":    clientIP,
		"retry_after":  retryAfter,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusTooManyRequests, response)
}

// HandleNotFound handles 404 errors
func (h *HTTPHandler) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"error":     "Not Found",
		"message":   "The requested resource was not found",
		"path":      r.URL.Path,
		"method":    r.Method,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusNotFound, response)
}

// HandleMethodNotAllowed handles 405 errors
func (h *HTTPHandler) HandleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"error":     "Method Not Allowed",
		"message":   fmt.Sprintf("Method %s is not allowed for this endpoint", r.Method),
		"method":    r.Method,
		"path":      r.URL.Path,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusMethodNotAllowed, response)
}

// HandleInternalError handles 500 errors
func (h *HTTPHandler) HandleInternalError(w http.ResponseWriter, r *http.Request, err error) {
	response := map[string]interface{}{
		"error":     "Internal Server Error",
		"message":   "An internal server error occurred",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	// Log the actual error (in production, use proper logging)
	fmt.Printf("Internal error: %v\n", err)
	
	writeJSONResponse(w, http.StatusInternalServerError, response)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for requests through proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// HealthCheck provides a simple health check response
func (h *HTTPHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "rate-limiter",
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}