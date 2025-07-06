package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"rate-limiter/internal/limitter"
)

// Config holds application configuration
type Config struct {
	ServerPort    string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Environment   string
}

// RedisClient wraps redis operations and implements limiter.RedisClient
type RedisClient struct {
	client *redis.Client
}

// Implement limitter.RedisClient interface methods
func (r *RedisClient) Get(ctx context.Context, key string) limitter.StringCmd {
	return &StringCmdWrapper{r.client.Get(ctx, key)}
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) limitter.StatusCmd {
	return &StatusCmdWrapper{r.client.Set(ctx, key, value, expiration)}
}

func (r *RedisClient) Incr(ctx context.Context, key string) limitter.IntCmd {
	return &IntCmdWrapper{r.client.Incr(ctx, key)}
}

func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) limitter.BoolCmd {
	return &BoolCmdWrapper{r.client.Expire(ctx, key, expiration)}
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) limitter.IntCmd {
	return &IntCmdWrapper{r.client.Del(ctx, keys...)}
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) TTL(ctx context.Context, key string) limitter.DurationCmd {
	return &DurationCmdWrapper{r.client.TTL(ctx, key)}
}

func (r *RedisClient) Ping(ctx context.Context) limitter.StatusCmd {
	return &StatusCmdWrapper{r.client.Ping(ctx)}
}

func (r *RedisClient) HealthCheck(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisClient) PoolStats() *redis.PoolStats {
	return r.client.PoolStats()
}

func (r *RedisClient) Pipeline() limitter.Pipeline {
	return &PipelineWrapper{r.client.Pipeline()}
}


func (r *RedisClient) ZRemRangeByScore(ctx context.Context, key string, min, max string) limitter.IntCmd {
	return &IntCmdWrapper{r.client.ZRemRangeByScore(ctx, key, min, max)}
}

func (r *RedisClient) ZCard(ctx context.Context, key string) limitter.IntCmd {
	return &IntCmdWrapper{r.client.ZCard(ctx, key)}
}

func (r *RedisClient) ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) limitter.StringSliceCmd {
	// Handle the WITHSCORES option if present
	if len(args) > 0 {
		if str, ok := args[0].(string); ok && str == "WITHSCORES" {
			return &StringSliceCmdWrapper{r.convertZRangeWithScoresToStringSlice(ctx, key, start, stop)}
		}
	}
	return &StringSliceCmdWrapper{r.client.ZRange(ctx, key, start, stop)}
}

// Fixed ZAdd method - use redis.Z instead of *redis.Z
func (r *RedisClient) ZAdd(ctx context.Context, key string, score float64, member interface{}) limitter.IntCmd {
	return &IntCmdWrapper{r.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member})}
}

func (r *RedisClient) ZCount(ctx context.Context, key string, min, max string) limitter.IntCmd {
	return &IntCmdWrapper{r.client.ZCount(ctx, key, min, max)}
}

