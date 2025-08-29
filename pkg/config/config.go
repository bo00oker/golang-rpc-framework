package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 配置管理器
type Config struct {
	viper *viper.Viper
}

// AppConfig 应用配置
type AppConfig struct {
	Server ServerConfig `mapstructure:"server"`
	Client ClientConfig `mapstructure:"client"`
	Etcd   EtcdConfig   `mapstructure:"etcd"`
	Nacos  NacosConfig  `mapstructure:"nacos"`
	NATS   NATSConfig   `mapstructure:"nats"`
	Log    LogConfig    `mapstructure:"log"`
	Trace  TraceConfig  `mapstructure:"trace"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	// 基础配置
	Address        string `mapstructure:"address"`
	Port           int    `mapstructure:"port"`
	MaxRecvMsgSize int    `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize int    `mapstructure:"max_send_msg_size"`

	// 高并发优化配置
	MaxConcurrentRequests int           `mapstructure:"max_concurrent_requests"`
	RequestTimeout        time.Duration `mapstructure:"request_timeout"`
	MaxConnections        int           `mapstructure:"max_connections"`
	ConnectionTimeout     time.Duration `mapstructure:"connection_timeout"`
	KeepAliveTime         time.Duration `mapstructure:"keep_alive_time"`
	KeepAliveTimeout      time.Duration `mapstructure:"keep_alive_timeout"`
	RateLimit             int           `mapstructure:"rate_limit"`
	EnableMetrics         bool          `mapstructure:"enable_metrics"`

	// 内存池配置
	EnableMemoryPool  bool `mapstructure:"enable_memory_pool"`
	MemoryPoolSize    int  `mapstructure:"memory_pool_size"`
	MemoryPoolMaxSize int  `mapstructure:"memory_pool_max_size"`

	// 异步处理配置
	EnableAsync      bool `mapstructure:"enable_async"`
	AsyncWorkerCount int  `mapstructure:"async_worker_count"`
	AsyncQueueSize   int  `mapstructure:"async_queue_size"`

	// 健康检查配置
	EnableHealthCheck   bool          `mapstructure:"enable_health_check"`
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	HealthCheckTimeout  time.Duration `mapstructure:"health_check_timeout"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Timeout         time.Duration `mapstructure:"timeout"`
	KeepAlive       time.Duration `mapstructure:"keep_alive"`
	MaxRecvMsgSize  int           `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize  int           `mapstructure:"max_send_msg_size"`
	RetryAttempts   int           `mapstructure:"retry_attempts"`
	RetryDelay      time.Duration `mapstructure:"retry_delay"`
	LoadBalanceType string        `mapstructure:"load_balance_type"`

	// 高并发优化配置
	MaxConnections          int           `mapstructure:"max_connections"`
	MaxIdleConns            int           `mapstructure:"max_idle_conns"`
	ConnTimeout             time.Duration `mapstructure:"conn_timeout"`
	IdleTimeout             time.Duration `mapstructure:"idle_timeout"`
	MaxRetries              int           `mapstructure:"max_retries"`
	RetryBackoff            time.Duration `mapstructure:"retry_backoff"`
	CircuitBreakerThreshold int           `mapstructure:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `mapstructure:"circuit_breaker_timeout"`

	// 缓存配置
	EnableCache  bool          `mapstructure:"enable_cache"`
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
	CacheMaxSize int           `mapstructure:"cache_max_size"`

	// 异步处理配置
	EnableAsync      bool `mapstructure:"enable_async"`
	AsyncWorkerCount int  `mapstructure:"async_worker_count"`
	AsyncQueueSize   int  `mapstructure:"async_queue_size"`
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	Endpoints []string      `mapstructure:"endpoints"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Username  string        `mapstructure:"username"`
	Password  string        `mapstructure:"password"`
}

