package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rpc-framework/core/internal/user/model"
	"github.com/rpc-framework/core/pkg/database"
	"github.com/rpc-framework/core/pkg/logger"
)

// mysqlUserRepository MySQL用户仓储实现
type mysqlUserRepository struct {
	db     *database.Database
	logger logger.Logger
}

// NewMySQLUserRepository 创建MySQL用户仓储
func NewMySQLUserRepository(db *database.Database) UserRepository {
	return &mysqlUserRepository{
		db:     db,
		logger: logger.GetGlobalLogger(),
	}
}

// Create 创建用户
func (r *mysqlUserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		INSERT INTO users (username, email, phone, age, address, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := r.db.GetDB().ExecContext(ctx, query,
		user.Username, user.Email, user.Phone, user.Age, user.Address, now, now)
	if err != nil {
		r.logger.Errorf("Failed to create user: %v", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	createdUser := &model.User{
		ID:        id,
		Username:  user.Username,
		Email:     user.Email,
		Phone:     user.Phone,
		Age:       user.Age,
		Address:   user.Address,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.logger.Infof("User created successfully: ID=%d, Username=%s", id, user.Username)
	return createdUser, nil
}

// GetByID 根据ID获取用户
func (r *mysqlUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, username, email, phone, age, address, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	var user model.User
	err := r.db.GetDB().QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Phone,
		&user.Age, &user.Address, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		r.logger.Errorf("Failed to get user by ID %d: %v", id, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (r *mysqlUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `
		SELECT id, username, email, phone, age, address, created_at, updated_at
		FROM users
		WHERE username = ?
	`

	var user model.User
	err := r.db.GetDB().QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Phone,
		&user.Age, &user.Address, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		r.logger.Errorf("Failed to get user by username %s: %v", username, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// Update 更新用户
func (r *mysqlUserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		UPDATE users 
		SET username = ?, email = ?, phone = ?, age = ?, address = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	result, err := r.db.GetDB().ExecContext(ctx, query,
		user.Username, user.Email, user.Phone, user.Age, user.Address, now, user.ID)
	if err != nil {
		r.logger.Errorf("Failed to update user ID %d: %v", user.ID, err)
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrUserNotFound
	}

	user.UpdatedAt = now
	r.logger.Infof("User updated successfully: ID=%d", user.ID)
	return user, nil
}

// Delete 删除用户
func (r *mysqlUserRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM users WHERE id = ?`

	result, err := r.db.GetDB().ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Errorf("Failed to delete user ID %d: %v", id, err)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	r.logger.Infof("User deleted successfully: ID=%d", id)
	return nil
}

// List 查询用户列表
func (r *mysqlUserRepository) List(ctx context.Context, req *model.ListUsersRequest) (*model.ListUsersResponse, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if req.Keyword != "" {
		conditions = append(conditions, "(username LIKE ? OR email LIKE ?)")
		keyword := "%" + req.Keyword + "%"
		args = append(args, keyword, keyword)
	}

	// 构建WHERE子句
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	var total int32
	err := r.db.GetDB().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		r.logger.Errorf("Failed to count users: %v", err)
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// 查询列表
	offset := (req.Page - 1) * req.PageSize
	listQuery := fmt.Sprintf(`
		SELECT id, username, email, phone, age, address, created_at, updated_at
		FROM users %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	listArgs := append(args, req.PageSize, offset)
	rows, err := r.db.GetDB().QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		r.logger.Errorf("Failed to list users: %v", err)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var user model.User
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.Phone,
			&user.Age, &user.Address, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			r.logger.Errorf("Failed to scan user: %v", err)
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate users: %w", err)
	}

	response := &model.ListUsersResponse{
		Users:    users,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	r.logger.Infof("Listed users successfully: total=%d, page=%d", total, req.Page)
	return response, nil
}

// CreateUsersTable 创建用户表
func CreateUsersTable(db *database.Database) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(50) NOT NULL UNIQUE,
		email VARCHAR(100) NOT NULL UNIQUE,
		phone VARCHAR(20) NOT NULL,
		age INT NOT NULL DEFAULT 0,
		address TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_username (username),
		INDEX idx_email (email),
		INDEX idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	_, err := db.GetDB().Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	return nil
}

// PostgreSQL用户仓储实现
type postgresUserRepository struct {
	db     *database.Database
	logger logger.Logger
}

// NewPostgresUserRepository 创建PostgreSQL用户仓储
func NewPostgresUserRepository(db *database.Database) UserRepository {
	return &postgresUserRepository{
		db:     db,
		logger: logger.GetGlobalLogger(),
	}
}

// Create 创建用户（PostgreSQL版本）
func (r *postgresUserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	query := `
		INSERT INTO users (username, email, phone, age, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	now := time.Now()
	var id int64
	err := r.db.GetDB().QueryRowContext(ctx, query,
		user.Username, user.Email, user.Phone, user.Age, user.Address, now, now).Scan(&id)
	if err != nil {
		r.logger.Errorf("Failed to create user: %v", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	createdUser := &model.User{
		ID:        id,
		Username:  user.Username,
		Email:     user.Email,
		Phone:     user.Phone,
		Age:       user.Age,
		Address:   user.Address,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.logger.Infof("User created successfully: ID=%d, Username=%s", id, user.Username)
	return createdUser, nil
}

// GetByID PostgreSQL版本的其他方法实现类似，主要是参数占位符不同($1, $2 vs ?)
func (r *postgresUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query := `
		SELECT id, username, email, phone, age, address, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user model.User
	err := r.db.GetDB().QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Phone,
		&user.Age, &user.Address, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		r.logger.Errorf("Failed to get user by ID %d: %v", id, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *postgresUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	// 类似实现，使用$1参数
	return nil, fmt.Errorf("not implemented")
}

func (r *postgresUserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	// 类似实现，使用$1, $2等参数
	return nil, fmt.Errorf("not implemented")
}

func (r *postgresUserRepository) Delete(ctx context.Context, id int64) error {
	// 类似实现，使用$1参数
	return fmt.Errorf("not implemented")
}

func (r *postgresUserRepository) List(ctx context.Context, req *model.ListUsersRequest) (*model.ListUsersResponse, error) {
	// 类似实现，使用$1, $2等参数
	return nil, fmt.Errorf("not implemented")
}

// CreateUsersTablePostgres 创建PostgreSQL用户表
func CreateUsersTablePostgres(db *database.Database) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL UNIQUE,
		email VARCHAR(100) NOT NULL UNIQUE,
		phone VARCHAR(20) NOT NULL,
		age INTEGER NOT NULL DEFAULT 0,
		address TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
	`

	_, err := db.GetDB().Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	return nil
}