// Helper method to convert ZRangeWithScores to StringSliceCmd
func (r *RedisClient) convertZRangeWithScoresToStringSlice(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd {
	// Get the ZRangeWithScores result
	zCmd := r.client.ZRangeWithScores(ctx, key, start, stop)
	
	// Create a new StringSliceCmd
	cmd := redis.NewStringSliceCmd(ctx, "zrange", key, start, stop, "withscores")
	
	// Convert the result
	if zSlice, err := zCmd.Result(); err == nil {
		result := make([]string, 0, len(zSlice)*2)
		for _, z := range zSlice {
			result = append(result, fmt.Sprintf("%v", z.Member))
			result = append(result, fmt.Sprintf("%g", z.Score))
		}
		cmd.SetVal(result)
	} else {
		cmd.SetErr(err)
	}
	
	return cmd
}

// Command wrapper types to implement limiter interfaces
type StringCmdWrapper struct {
	cmd *redis.StringCmd
}

func (w *StringCmdWrapper) Result() (string, error) {
	return w.cmd.Result()
}

func (w *StringCmdWrapper) Err() error {
	return w.cmd.Err()
}

func (w *StringCmdWrapper) Val() string {
	return w.cmd.Val()
}

type StatusCmdWrapper struct {
	cmd *redis.StatusCmd
}

func (w *StatusCmdWrapper) Result() (string, error) {
	return w.cmd.Result()
}

func (w *StatusCmdWrapper) Err() error {
	return w.cmd.Err()
}

func (w *StatusCmdWrapper) Val() string {
	return w.cmd.Val()
}

type IntCmdWrapper struct {
	cmd *redis.IntCmd
}

func (w *IntCmdWrapper) Result() (int64, error) {
	return w.cmd.Result()
}

func (w *IntCmdWrapper) Err() error {
	return w.cmd.Err()
}

func (w *IntCmdWrapper) Val() int64 {
	return w.cmd.Val()
}

type BoolCmdWrapper struct {
	cmd *redis.BoolCmd
}

func (w *BoolCmdWrapper) Result() (bool, error) {
	return w.cmd.Result()
}

func (w *BoolCmdWrapper) Err() error {
	return w.cmd.Err()
}

func (w *BoolCmdWrapper) Val() bool {
	return w.cmd.Val()
}

type DurationCmdWrapper struct {
	cmd *redis.DurationCmd
}

func (w *DurationCmdWrapper) Result() (time.Duration, error) {
	return w.cmd.Result()
}

func (w *DurationCmdWrapper) Err() error {
	return w.cmd.Err()
}

func (w *DurationCmdWrapper) Val() time.Duration {
	return w.cmd.Val()
}

type StringSliceCmdWrapper struct {
	cmd *redis.StringSliceCmd
}

func (w *StringSliceCmdWrapper) Result() ([]string, error) {
	return w.cmd.Result()
}

func (w *StringSliceCmdWrapper) Err() error {
	return w.cmd.Err()
}

func (w *StringSliceCmdWrapper) Val() []string {
	return w.cmd.Val()
}

type PipelineWrapper struct {
	pipe redis.Pipeliner
}

func (p *PipelineWrapper) ZRemRangeByScore(ctx context.Context, key string, min, max string) limitter.IntCmd {
	return &IntCmdWrapper{p.pipe.ZRemRangeByScore(ctx, key, min, max)}
}

func (p *PipelineWrapper) ZCard(ctx context.Context, key string) limitter.IntCmd {
	return &IntCmdWrapper{p.pipe.ZCard(ctx, key)}
}

func (p *PipelineWrapper) ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) limitter.StringSliceCmd {
	// Handle WITHSCORES option
	var withScores bool
	for _, arg := range args {
		if str, ok := arg.(string); ok && str == "WITHSCORES" {
			withScores = true
			break
		}
	}

	if withScores {
		// Fix: Convert ZRangeWithScores (ZSliceCmd) to StringSliceCmd
		zCmd := p.pipe.ZRangeWithScores(ctx, key, start, stop)
		return &ZSliceCmdToStringSliceWrapper{zCmd}
	}
	return &StringSliceCmdWrapper{p.pipe.ZRange(ctx, key, start, stop)}
}

func (p *PipelineWrapper) Exec(ctx context.Context) ([]limitter.Cmd, error) {
	cmds, err := p.pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	// Convert redis.Cmder to limitter.Cmd
	result := make([]limitter.Cmd, len(cmds))
	for i, cmd := range cmds {
		result[i] = &CmdWrapper{cmd}
	}
	return result, nil
}

// Add this method to your PipelineWrapper struct in main.go

func (p *PipelineWrapper) ZAdd(ctx context.Context, key string, score float64, member interface{}) limitter.IntCmd {
	return &IntCmdWrapper{p.pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})}
}

// New wrapper for ZSliceCmd to StringSliceCmd conversion
type ZSliceCmdToStringSliceWrapper struct {
	cmd *redis.ZSliceCmd
}

func (w *ZSliceCmdToStringSliceWrapper) Result() ([]string, error) {
	zSlice, err := w.cmd.Result()
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(zSlice)*2)
	for _, z := range zSlice {
		result = append(result, fmt.Sprintf("%v", z.Member))
		result = append(result, fmt.Sprintf("%g", z.Score))
	}
	return result, nil
}

func (w *ZSliceCmdToStringSliceWrapper) Err() error {
	return w.cmd.Err()
}

func (w *ZSliceCmdToStringSliceWrapper) Val() []string {
	zSlice := w.cmd.Val()
	result := make([]string, 0, len(zSlice)*2)
	for _, z := range zSlice {
		result = append(result, fmt.Sprintf("%v", z.Member))
		result = append(result, fmt.Sprintf("%g", z.Score))
	}
	return result
}