// NacosConfig Nacos配置
type NacosConfig struct {
	ServerAddr string        `mapstructure:"server_addr"`
	Namespace  string        `mapstructure:"namespace"`
	Group      string        `mapstructure:"group"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Username   string        `mapstructure:"username"`
	Password   string        `mapstructure:"password"`
}

// NATSConfig NATS配置
type NATSConfig struct {
	URL      string        `mapstructure:"url"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// TraceConfig 追踪配置
type TraceConfig struct {
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
	EnableConsole  bool    `mapstructure:"enable_console"`
}

// New 创建新的配置管理器
func New() *Config {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return &Config{viper: v}
}

// LoadFromFile 从文件加载配置
func (c *Config) LoadFromFile(filename string) error {
	c.viper.SetConfigFile(filename)
	if err := c.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 加载环境变量覆盖
	c.loadFromEnv()

	return nil
}

// loadFromEnv 从环境变量加载配置
func (c *Config) loadFromEnv() {
	// 服务器配置
	if val := os.Getenv("SERVER_ADDRESS"); val != "" {
		c.viper.Set("server.address", val)
	}
	if val := os.Getenv("SERVER_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.viper.Set("server.port", port)
		}
	}
	if val := os.Getenv("SERVER_MAX_CONCURRENT_REQUESTS"); val != "" {
		if max, err := strconv.Atoi(val); err == nil {
			c.viper.Set("server.max_concurrent_requests", max)
		}
	}
	if val := os.Getenv("SERVER_RATE_LIMIT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			c.viper.Set("server.rate_limit", limit)
		}
	}
	if val := os.Getenv("SERVER_ENABLE_MEMORY_POOL"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			c.viper.Set("server.enable_memory_pool", enable)
		}
	}
	if val := os.Getenv("SERVER_ENABLE_ASYNC"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			c.viper.Set("server.enable_async", enable)
		}
	}

	// 客户端配置
	if val := os.Getenv("CLIENT_MAX_CONNECTIONS"); val != "" {
		if max, err := strconv.Atoi(val); err == nil {
			c.viper.Set("client.max_connections", max)
		}
	}
	if val := os.Getenv("CLIENT_CIRCUIT_BREAKER_THRESHOLD"); val != "" {
		if threshold, err := strconv.Atoi(val); err == nil {
			c.viper.Set("client.circuit_breaker_threshold", threshold)
		}
	}
	if val := os.Getenv("CLIENT_ENABLE_CACHE"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			c.viper.Set("client.enable_cache", enable)
		}
	}
	if val := os.Getenv("CLIENT_ENABLE_ASYNC"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			c.viper.Set("client.enable_async", enable)
		}
	}
	if val := os.Getenv("CLIENT_LOAD_BALANCE_TYPE"); val != "" {
		c.viper.Set("client.load_balance_type", val)
	}

	// Nacos配置
	if val := os.Getenv("NACOS_SERVER_ADDR"); val != "" {
		c.viper.Set("nacos.server_addr", val)
	}
	if val := os.Getenv("NACOS_NAMESPACE"); val != "" {
		c.viper.Set("nacos.namespace", val)
	}

	// 追踪配置
	if val := os.Getenv("TRACE_SERVICE_NAME"); val != "" {
		c.viper.Set("trace.service_name", val)
	}
	if val := os.Getenv("TRACE_JAEGER_ENDPOINT"); val != "" {
		c.viper.Set("trace.jaeger_endpoint", val)
	}
}

