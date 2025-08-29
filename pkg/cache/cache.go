package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
)

// CacheConfig 缓存配置
type CacheConfig struct {
	Type        string        `yaml:"type" mapstructure:"type"` // redis, memory
	Redis       RedisConfig   `yaml:"redis" mapstructure:"redis"`
	DefaultTTL  time.Duration `yaml:"default_ttl" mapstructure:"default_ttl"`
	MaxSize     int           `yaml:"max_size" mapstructure:"max_size"`         // 内存缓存最大大小
	EnableStats bool          `yaml:"enable_stats" mapstructure:"enable_stats"` // 是否启用统计
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string        `yaml:"addr" mapstructure:"addr"`
	Password     string        `yaml:"password" mapstructure:"password"`
	DB           int           `yaml:"db" mapstructure:"db"`
	PoolSize     int           `yaml:"pool_size" mapstructure:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout" mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" mapstructure:"write_timeout"`
}

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	GetStats() CacheStats
	Close() error
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	HitRate   float64 `json:"hit_rate"`
	TotalOps  int64   `json:"total_ops"`
	TotalKeys int64   `json:"total_keys"`
	Memory    int64   `json:"memory_bytes"`
}

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client
	config *CacheConfig
	logger logger.Logger
	stats  CacheStats
}

// NewRedisCache 创建Redis缓存
func NewRedisCache(cfg *config.Config) (*RedisCache, error) {
	cacheConfig := &CacheConfig{
		Type:       cfg.GetString("cache.type", "redis"),
		DefaultTTL: cfg.GetDuration("cache.default_ttl", time.Hour),
		Redis: RedisConfig{
			Addr:         cfg.GetString("cache.redis.addr", "localhost:6379"),
			Password:     cfg.GetString("cache.redis.password", ""),
			DB:           cfg.GetInt("cache.redis.db", 0),
			PoolSize:     cfg.GetInt("cache.redis.pool_size", 10),
			MinIdleConns: cfg.GetInt("cache.redis.min_idle_conns", 5),
			DialTimeout:  cfg.GetDuration("cache.redis.dial_timeout", 5*time.Second),
			ReadTimeout:  cfg.GetDuration("cache.redis.read_timeout", 3*time.Second),
			WriteTimeout: cfg.GetDuration("cache.redis.write_timeout", 3*time.Second),
		},
		EnableStats: cfg.GetBool("cache.enable_stats", true),
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cacheConfig.Redis.Addr,
		Password:     cacheConfig.Redis.Password,
		DB:           cacheConfig.Redis.DB,
		PoolSize:     cacheConfig.Redis.PoolSize,
		MinIdleConns: cacheConfig.Redis.MinIdleConns,
		DialTimeout:  cacheConfig.Redis.DialTimeout,
		ReadTimeout:  cacheConfig.Redis.ReadTimeout,
		WriteTimeout: cacheConfig.Redis.WriteTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: rdb,
		config: cacheConfig,
		logger: logger.GetGlobalLogger(),
	}, nil
}

// Get 获取缓存
func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			if r.config.EnableStats {
				r.stats.Misses++
				r.stats.TotalOps++
			}
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get cache: %w", err)
	}

	if r.config.EnableStats {
		r.stats.Hits++
		r.stats.TotalOps++
		r.updateHitRate()
	}

	return val, nil
}

// Set 设置缓存
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = r.config.DefaultTTL
	}

	err := r.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	if r.config.EnableStats {
		r.stats.TotalOps++
	}

	return nil
}

// Delete 删除缓存
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}

	if r.config.EnableStats {
		r.stats.TotalOps++
	}

	return nil
}

// Exists 检查key是否存在
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	if r.config.EnableStats {
		r.stats.TotalOps++
	}

	return count > 0, nil
}

// TTL 获取key的过期时间
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}

	return ttl, nil
}

// Increment 递增计数器
func (r *RedisCache) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	val, err := r.client.IncrBy(ctx, key, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment: %w", err)
	}

	if r.config.EnableStats {
		r.stats.TotalOps++
	}

	return val, nil
}

// GetStats 获取统计信息
func (r *RedisCache) GetStats() CacheStats {
	// 更新Redis内存使用情况
	if r.config.EnableStats {
		if info, err := r.client.Info(context.Background(), "memory").Result(); err == nil {
			// 简化处理，实际应解析Redis INFO命令的输出
			r.stats.Memory = int64(len(info))
		}
	}

	return r.stats
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// updateHitRate 更新命中率
func (r *RedisCache) updateHitRate() {
	if r.stats.TotalOps > 0 {
		r.stats.HitRate = float64(r.stats.Hits) / float64(r.stats.TotalOps)
	}
}

// CacheManager 缓存管理器，支持多层缓存
type CacheManager struct {
	primary   Cache
	secondary Cache
	logger    logger.Logger
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(primary, secondary Cache) *CacheManager {
	return &CacheManager{
		primary:   primary,
		secondary: secondary,
		logger:    logger.GetGlobalLogger(),
	}
}

// GetWithFallback 多层缓存获取
func (c *CacheManager) GetWithFallback(ctx context.Context, key string) ([]byte, error) {
	// 先从主缓存获取
	if data, err := c.primary.Get(ctx, key); err == nil {
		return data, nil
	}

	// 主缓存失败，从备用缓存获取
	if c.secondary != nil {
		if data, err := c.secondary.Get(ctx, key); err == nil {
			// 同步到主缓存
			c.primary.Set(ctx, key, data, 0)
			return data, nil
		}
	}

	return nil, fmt.Errorf("cache miss for key: %s", key)
}

// TypedCache 类型化缓存包装器
type TypedCache[T any] struct {
	cache Cache
}

// NewTypedCache 创建类型化缓存
func NewTypedCache[T any](cache Cache) *TypedCache[T] {
	return &TypedCache[T]{cache: cache}
}

// Get 获取类型化数据
func (t *TypedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var result T

	data, err := t.cache.Get(ctx, key)
	if err != nil {
		return result, err
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return result, nil
}

// Set 设置类型化数据
func (t *TypedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	return t.cache.Set(ctx, key, data, ttl)
}
