package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosOptions Nacos 配置
type NacosOptions struct {
	Endpoints    []string      // 例如: ["127.0.0.1:8848"]
	NamespaceID  string        // 命名空间，如 public
	Group        string        // 分组，如 DEFAULT_GROUP
	Username     string        // 账号
	Password     string        // 密码
	TimeoutMs    uint64        // 请求超时 (ms)
	BeatInterval time.Duration // 心跳间隔（仅用于注释，SDK内部有默认）
	Secure       bool          // 是否启用 https
	ContextPath  string        // 自定义上下文路径，通常为空
}

// DefaultNacosOptions 默认配置
func DefaultNacosOptions() *NacosOptions {
	return &NacosOptions{
		Endpoints:   []string{"127.0.0.1:8848"},
		NamespaceID: "public",
		Group:       "DEFAULT_GROUP",
		TimeoutMs:   5000,
	}
}

// NacosRegistry Nacos 服务注册与发现实现
type NacosRegistry struct {
	mu      sync.RWMutex
	client  naming_client.INamingClient
	options *NacosOptions
}

// NewNacosRegistry 创建 Nacos 注册中心
func NewNacosRegistry(opts *NacosOptions) (*NacosRegistry, error) {
	if opts == nil {
		opts = DefaultNacosOptions()
	}

	serverConfigs := make([]constant.ServerConfig, 0, len(opts.Endpoints))
	for _, ep := range opts.Endpoints {
		// ep 格式 host:port
		host, port, err := splitHostPort(ep)
		if err != nil {
			return nil, fmt.Errorf("invalid nacos endpoint %q: %w", ep, err)
		}
		serverConfigs = append(serverConfigs, *constant.NewServerConfig(host, port, constant.WithContextPath(opts.ContextPath), constant.WithScheme(func() string {
			if opts.Secure {
				return "https"
			}
			return "http"
		}())))
	}

	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(opts.NamespaceID),
		constant.WithTimeoutMs(opts.TimeoutMs),
		constant.WithUsername(opts.Username),
		constant.WithPassword(opts.Password),
	)

	c, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos naming client: %w", err)
	}

	return &NacosRegistry{client: c, options: opts}, nil
}

// Register 注册服务实例
func (r *NacosRegistry) Register(ctx context.Context, service *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ip, port, err := splitHostPort(fmt.Sprintf("%s:%d", service.Address, service.Port))
	if err != nil {
		return err
	}

	_, err = r.client.RegisterInstance(vo.RegisterInstanceParam{
		ServiceName: service.Name,
		Ip:          ip,
		Port:        port,
		GroupName:   r.options.Group,
		Weight:      1.0,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    service.Metadata,
	})
	if err != nil {
		return fmt.Errorf("nacos register failed: %w", err)
	}
	return nil
}

// Deregister 注销服务实例
func (r *NacosRegistry) Deregister(ctx context.Context, serviceID string) error {
	// Nacos 注销需要 serviceName + ip + port，现约定 serviceID = name@ip:port
	name, ip, port, err := parseServiceID(serviceID)
	if err != nil {
		return err
	}
	_, err = r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		ServiceName: name,
		Ip:          ip,
		Port:        port,
		GroupName:   r.options.Group,
	})
	if err != nil {
		return fmt.Errorf("nacos deregister failed: %w", err)
	}
	return nil
}

// Discover 发现服务实例列表
func (r *NacosRegistry) Discover(ctx context.Context, serviceName string) ([]*ServiceInfo, error) {
	ins, err := r.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   r.options.Group,
		HealthyOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("nacos discover failed: %w", err)
	}

	services := make([]*ServiceInfo, 0, len(ins))
	for _, in := range ins {
		services = append(services, &ServiceInfo{
			ID:       fmt.Sprintf("%s@%s:%d", serviceName, in.Ip, in.Port),
			Name:     serviceName,
			Version:  "",
			Address:  in.Ip,
			Port:     int(in.Port),
			Metadata: in.Metadata,
			TTL:      0,
		})
	}
	return services, nil
}

// Watch 订阅服务变更
func (r *NacosRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInfo, error) {
	ch := make(chan []*ServiceInfo, 1)

	// 先推一次当前可用实例
	services, err := r.Discover(ctx, serviceName)
	if err == nil {
		ch <- services
	}

	// 订阅变更
	err = r.client.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   r.options.Group,
		SubscribeCallback: func(services []model.Instance, err error) {
			if err != nil {
				return
			}
			out := make([]*ServiceInfo, 0, len(services))
			for _, s := range services {
				out = append(out, &ServiceInfo{
					ID:       fmt.Sprintf("%s@%s:%d", serviceName, s.Ip, s.Port),
					Name:     serviceName,
					Address:  s.Ip,
					Port:     int(s.Port),
					Metadata: s.Metadata,
				})
			}
			select {
			case ch <- out:
			default:
			}
		},
	})
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("nacos subscribe failed: %w", err)
	}

	// 当 ctx 结束时取消订阅
	go func() {
		<-ctx.Done()
		_ = r.client.Unsubscribe(&vo.SubscribeParam{ServiceName: serviceName, GroupName: r.options.Group})
		close(ch)
	}()

	return ch, nil
}

// Close 关闭
func (r *NacosRegistry) Close() error { return nil }

// 工具函数
func splitHostPort(addr string) (string, uint64, error) {
	var host string
	var port uint64
	_, err := fmt.Sscanf(addr, "%[^:]:%d", &host, &port)
	if err != nil || host == "" || port == 0 {
		return "", 0, fmt.Errorf("invalid host:port %q", addr)
	}
	return host, port, nil
}

func parseServiceID(id string) (name, ip string, port uint64, err error) {
	// 格式: name@ip:port
	var tail string
	_, err = fmt.Sscanf(id, "%[^@]@%s", &name, &tail)
	if err != nil {
		return
	}
	ip, port, err = splitHostPort(tail)
	return
}