type CmdWrapper struct {
	cmd redis.Cmder
}

func (c *CmdWrapper) Err() error {
	return c.cmd.Err()
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &Config{
		ServerPort:    getEnv("SERVER_PORT", "8081"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       0,
		Environment:   getEnv("ENVIRONMENT", "development"),
	}

	return config
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// NewRedisClient creates a new Redis client
func NewRedisClient(config *Config) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis")
	return &RedisClient{client: rdb}
}

// JSONResponse creates a standardized JSON response
func JSONResponse(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{
		"status":    status,
		"data":      data,
		"timestamp": time.Now().Unix(),
	})
}

// JSONError creates a standardized JSON error response
func JSONError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"status":    status,
		"error":     message,
		"timestamp": time.Now().Unix(),
	})
}

// RateLimiterAdapter adapts limitter.RateLimiter to middleware.Limiter
type RateLimiterAdapter struct {
	limiter limitter.RateLimiter
}

func (r *RateLimiterAdapter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
	// Use your limitter's IsAllowed method
	result, err := r.limiter.IsAllowed(ctx, key, limit, window)
	if err != nil {
		return false, 0, time.Time{}, err
	}
	
	return result.Allowed, result.Remaining, result.ResetTime, nil
}

// Create a Gin-compatible rate limit middleware
func rateLimitMiddleware(limiterAdapter *RateLimiterAdapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create rate limit key based on client IP
		clientIP := c.ClientIP()
		key := fmt.Sprintf("rate_limit:ip:%s", clientIP)
		
		// Check rate limit (10 requests per minute)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		
		allowed, remaining, resetTime, err := limiterAdapter.Allow(ctx, key, 10, time.Minute)
		if err != nil {
			// Log error but don't block request
			log.Printf("Rate limit error: %v", err)
			c.Next()
			return
		}
		
		// Set rate limit headers
		c.Header("X-RateLimit-Limit", "10")
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))
		
		if !allowed {
			// Rate limit exceeded
			c.Header("Retry-After", fmt.Sprintf("%d", int64(time.Until(resetTime).Seconds())))
			JSONError(c, http.StatusTooManyRequests, "Rate limit exceeded")
			c.Abort()
			return
		}
		
		// Request is allowed, proceed
		c.Next()
	}
}

// setupRoutes sets up all HTTP routes
func setupRoutes(router *gin.Engine, redisLimiter limitter.RateLimiter) {
	// Create adapter for the middleware
	adapter := &RateLimiterAdapter{limiter: redisLimiter}
	
	// Health check endpoint (no rate limiting)
	router.GET("/ping", func(c *gin.Context) {
		JSONResponse(c, http.StatusOK, gin.H{
			"message": "pong",
			"service": "rate-limiter",
		})
	})

	// API v1 routes with rate limiting
	v1 := router.Group("/api/v1")
	v1.Use(rateLimitMiddleware(adapter)) // Apply rate limiting to this group
	{
		// Status endpoint
		v1.GET("/status", func(c *gin.Context) {
			JSONResponse(c, http.StatusOK, gin.H{
				"service": "rate-limiter",
				"version": "1.0.0",
				"uptime":  time.Now().Unix(),
			})
		})

		// Test endpoint to verify rate limiting
		v1.GET("/test", func(c *gin.Context) {
			JSONResponse(c, http.StatusOK, gin.H{
				"message": "Rate limiter is working",
				"ip":      c.ClientIP(),
			})
		})
	}
}

func main() {
	// Load configuration
	config := LoadConfig()

	// Set Gin mode based on environment
	if config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Redis client
	redisClient := NewRedisClient(config)
	defer redisClient.client.Close()

	// Initialize rate limiter
	limiterConfig := &limitter.Config{
		DefaultLimit:  10,
		DefaultWindow: time.Minute,
	}

	redisLimiter := limitter.NewRedisRateLimiter(redisClient, limiterConfig)

	// Create Gin router
	router := gin.Default()

	// Add basic middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	})

	// Setup routes
	setupRoutes(router, redisLimiter)

	// Create HTTP server
	server := &http.Server{
		Addr:           ":" + config.ServerPort,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", config.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited successfully")
}