package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Message 消息结构
type Message struct {
	ID        string            `json:"id"`
	Subject   string            `json:"subject"`
	Data      []byte            `json:"data"`
	Headers   map[string]string `json:"headers"`
	Timestamp time.Time         `json:"timestamp"`
	ReplyTo   string            `json:"reply_to,omitempty"`
}

// MessageHandler 消息处理器
type MessageHandler func(msg *Message) error

// Publisher 发布者接口
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
	PublishWithReply(ctx context.Context, subject string, data []byte, timeout time.Duration) (*Message, error)
	Close() error
}

// Subscriber 订阅者接口
type Subscriber interface {
	Subscribe(subject string, handler MessageHandler) error
	QueueSubscribe(subject, queue string, handler MessageHandler) error
	Unsubscribe(subject string) error
	Close() error
}

// MessageQueue 消息队列接口
type MessageQueue interface {
	Publisher
	Subscriber
}

// NatsMessageQueue NATS消息队列实现
type NatsMessageQueue struct {
	mu            sync.RWMutex
	conn          *nats.Conn
	options       *NatsOptions
	subscriptions map[string]*nats.Subscription
}

// NatsOptions NATS配置选项
type NatsOptions struct {
	URL              string        // NATS服务器URL
	Name             string        // 连接名称
	Timeout          time.Duration // 连接超时
	ReconnectWait    time.Duration // 重连等待时间
	MaxReconnects    int           // 最大重连次数
	PingInterval     time.Duration // Ping间隔
	MaxPingsOut      int           // 最大未响应Ping数
	ReconnectBufSize int           // 重连缓冲区大小
}

// DefaultNatsOptions 默认NATS配置
func DefaultNatsOptions() *NatsOptions {
	return &NatsOptions{
		URL:              nats.DefaultURL,
		Name:             "rpc-framework",
		Timeout:          5 * time.Second,
		ReconnectWait:    2 * time.Second,
		MaxReconnects:    10,
		PingInterval:     2 * time.Minute,
		MaxPingsOut:      2,
		ReconnectBufSize: 8 * 1024 * 1024, // 8MB
	}
}

