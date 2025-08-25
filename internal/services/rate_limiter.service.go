package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// RateLimiter interface for rate limiting implementations
type RateLimiter interface {
	Allow(key string, limit int, window time.Duration) (bool, error)
	Reset(key string) error
}

// RateLimiterService handles rate limiting for both user and API key requests
type RateLimiterService struct {
	redisClient     *redis.Client
	inMemoryLimiter *InMemoryRateLimiter
	useRedis        bool
}

// InMemoryRateLimiter provides fallback rate limiting
type InMemoryRateLimiter struct {
	limiters map[string]*rate.Limiter
	mutex    sync.RWMutex
}

// RateLimitResult contains rate limiting information
type RateLimitResult struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
}

// NewRateLimiterService creates a new rate limiter service
func NewRateLimiterService(redisURL string) *RateLimiterService {
	service := &RateLimiterService{
		inMemoryLimiter: &InMemoryRateLimiter{
			limiters: make(map[string]*rate.Limiter),
		},
	}

	if redisURL != "" {
		// Try to connect to Redis
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.WithError(err).Warn("Failed to parse Redis URL, falling back to in-memory rate limiting")
		} else {
			rdb := redis.NewClient(opt)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := rdb.Ping(ctx).Err(); err != nil {
				log.WithError(err).Warn("Failed to connect to Redis, falling back to in-memory rate limiting")
			} else {
				service.redisClient = rdb
				service.useRedis = true
				log.Info("Connected to Redis for distributed rate limiting")
			}
		}
	}

	if !service.useRedis {
		log.Info("Using in-memory rate limiting")
	}

	return service
}

// Allow checks if a request should be allowed based on rate limits
func (r *RateLimiterService) Allow(key string, limit int, window time.Duration) (bool, error) {
	if r.useRedis {
		return r.allowRedis(key, limit, window)
	}
	return r.inMemoryLimiter.Allow(key, limit, window), nil
}

// allowRedis implements sliding window rate limiting using Redis
func (r *RateLimiterService) allowRedis(key string, limit int, window time.Duration) (bool, error) {
	ctx := context.Background()
	now := time.Now()
	windowStart := now.Add(-window)

	// Use a Lua script for atomic operations
	luaScript := `
		local key = KEYS[1]
		local window_start = tonumber(ARGV[1])
		local now = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		
		-- Remove old entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)
		
		-- Count current entries in window
		local current = redis.call('ZCARD', key)
		
		if current < limit then
			-- Add current request
			redis.call('ZADD', key, now, now)
			-- Set expiration for cleanup
			redis.call('EXPIRE', key, 3600)
			return {1, limit - current - 1}
		else
			return {0, 0}
		end
	`

	result, err := r.redisClient.Eval(ctx, luaScript, []string{key},
		windowStart.UnixNano(), now.UnixNano(), limit).Result()
	if err != nil {
		log.WithError(err).Error("Redis rate limit check failed")
		// Fallback to in-memory
		return r.inMemoryLimiter.Allow(key, limit, window), nil
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	return allowed, nil
}

// Reset removes rate limit data for a key
func (r *RateLimiterService) Reset(key string) error {
	if r.useRedis {
		ctx := context.Background()
		return r.redisClient.Del(ctx, key).Err()
	}
	r.inMemoryLimiter.Reset(key)
	return nil
}

// Allow implements in-memory rate limiting using token bucket
func (i *InMemoryRateLimiter) Allow(key string, limit int, window time.Duration) bool {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	limiter, exists := i.limiters[key]
	if !exists {
		// Create new limiter with token bucket: limit tokens per window
		limiter = rate.NewLimiter(rate.Every(window/time.Duration(limit)), limit)
		i.limiters[key] = limiter
	}

	return limiter.Allow()
}

// Reset removes a limiter for a key
func (i *InMemoryRateLimiter) Reset(key string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	delete(i.limiters, key)
}

// Cleanup removes old limiters (call periodically)
func (i *InMemoryRateLimiter) Cleanup() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	// Simple cleanup - remove all limiters
	// In production, you might want to track last access time
	i.limiters = make(map[string]*rate.Limiter)
}

// GenerateRateLimitKey creates a rate limit key for different types of requests
func GenerateRateLimitKey(userType, identifier, endpoint string) string {
	// Create a hash to avoid very long keys
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%s:%s", userType, identifier, endpoint)))
	hash := hex.EncodeToString(hasher.Sum(nil))[:16]
	return fmt.Sprintf("rate_limit:%s:%s", userType, hash)
}

// GetUserRateLimitKey creates a rate limit key for user requests
func GetUserRateLimitKey(clerkUserID, endpoint string) string {
	return GenerateRateLimitKey("user", clerkUserID, endpoint)
}

// GetAPIKeyRateLimitKey creates a rate limit key for API key requests
func GetAPIKeyRateLimitKey(apiKeyID, endpoint string) string {
	return GenerateRateLimitKey("api", apiKeyID, endpoint)
}

// GetGlobalRateLimitKey creates a rate limit key for global limits
func GetGlobalRateLimitKey(endpoint string) string {
	return GenerateRateLimitKey("global", "all", endpoint)
}

// Close closes the rate limiter service
func (r *RateLimiterService) Close() error {
	if r.redisClient != nil {
		return r.redisClient.Close()
	}
	return nil
}
