package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string        `yaml:"driver" mapstructure:"driver"`                         // mysql, postgres
	DSN             string        `yaml:"dsn" mapstructure:"dsn"`                               // 数据库连接字符串
	MaxOpenConns    int           `yaml:"max_open_conns" mapstructure:"max_open_conns"`         // 最大连接数
	MaxIdleConns    int           `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`         // 最大空闲连接数
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`   // 连接最大存活时间
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"` // 连接最大空闲时间
}

// Database 数据库管理器
type Database struct {
	db     *sql.DB
	config *DatabaseConfig
	logger logger.Logger
}

// NewDatabase 创建数据库连接
func NewDatabase(cfg *config.Config) (*Database, error) {
	dbConfig := &DatabaseConfig{
		Driver:          cfg.GetString("database.driver", "mysql"),
		DSN:             cfg.GetString("database.dsn", ""),
		MaxOpenConns:    cfg.GetInt("database.max_open_conns", 100),
		MaxIdleConns:    cfg.GetInt("database.max_idle_conns", 10),
		ConnMaxLifetime: cfg.GetDuration("database.conn_max_lifetime", time.Hour),
		ConnMaxIdleTime: cfg.GetDuration("database.conn_max_idle_time", 30*time.Minute),
	}

	if dbConfig.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	db, err := sql.Open(dbConfig.Driver, dbConfig.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dbConfig.ConnMaxIdleTime)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{
		db:     db,
		config: dbConfig,
		logger: logger.GetGlobalLogger(),
	}, nil
}

// GetDB 获取数据库连接
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Health 健康检查
func (d *Database) Health(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Stats 获取连接池统计信息
func (d *Database) Stats() sql.DBStats {
	return d.db.Stats()
}

// Transaction 事务管理器
type Transaction struct {
	tx     *sql.Tx
	logger logger.Logger
}

// BeginTransaction 开始事务
func (d *Database) BeginTransaction(ctx context.Context) (*Transaction, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &Transaction{
		tx:     tx,
		logger: d.logger,
	}, nil
}

// GetTx 获取事务
func (t *Transaction) GetTx() *sql.Tx {
	return t.tx
}

// Commit 提交事务
func (t *Transaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		t.logger.Errorf("Failed to commit transaction: %v", err)
		return err
	}
	return nil
}

// Rollback 回滚事务
func (t *Transaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		t.logger.Errorf("Failed to rollback transaction: %v", err)
		return err
	}
	return nil
}

// WithTransaction 事务助手函数
func (d *Database) WithTransaction(ctx context.Context, fn func(*Transaction) error) error {
	tx, err := d.BeginTransaction(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