// GetAppConfig 获取应用配置
func (c *Config) GetAppConfig() (*AppConfig, error) {
	var config AppConfig
	if err := c.viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 设置默认值
	c.setDefaults(&config)

	return &config, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults(config *AppConfig) {
	// 服务器默认值
	if config.Server.Address == "" {
		config.Server.Address = ":50051"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 50051
	}
	if config.Server.MaxRecvMsgSize == 0 {
		config.Server.MaxRecvMsgSize = 4 * 1024 * 1024
	}
	if config.Server.MaxSendMsgSize == 0 {
		config.Server.MaxSendMsgSize = 4 * 1024 * 1024
	}
	if config.Server.MaxConcurrentRequests == 0 {
		config.Server.MaxConcurrentRequests = 1000
	}
	if config.Server.RequestTimeout == 0 {
		config.Server.RequestTimeout = 30 * time.Second
	}
	if config.Server.MaxConnections == 0 {
		config.Server.MaxConnections = 1000
	}
	if config.Server.ConnectionTimeout == 0 {
		config.Server.ConnectionTimeout = 5 * time.Second
	}
	if config.Server.KeepAliveTime == 0 {
		config.Server.KeepAliveTime = 30 * time.Second
	}
	if config.Server.KeepAliveTimeout == 0 {
		config.Server.KeepAliveTimeout = 5 * time.Second
	}
	if config.Server.RateLimit == 0 {
		config.Server.RateLimit = 1000
	}
	if config.Server.MemoryPoolSize == 0 {
		config.Server.MemoryPoolSize = 1000
	}
	if config.Server.MemoryPoolMaxSize == 0 {
		config.Server.MemoryPoolMaxSize = 10000
	}
	if config.Server.AsyncWorkerCount == 0 {
		config.Server.AsyncWorkerCount = 10
	}
	if config.Server.AsyncQueueSize == 0 {
		config.Server.AsyncQueueSize = 1000
	}
	if config.Server.HealthCheckInterval == 0 {
		config.Server.HealthCheckInterval = 30 * time.Second
	}
	if config.Server.HealthCheckTimeout == 0 {
		config.Server.HealthCheckTimeout = 5 * time.Second
	}

	// 客户端默认值
	if config.Client.Timeout == 0 {
		config.Client.Timeout = 30 * time.Second
	}
	if config.Client.KeepAlive == 0 {
		config.Client.KeepAlive = 30 * time.Second
	}
	if config.Client.MaxRecvMsgSize == 0 {
		config.Client.MaxRecvMsgSize = 4 * 1024 * 1024
	}
	if config.Client.MaxSendMsgSize == 0 {
		config.Client.MaxSendMsgSize = 4 * 1024 * 1024
	}
	if config.Client.RetryAttempts == 0 {
		config.Client.RetryAttempts = 3
	}
	if config.Client.RetryDelay == 0 {
		config.Client.RetryDelay = time.Second
	}
	if config.Client.LoadBalanceType == "" {
		config.Client.LoadBalanceType = "round_robin"
	}
	if config.Client.MaxConnections == 0 {
		config.Client.MaxConnections = 100
	}
	if config.Client.MaxIdleConns == 0 {
		config.Client.MaxIdleConns = 10
	}
	if config.Client.ConnTimeout == 0 {
		config.Client.ConnTimeout = 5 * time.Second
	}
	if config.Client.IdleTimeout == 0 {
		config.Client.IdleTimeout = 30 * time.Second
	}
	if config.Client.MaxRetries == 0 {
		config.Client.MaxRetries = 3
	}
	if config.Client.RetryBackoff == 0 {
		config.Client.RetryBackoff = time.Second
	}
	if config.Client.CircuitBreakerThreshold == 0 {
		config.Client.CircuitBreakerThreshold = 5
	}
	if config.Client.CircuitBreakerTimeout == 0 {
		config.Client.CircuitBreakerTimeout = 30 * time.Second
	}
	if config.Client.CacheTTL == 0 {
		config.Client.CacheTTL = 5 * time.Minute
	}
	if config.Client.CacheMaxSize == 0 {
		config.Client.CacheMaxSize = 1000
	}
	if config.Client.AsyncWorkerCount == 0 {
		config.Client.AsyncWorkerCount = 10
	}
	if config.Client.AsyncQueueSize == 0 {
		config.Client.AsyncQueueSize = 1000
	}

	// Nacos默认值
	if config.Nacos.Group == "" {
		config.Nacos.Group = "DEFAULT_GROUP"
	}
	if config.Nacos.Timeout == 0 {
		config.Nacos.Timeout = 10 * time.Second
	}

	// NATS默认值
	if config.NATS.URL == "" {
		config.NATS.URL = "nats://localhost:4222"
	}
	if config.NATS.Timeout == 0 {
		config.NATS.Timeout = 10 * time.Second
	}

	// 日志默认值
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.Format == "" {
		config.Log.Format = "json"
	}
	if config.Log.Output == "" {
		config.Log.Output = "stdout"
	}
	if config.Log.MaxSize == 0 {
		config.Log.MaxSize = 100
	}
	if config.Log.MaxBackups == 0 {
		config.Log.MaxBackups = 3
	}
	if config.Log.MaxAge == 0 {
		config.Log.MaxAge = 28
	}

	// 追踪默认值
	if config.Trace.ServiceName == "" {
		config.Trace.ServiceName = "rpc-framework"
	}
	if config.Trace.ServiceVersion == "" {
		config.Trace.ServiceVersion = "v1.0.0"
	}
	if config.Trace.Environment == "" {
		config.Trace.Environment = "development"
	}
	if config.Trace.SampleRate == 0 {
		config.Trace.SampleRate = 1.0
	}
}

// GetString 获取字符串配置
func (c *Config) GetString(key string) string {
	return c.viper.GetString(key)
}

// GetInt 获取整数配置
func (c *Config) GetInt(key string) int {
	return c.viper.GetInt(key)
}

// GetFloat64 获取浮点数配置
func (c *Config) GetFloat64(key string) float64 {
	return c.viper.GetFloat64(key)
}

// GetBool 获取布尔配置
func (c *Config) GetBool(key string) bool {
	return c.viper.GetBool(key)
}

// GetDuration 获取时间配置
func (c *Config) GetDuration(key string) time.Duration {
	return c.viper.GetDuration(key)
}

// GetStringSlice 获取字符串切片配置
func (c *Config) GetStringSlice(key string) []string {
	return c.viper.GetStringSlice(key)
}

// GetServerConfig 获取服务器配置
func (c *Config) GetServerConfig() (*ServerConfig, error) {
	config, err := c.GetAppConfig()
	if err != nil {
		return nil, err
	}
	return &config.Server, nil
}

// GetClientConfig 获取客户端配置
func (c *Config) GetClientConfig() (*ClientConfig, error) {
	config, err := c.GetAppConfig()
	if err != nil {
		return nil, err
	}
	return &config.Client, nil
}

// GetNacosConfig 获取Nacos配置
func (c *Config) GetNacosConfig() (*NacosConfig, error) {
	config, err := c.GetAppConfig()
	if err != nil {
		return nil, err
	}
	return &config.Nacos, nil
}

// GetNATSConfig 获取NATS配置
func (c *Config) GetNATSConfig() (*NATSConfig, error) {
	config, err := c.GetAppConfig()
	if err != nil {
		return nil, err
	}
	return &config.NATS, nil
}

// GetLogConfig 获取日志配置
func (c *Config) GetLogConfig() (*LogConfig, error) {
	config, err := c.GetAppConfig()
	if err != nil {
		return nil, err
	}
	return &config.Log, nil
}

// GetTraceConfig 获取追踪配置
func (c *Config) GetTraceConfig() (*TraceConfig, error) {
	config, err := c.GetAppConfig()
	if err != nil {
		return nil, err
	}
	return &config.Trace, nil
}

// SaveToFile 保存配置到文件
func (c *Config) SaveToFile(filename string) error {
	// 确保目录存在
	dir := filename[:strings.LastIndex(filename, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return c.viper.WriteConfigAs(filename)
}
