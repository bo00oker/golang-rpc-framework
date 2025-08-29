package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// HotReloadConfig 热更新配置管理器
type HotReloadConfig struct {
	mu        sync.RWMutex
	viper     *viper.Viper
	callbacks map[string][]ConfigChangeCallback
	watcher   *fsnotify.Watcher
	stopChan  chan struct{}
}

// ConfigChangeCallback 配置变更回调函数
type ConfigChangeCallback func(key string, oldValue, newValue interface{}) error

// ConfigChange 配置变更事件
type ConfigChange struct {
	Key      string      `json:"key"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
	Time     time.Time   `json:"time"`
}

// NewHotReloadConfig 创建热更新配置管理器
func NewHotReloadConfig() *HotReloadConfig {
	watcher, _ := fsnotify.NewWatcher()

	return &HotReloadConfig{
		viper:     viper.New(),
		callbacks: make(map[string][]ConfigChangeCallback),
		watcher:   watcher,
		stopChan:  make(chan struct{}),
	}
}

// LoadConfig 加载配置文件并启动监听
func (hrc *HotReloadConfig) LoadConfig(configPath string) error {
	hrc.viper.SetConfigFile(configPath)

	if err := hrc.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// 添加文件监听
	if err := hrc.watcher.Add(configPath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	// 启动监听协程
	go hrc.watchConfigChanges()

	return nil
}

// RegisterCallback 注册配置变更回调
func (hrc *HotReloadConfig) RegisterCallback(key string, callback ConfigChangeCallback) {
	hrc.mu.Lock()
	defer hrc.mu.Unlock()

	hrc.callbacks[key] = append(hrc.callbacks[key], callback)
}

// GetString 获取字符串配置
func (hrc *HotReloadConfig) GetString(key string) string {
	hrc.mu.RLock()
	defer hrc.mu.RUnlock()
	return hrc.viper.GetString(key)
}

// GetInt 获取整数配置
func (hrc *HotReloadConfig) GetInt(key string) int {
	hrc.mu.RLock()
	defer hrc.mu.RUnlock()
	return hrc.viper.GetInt(key)
}

// GetDuration 获取时间配置
func (hrc *HotReloadConfig) GetDuration(key string) time.Duration {
	hrc.mu.RLock()
	defer hrc.mu.RUnlock()
	return hrc.viper.GetDuration(key)
}

// GetBool 获取布尔配置
func (hrc *HotReloadConfig) GetBool(key string) bool {
	hrc.mu.RLock()
	defer hrc.mu.RUnlock()
	return hrc.viper.GetBool(key)
}

// watchConfigChanges 监听配置文件变更
func (hrc *HotReloadConfig) watchConfigChanges() {
	for {
		select {
		case event, ok := <-hrc.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				hrc.handleConfigChange()
			}

		case err, ok := <-hrc.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Config watcher error: %v\n", err)

		case <-hrc.stopChan:
			return
		}
	}
}

// handleConfigChange 处理配置变更
func (hrc *HotReloadConfig) handleConfigChange() {
	// 备份当前配置
	oldConfig := make(map[string]interface{})
	for _, key := range hrc.viper.AllKeys() {
		oldConfig[key] = hrc.viper.Get(key)
	}

	// 重新加载配置
	if err := hrc.viper.ReadInConfig(); err != nil {
		fmt.Printf("Failed to reload config: %v\n", err)
		return
	}

	// 检测变更并触发回调
	hrc.mu.Lock()
	defer hrc.mu.Unlock()

	for _, key := range hrc.viper.AllKeys() {
		newValue := hrc.viper.Get(key)
		oldValue := oldConfig[key]

		if fmt.Sprintf("%v", oldValue) != fmt.Sprintf("%v", newValue) {
			// 配置发生变更，触发回调
			if callbacks, exists := hrc.callbacks[key]; exists {
				for _, callback := range callbacks {
					if err := callback(key, oldValue, newValue); err != nil {
						fmt.Printf("Config callback error for key %s: %v\n", key, err)
					}
				}
			}

			// 触发通配符回调
			if callbacks, exists := hrc.callbacks["*"]; exists {
				for _, callback := range callbacks {
					if err := callback(key, oldValue, newValue); err != nil {
						fmt.Printf("Config wildcard callback error for key %s: %v\n", key, err)
					}
				}
			}
		}
	}
}

// Stop 停止配置监听
func (hrc *HotReloadConfig) Stop() {
	close(hrc.stopChan)
	if hrc.watcher != nil {
		hrc.watcher.Close()
	}
}

// ConfigManager 全局配置管理器
type ConfigManager struct {
	hotReload *HotReloadConfig
	namespace string
}

// NewConfigManager 创建配置管理器
func NewConfigManager(namespace string) *ConfigManager {
	return &ConfigManager{
		hotReload: NewHotReloadConfig(),
		namespace: namespace,
	}
}

// Initialize 初始化配置
func (cm *ConfigManager) Initialize(configPath string) error {
	return cm.hotReload.LoadConfig(configPath)
}

// RegisterServerConfigCallback 注册服务器配置变更回调
func (cm *ConfigManager) RegisterServerConfigCallback(callback func(*ServerConfig) error) {
	cm.hotReload.RegisterCallback("server.*", func(key string, oldValue, newValue interface{}) error {
		// 重新构建服务器配置
		serverConfig := &ServerConfig{
			Address:               cm.hotReload.GetString("server.address"),
			Port:                  cm.hotReload.GetInt("server.port"),
			MaxConcurrentRequests: cm.hotReload.GetInt("server.max_concurrent_requests"),
			RequestTimeout:        cm.hotReload.GetDuration("server.request_timeout"),
			RateLimit:             cm.hotReload.GetInt("server.rate_limit"),
			EnableMetrics:         cm.hotReload.GetBool("server.enable_metrics"),
			EnableMemoryPool:      cm.hotReload.GetBool("server.enable_memory_pool"),
			EnableAsync:           cm.hotReload.GetBool("server.enable_async"),
			EnableHealthCheck:     cm.hotReload.GetBool("server.enable_health_check"),
		}

		return callback(serverConfig)
	})
}

// RegisterClientConfigCallback 注册客户端配置变更回调
func (cm *ConfigManager) RegisterClientConfigCallback(callback func(*ClientConfig) error) {
	cm.hotReload.RegisterCallback("client.*", func(key string, oldValue, newValue interface{}) error {
		clientConfig := &ClientConfig{
			Timeout:                 cm.hotReload.GetDuration("client.timeout"),
			MaxConnections:          cm.hotReload.GetInt("client.max_connections"),
			MaxIdleConns:            cm.hotReload.GetInt("client.max_idle_conns"),
			MaxRetries:              cm.hotReload.GetInt("client.max_retries"),
			CircuitBreakerThreshold: cm.hotReload.GetInt("client.circuit_breaker_threshold"),
			EnableCache:             cm.hotReload.GetBool("client.enable_cache"),
			EnableAsync:             cm.hotReload.GetBool("client.enable_async"),
		}

		return callback(clientConfig)
	})
}
