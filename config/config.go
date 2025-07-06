package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the rate limiter service
type Config struct {
	// Server configuration
	Server ServerConfig `json:"server"`
	
	// Redis configuration
	Redis RedisConfig `json:"redis"`
	
	// Rate limiting configuration
	RateLimit RateLimitConfig `json:"rate_limit"`
	
	// Logging configuration
	Log LogConfig `json:"log"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string        `json:"port"`
	Host         string        `json:"host"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host         string        `json:"host"`
	Port         string        `json:"port"`
	Password     string        `json:"password"`
	DB           int           `json:"db"`
	PoolSize     int           `json:"pool_size"`
	MinIdleConns int           `json:"min_idle_conns"`
	MaxRetries   int           `json:"max_retries"`
	DialTimeout  time.Duration `json:"dial_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	PoolTimeout  time.Duration `json:"pool_timeout"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	// Default rate limit (requests per window)
	DefaultLimit int `json:"default_limit"`
	
	// Time window for rate limiting
	Window time.Duration `json:"window"`
	
	// Key prefix for Redis keys
	KeyPrefix string `json:"key_prefix"`
	
	// Whether to enable rate limiting
	Enabled bool `json:"enabled"`
	
	// Custom limits for different endpoints or users
	CustomLimits map[string]int `json:"custom_limits"`
	
	// Burst allowance
	BurstLimit int `json:"burst_limit"`
	
	// Skip rate limiting for these IPs (whitelist)
	WhitelistedIPs []string `json:"whitelisted_ips"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"` // json or text
}

// Load loads configuration from environment variables
func Load() *Config {
	config := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Redis: RedisConfig{
			Host:         getEnv("REDIS_HOST", "localhost"),
			Port:         getEnv("REDIS_PORT", "6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getIntEnv("REDIS_DB", 0),
			PoolSize:     getIntEnv("REDIS_POOL_SIZE", 10),
			MinIdleConns: getIntEnv("REDIS_MIN_IDLE_CONNS", 5),
			MaxRetries:   getIntEnv("REDIS_MAX_RETRIES", 3),
			DialTimeout:  getDurationEnv("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getDurationEnv("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getDurationEnv("REDIS_WRITE_TIMEOUT", 3*time.Second),
			PoolTimeout:  getDurationEnv("REDIS_POOL_TIMEOUT", 4*time.Second),
		},
		RateLimit: RateLimitConfig{
			DefaultLimit:   getIntEnv("RATE_LIMIT_DEFAULT", 100),
			Window:         getDurationEnv("RATE_LIMIT_WINDOW", 1*time.Hour),
			KeyPrefix:      getEnv("RATE_LIMIT_KEY_PREFIX", "rate_limit:"),
			Enabled:        getBoolEnv("RATE_LIMIT_ENABLED", true),
			CustomLimits:   parseCustomLimits(),
			BurstLimit:     getIntEnv("RATE_LIMIT_BURST", 10),
			WhitelistedIPs: parseWhitelistedIPs(),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	
	return config
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}
	
	// Validate Redis config
	if c.Redis.Host == "" {
		return fmt.Errorf("redis host cannot be empty")
	}
	
	if c.Redis.Port == "" {
		return fmt.Errorf("redis port cannot be empty")
	}
	
	// Validate rate limit config
	if c.RateLimit.DefaultLimit <= 0 {
		return fmt.Errorf("default rate limit must be greater than 0")
	}
	
	if c.RateLimit.Window <= 0 {
		return fmt.Errorf("rate limit window must be greater than 0")
	}
	
	// Validate log config
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	
	if !validLogLevels[c.Log.Level] {
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}
	
	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	
	if !validLogFormats[c.Log.Format] {
		return fmt.Errorf("invalid log format: %s", c.Log.Format)
	}
	
	return nil
}

// GetRedisAddr returns the Redis address in host:port format
func (c *Config) GetRedisAddr() string {
	return c.Redis.Host + ":" + c.Redis.Port
}

// GetServerAddr returns the server address in host:port format
func (c *Config) GetServerAddr() string {
	return c.Server.Host + ":" + c.Server.Port
}

// Helper functions for environment variable parsing

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: Invalid integer value for %s: %s, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		log.Printf("Warning: Invalid boolean value for %s: %s, using default: %t", key, value, defaultValue)
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		log.Printf("Warning: Invalid duration value for %s: %s, using default: %v", key, value, defaultValue)
	}
	return defaultValue
}

func parseCustomLimits() map[string]int {
	customLimits := make(map[string]int)
	
	// Parse custom limits from environment variables
	// Format: CUSTOM_LIMIT_ENDPOINT_NAME=limit_value
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "CUSTOM_LIMIT_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "CUSTOM_LIMIT_")
				key = strings.ToLower(strings.ReplaceAll(key, "_", "/"))
				if limit, err := strconv.Atoi(parts[1]); err == nil {
					customLimits[key] = limit
				}
			}
		}
	}
	
	return customLimits
}

func parseWhitelistedIPs() []string {
	whitelistStr := getEnv("RATE_LIMIT_WHITELIST", "")
	if whitelistStr == "" {
		return []string{}
	}
	
	// Split by comma and trim whitespace
	ips := strings.Split(whitelistStr, ",")
	var result []string
	for _, ip := range ips {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}

// LoadFromFile loads configuration from a JSON file (optional)
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// String returns a string representation of the config (without sensitive data)
func (c *Config) String() string {
	redisConfig := c.Redis
	redisConfig.Password = "[REDACTED]"
	
	return fmt.Sprintf("Config{Server: %+v, Redis: %+v, RateLimit: %+v, Log: %+v}", 
		c.Server, redisConfig, c.RateLimit, c.Log)
}