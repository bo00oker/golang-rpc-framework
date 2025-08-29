package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ServiceInfo 服务信息
type ServiceInfo struct {
	ID       string            `json:"id"`       // 服务实例ID
	Name     string            `json:"name"`     // 服务名称
	Version  string            `json:"version"`  // 服务版本
	Address  string            `json:"address"`  // 服务地址
	Port     int               `json:"port"`     // 服务端口
	Metadata map[string]string `json:"metadata"` // 元数据
	TTL      int64             `json:"ttl"`      // 生存时间
}

// Registry 服务注册发现接口
type Registry interface {
	Register(ctx context.Context, service *ServiceInfo) error
	Deregister(ctx context.Context, serviceID string) error
	Discover(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInfo, error)
	Close() error
}

// EtcdRegistry etcd服务注册发现实现
type EtcdRegistry struct {
	mu       sync.RWMutex
	client   *clientv3.Client
	options  *EtcdOptions
	leases   map[string]clientv3.LeaseID   // 服务ID到租约ID的映射
	watchers map[string]context.CancelFunc // 监听器取消函数
}

// EtcdOptions etcd配置选项
type EtcdOptions struct {
	Endpoints   []string      // etcd端点
	Timeout     time.Duration // 连接超时
	KeyPrefix   string        // 键前缀
	DialTimeout time.Duration // 拨号超时
	Username    string        // 用户名
	Password    string        // 密码
}

// DefaultEtcdOptions 默认etcd配置
func DefaultEtcdOptions() *EtcdOptions {
	return &EtcdOptions{
		Endpoints:   []string{"localhost:2379"},
		Timeout:     5 * time.Second,
		KeyPrefix:   "/services",
		DialTimeout: 5 * time.Second,
	}
}

// NewEtcdRegistry 创建etcd注册中心
func NewEtcdRegistry(opts *EtcdOptions) (*EtcdRegistry, error) {
	if opts == nil {
		opts = DefaultEtcdOptions()
	}

	config := clientv3.Config{
		Endpoints:   opts.Endpoints,
		DialTimeout: opts.DialTimeout,
	}

	if opts.Username != "" && opts.Password != "" {
		config.Username = opts.Username
		config.Password = opts.Password
	}

	client, err := clientv3.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &EtcdRegistry{
		client:   client,
		options:  opts,
		leases:   make(map[string]clientv3.LeaseID),
		watchers: make(map[string]context.CancelFunc),
	}, nil
}

// Register 注册服务
func (r *EtcdRegistry) Register(ctx context.Context, service *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 序列化服务信息
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service info: %w", err)
	}

	// 创建租约
	leaseResp, err := r.client.Grant(ctx, service.TTL)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	// 构建服务键
	key := r.buildServiceKey(service.Name, service.ID)

	// 注册服务
	_, err = r.client.Put(ctx, key, string(data), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 保存租约ID
	r.leases[service.ID] = leaseResp.ID

	// 启动租约续期
	go r.keepAlive(service.ID, leaseResp.ID)

	return nil
}

// Deregister 注销服务
func (r *EtcdRegistry) Deregister(ctx context.Context, serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 撤销租约
	if leaseID, exists := r.leases[serviceID]; exists {
		_, err := r.client.Revoke(ctx, leaseID)
		if err != nil {
			return fmt.Errorf("failed to revoke lease: %w", err)
		}
		delete(r.leases, serviceID)
	}

	return nil
}

// Discover 发现服务
func (r *EtcdRegistry) Discover(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	key := r.buildServicePrefix(serviceName)

	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	var services []*ServiceInfo
	for _, kv := range resp.Kvs {
		var service ServiceInfo
		if err := json.Unmarshal(kv.Value, &service); err != nil {
			continue // 跳过无效的服务信息
		}
		services = append(services, &service)
	}

	return services, nil
}

// Watch 监听服务变化
func (r *EtcdRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInfo, error) {
	key := r.buildServicePrefix(serviceName)
	ch := make(chan []*ServiceInfo, 1)

	// 首次获取服务列表
	services, err := r.Discover(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	ch <- services

	// 启动监听
	watchCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.watchers[serviceName] = cancel
	r.mu.Unlock()

	go func() {
		defer close(ch)
		defer func() {
			r.mu.Lock()
			delete(r.watchers, serviceName)
			r.mu.Unlock()
		}()

		watchCh := r.client.Watch(watchCtx, key, clientv3.WithPrefix())
		for {
			select {
			case <-watchCtx.Done():
				return
			case watchResp := <-watchCh:
				if watchResp.Err() != nil {
					return
				}

				// 重新获取服务列表
				services, err := r.Discover(context.Background(), serviceName)
				if err != nil {
					continue
				}

				select {
				case ch <- services:
				case <-watchCtx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// Close 关闭注册中心
func (r *EtcdRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 取消所有监听器
	for _, cancel := range r.watchers {
		cancel()
	}
	r.watchers = make(map[string]context.CancelFunc)

	// 撤销所有租约
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for serviceID, leaseID := range r.leases {
		r.client.Revoke(ctx, leaseID)
		delete(r.leases, serviceID)
	}

	return r.client.Close()
}

// keepAlive 保持租约活跃
func (r *EtcdRegistry) keepAlive(serviceID string, leaseID clientv3.LeaseID) {
	ch, kaerr := r.client.KeepAlive(context.Background(), leaseID)
	if kaerr != nil {
		return
	}

	for ka := range ch {
		// 处理续期响应
		_ = ka
	}

	// 租约过期，清理
	r.mu.Lock()
	delete(r.leases, serviceID)
	r.mu.Unlock()
}

// buildServiceKey 构建服务键
func (r *EtcdRegistry) buildServiceKey(serviceName, serviceID string) string {
	return path.Join(r.options.KeyPrefix, serviceName, serviceID)
}

// buildServicePrefix 构建服务前缀
func (r *EtcdRegistry) buildServicePrefix(serviceName string) string {
	return path.Join(r.options.KeyPrefix, serviceName) + "/"
}

// HealthChecker 健康检查器
type HealthChecker struct {
	registry Registry
	service  *ServiceInfo
	interval time.Duration
	stopCh   chan struct{}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(registry Registry, service *ServiceInfo, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		registry: registry,
		service:  service,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start(ctx context.Context, healthCheckFunc func() bool) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopCh:
			return
		case <-ticker.C:
			if healthCheckFunc() {
				// 健康，重新注册服务
				hc.registry.Register(ctx, hc.service)
			} else {
				// 不健康，注销服务
				hc.registry.Deregister(ctx, hc.service.ID)
			}
		}
	}
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}