// NewNatsMessageQueue 创建NATS消息队列
func NewNatsMessageQueue(opts *NatsOptions) (*NatsMessageQueue, error) {
	if opts == nil {
		opts = DefaultNatsOptions()
	}

	// 构建NATS选项
	natsOpts := []nats.Option{
		nats.Name(opts.Name),
		nats.Timeout(opts.Timeout),
		nats.ReconnectWait(opts.ReconnectWait),
		nats.MaxReconnects(opts.MaxReconnects),
		nats.PingInterval(opts.PingInterval),
		nats.MaxPingsOutstanding(opts.MaxPingsOut),
		nats.ReconnectBufSize(opts.ReconnectBufSize),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			// 处理断开连接错误
			fmt.Printf("NATS disconnected: %v\n", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			// 处理重连
			fmt.Printf("NATS reconnected to %v\n", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			// 处理连接关闭
			fmt.Printf("NATS connection closed\n")
		}),
	}

	// 连接到NATS
	conn, err := nats.Connect(opts.URL, natsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &NatsMessageQueue{
		conn:          conn,
		options:       opts,
		subscriptions: make(map[string]*nats.Subscription),
	}, nil
}

// Publish 发布消息
func (mq *NatsMessageQueue) Publish(ctx context.Context, subject string, data []byte) error {
	msg := &Message{
		ID:        generateMessageID(),
		Subject:   subject,
		Data:      data,
		Timestamp: time.Now(),
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return mq.conn.Publish(subject, msgData)
}

// PublishWithReply 发布消息并等待回复
func (mq *NatsMessageQueue) PublishWithReply(ctx context.Context, subject string, data []byte, timeout time.Duration) (*Message, error) {
	msg := &Message{
		ID:        generateMessageID(),
		Subject:   subject,
		Data:      data,
		Timestamp: time.Now(),
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	reply, err := mq.conn.Request(subject, msgData, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to request: %w", err)
	}

	var replyMsg Message
	if err := json.Unmarshal(reply.Data, &replyMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reply: %w", err)
	}

	return &replyMsg, nil
}

// Subscribe 订阅消息
func (mq *NatsMessageQueue) Subscribe(subject string, handler MessageHandler) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	// 检查是否已经订阅
	if _, exists := mq.subscriptions[subject]; exists {
		return fmt.Errorf("already subscribed to subject: %s", subject)
	}

	// 创建订阅
	sub, err := mq.conn.Subscribe(subject, func(natsMsg *nats.Msg) {
		var msg Message
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			return // 跳过无效消息
		}

		// 设置回复地址
		if natsMsg.Reply != "" {
			msg.ReplyTo = natsMsg.Reply
		}

		// 处理消息
		if err := handler(&msg); err != nil {
			// 处理错误，可以记录日志
			fmt.Printf("Message handler error: %v\n", err)
		}

		// 如果有回复地址，发送确认
		if natsMsg.Reply != "" {
			replyMsg := &Message{
				ID:        generateMessageID(),
				Subject:   natsMsg.Reply,
				Data:      []byte("ACK"),
				Timestamp: time.Now(),
			}
			replyData, _ := json.Marshal(replyMsg)
			natsMsg.Respond(replyData)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	mq.subscriptions[subject] = sub
	return nil
}

// QueueSubscribe 队列订阅
func (mq *NatsMessageQueue) QueueSubscribe(subject, queue string, handler MessageHandler) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	key := fmt.Sprintf("%s:%s", subject, queue)
	// 检查是否已经订阅
	if _, exists := mq.subscriptions[key]; exists {
		return fmt.Errorf("already subscribed to subject: %s with queue: %s", subject, queue)
	}

	// 创建队列订阅
	sub, err := mq.conn.QueueSubscribe(subject, queue, func(natsMsg *nats.Msg) {
		var msg Message
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			return // 跳过无效消息
		}

		// 设置回复地址
		if natsMsg.Reply != "" {
			msg.ReplyTo = natsMsg.Reply
		}

		// 处理消息
		if err := handler(&msg); err != nil {
			// 处理错误，可以记录日志
			fmt.Printf("Message handler error: %v\n", err)
		}

		// 如果有回复地址，发送确认
		if natsMsg.Reply != "" {
			replyMsg := &Message{
				ID:        generateMessageID(),
				Subject:   natsMsg.Reply,
				Data:      []byte("ACK"),
				Timestamp: time.Now(),
			}
			replyData, _ := json.Marshal(replyMsg)
			natsMsg.Respond(replyData)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to queue subscribe: %w", err)
	}

	mq.subscriptions[key] = sub
	return nil
}

// Unsubscribe 取消订阅
func (mq *NatsMessageQueue) Unsubscribe(subject string) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if sub, exists := mq.subscriptions[subject]; exists {
		err := sub.Unsubscribe()
		delete(mq.subscriptions, subject)
		return err
	}

	return fmt.Errorf("no subscription found for subject: %s", subject)
}

// Close 关闭消息队列
func (mq *NatsMessageQueue) Close() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	// 取消所有订阅
	for subject, sub := range mq.subscriptions {
		sub.Unsubscribe()
		delete(mq.subscriptions, subject)
	}

	// 关闭连接
	mq.conn.Close()
	return nil
}

// IsConnected 检查连接状态
func (mq *NatsMessageQueue) IsConnected() bool { return mq.conn.IsConnected() }

// GetStats 获取统计信息
func (mq *NatsMessageQueue) GetStats() nats.Statistics { return mq.conn.Stats() }

// generateMessageID 生成消息ID
func generateMessageID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

// StreamPublisher 流发布者（JetStream支持）
type StreamPublisher struct{ js nats.JetStreamContext }

// NewStreamPublisher 创建流发布者
func NewStreamPublisher(conn *nats.Conn) (*StreamPublisher, error) {
	js, err := conn.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}
	return &StreamPublisher{js: js}, nil
}

// PublishAsync 异步发布到流
func (sp *StreamPublisher) PublishAsync(subject string, data []byte) (nats.PubAckFuture, error) {
	msg := &Message{ID: generateMessageID(), Subject: subject, Data: data, Timestamp: time.Now()}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	fut, perr := sp.js.PublishAsync(subject, msgData)
	if perr != nil {
		return nil, perr
	}
	return fut, nil
}

// PublishSync 同步发布到流
func (sp *StreamPublisher) PublishSync(subject string, data []byte) (*nats.PubAck, error) {
	msg := &Message{ID: generateMessageID(), Subject: subject, Data: data, Timestamp: time.Now()}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	return sp.js.Publish(subject, msgData)
}
