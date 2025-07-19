package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps redis client with additional methods
type Client struct {
	*redis.Client
	ctx context.Context
}

// Config holds Redis configuration options
type Config struct {
	URL          string
	MaxRetries   int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultConfig returns default Redis configuration
func DefaultConfig(url string) *Config {
	return &Config{
		URL:          url,
		MaxRetries:   3,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// ConnectRedis establishes connection to Redis
func ConnectRedis(config *Config) (*Client, error) {
	// Parse Redis URL
	opt, err := redis.ParseURL(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure connection options
	opt.MaxRetries = config.MaxRetries
	opt.PoolSize = config.PoolSize
	opt.MinIdleConns = config.MinIdleConns
	opt.DialTimeout = config.DialTimeout
	opt.ReadTimeout = config.ReadTimeout
	opt.WriteTimeout = config.WriteTimeout

	// Create Redis client
	rdb := redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &Client{
		Client: rdb,
		ctx:    ctx,
	}, nil
}

// Set stores a key-value pair with optional expiration
func (c *Client) Set(key string, value interface{}, expiration time.Duration) error {
	return c.Client.Set(c.ctx, key, value, expiration).Err()
}

// Get retrieves a value by key
func (c *Client) Get(key string) (string, error) {
	result := c.Client.Get(c.ctx, key)
	if result.Err() == redis.Nil {
		return "", fmt.Errorf("key %s not found", key)
	}
	return result.Result()
}

// Delete removes a key
func (c *Client) Delete(key string) error {
	return c.Client.Del(c.ctx, key).Err()
}

// Exists checks if a key exists
func (c *Client) Exists(key string) (bool, error) {
	result := c.Client.Exists(c.ctx, key)
	if result.Err() != nil {
		return false, result.Err()
	}
	return result.Val() > 0, nil
}

// SetExpiration sets expiration for an existing key
func (c *Client) SetExpiration(key string, expiration time.Duration) error {
	return c.Client.Expire(c.ctx, key, expiration).Err()
}

// Ping checks Redis connection health
func (c *Client) Ping() error {
	return c.Client.Ping(c.ctx).Err()
}

// Close gracefully closes Redis connection
func (c *Client) Close() error {
	return c.Client.Close()
}

// Stats returns Redis connection statistics
func (c *Client) Stats() *redis.PoolStats {
	return c.Client.PoolStats()
}
