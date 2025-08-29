# API 接口文档

## 概述

本文档详细描述了RPC微服务框架通过API Gateway提供的所有HTTP接口。所有接口均遵循RESTful设计规范，使用JSON格式进行数据交换。

## 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json`
- **字符编码**: UTF-8

## 通用响应格式

所有API响应都采用统一的JSON格式：

```json
{
  "code": 200,
  "message": "Success",
  "data": {
    // 具体数据内容
  }
}
```

### 状态码说明

| HTTP状态码 | 业务码 | 说明 |
|-----------|-------|------|
| 200 | 200 | 请求成功 |
| 201 | 201 | 资源创建成功 |
| 400 | 400 | 请求参数错误 |
| 401 | 401 | 未认证或认证失败 |
| 403 | 403 | 权限不足 |
| 404 | 404 | 资源不存在 |
| 409 | 409 | 资源冲突 |
| 429 | 429 | 请求频率过高 |
| 500 | 500 | 服务器内部错误 |
| 503 | 503 | 服务不可用 |

## 认证授权

### JWT Token认证

除了公开接口外，所有API都需要在请求头中携带JWT Token：

```http
Authorization: Bearer <token>
```

### 权限角色

- **admin**: 管理员，拥有所有权限
- **user**: 普通用户，可以操作自己的数据
- **guest**: 访客，只有读取权限

## 接口列表

### 1. 认证相关接口

#### 1.1 用户登录

**接口地址**: `POST /api/v1/auth/login`

**请求参数**:
```json
{
  "username": "admin",
  "password": "password"
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "Login successful",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "user": {
      "id": 1,
      "username": "admin",
      "role": "admin"
    }
  }
}
```

#### 1.2 刷新Token

**接口地址**: `POST /api/v1/auth/refresh`

**请求参数**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "Token refreshed successfully",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600
  }
}
```

#### 1.3 用户登出

**接口地址**: `POST /api/v1/auth/logout`

**请求头**: 需要Authorization

**响应示例**:
```json
{
  "code": 200,
  "message": "Logout successful",
  "data": null
}
```

### 2. 用户管理接口

#### 2.1 创建用户

**接口地址**: `POST /api/v1/users`

**请求参数**:
```json
{
  "username": "testuser",
  "email": "test@example.com",
  "phone": "13800138000",
  "age": 25
}
```

**响应示例**:
```json
{
  "code": 201,
  "message": "User created successfully",
  "data": {
    "id": 1,
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "phone": "13800138000",
      "age": 25,
      "created_at": "2024-08-29T10:00:00Z",
      "updated_at": "2024-08-29T10:00:00Z"
    }
  }
}
```

#### 2.2 获取用户列表

**接口地址**: `GET /api/v1/users`

**查询参数**:
- `page`: 页码，默认1
- `page_size`: 每页数量，默认10
- `keyword`: 搜索关键词（可选）

**示例**: `GET /api/v1/users?page=1&page_size=10&keyword=test`

**响应示例**:
```json
{
  "code": 200,
  "message": "Users retrieved successfully",
  "data": {
    "users": [
      {
        "id": 1,
        "username": "testuser",
        "email": "test@example.com",
        "phone": "13800138000",
        "age": 25,
        "created_at": "2024-08-29T10:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "page_size": 10,
      "total": 1
    }
  }
}
```

#### 2.3 获取用户详情

**接口地址**: `GET /api/v1/users/{id}`

**路径参数**:
- `id`: 用户ID

**响应示例**:
```json
{
  "code": 200,
  "message": "User retrieved successfully",
  "data": {
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "test@example.com",
      "phone": "13800138000",
      "age": 25,
      "created_at": "2024-08-29T10:00:00Z",
      "updated_at": "2024-08-29T10:00:00Z"
    }
  }
}
```

#### 2.4 更新用户信息

**接口地址**: `PUT /api/v1/users/{id}`

**权限要求**: 需要认证，只能更新自己的信息或管理员权限

**请求参数**:
```json
{
  "email": "newemail@example.com",
  "phone": "13900139000",
  "age": 26
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "User updated successfully",
  "data": {
    "user": {
      "id": 1,
      "username": "testuser",
      "email": "newemail@example.com",
      "phone": "13900139000",
      "age": 26,
      "updated_at": "2024-08-29T11:00:00Z"
    }
  }
}
```

#### 2.5 删除用户

**接口地址**: `DELETE /api/v1/users/{id}`

**权限要求**: 管理员权限

**响应示例**:
```json
{
  "code": 200,
  "message": "User deleted successfully",
  "data": null
}
```

### 3. 订单管理接口

#### 3.1 创建订单

**接口地址**: `POST /api/v1/orders`

**权限要求**: 需要认证

**请求参数**:
```json
{
  "user_id": 1,
  "amount": 299.99,
  "description": "商品订单"
}
```

**响应示例**:
```json
{
  "code": 201,
  "message": "Order created successfully",
  "data": {
    "id": 1,
    "order": {
      "id": 1,
      "user_id": 1,
      "amount": 299.99,
      "description": "商品订单",
      "status": "pending",
      "created_at": "2024-08-29T10:00:00Z"
    }
  }
}
```

#### 3.2 获取订单列表

**接口地址**: `GET /api/v1/orders`

**权限要求**: 需要认证

**查询参数**:
- `page`: 页码，默认1
- `page_size`: 每页数量，默认10
- `status`: 订单状态过滤（可选）

**响应示例**:
```json
{
  "code": 200,
  "message": "Orders retrieved successfully",
  "data": {
    "orders": [
      {
        "id": 1,
        "user_id": 1,
        "amount": 299.99,
        "description": "商品订单",
        "status": "pending",
        "created_at": "2024-08-29T10:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "page_size": 10,
      "total": 1
    }
  }
}
```

#### 3.3 获取订单详情

**接口地址**: `GET /api/v1/orders/{id}`

**权限要求**: 需要认证，只能查看自己的订单或管理员权限

**响应示例**:
```json
{
  "code": 200,
  "message": "Order retrieved successfully",
  "data": {
    "order": {
      "id": 1,
      "user_id": 1,
      "amount": 299.99,
      "description": "商品订单",
      "status": "pending",
      "created_at": "2024-08-29T10:00:00Z",
      "updated_at": "2024-08-29T10:00:00Z"
    }
  }
}
```

#### 3.4 更新订单

**接口地址**: `PUT /api/v1/orders/{id}`

**权限要求**: 需要认证，只能更新自己的订单或管理员权限

**请求参数**:
```json
{
  "amount": 399.99,
  "description": "更新后的订单描述",
  "status": "confirmed"
}
```

**响应示例**:
```json
{
  "code": 200,
  "message": "Order updated successfully",
  "data": {
    "order": {
      "id": 1,
      "user_id": 1,
      "amount": 399.99,
      "description": "更新后的订单描述",
      "status": "confirmed",
      "updated_at": "2024-08-29T11:00:00Z"
    }
  }
}
```

#### 3.5 删除订单

**接口地址**: `DELETE /api/v1/orders/{id}`

**权限要求**: 管理员权限

**响应示例**:
```json
{
  "code": 200,
  "message": "Order deleted successfully",
  "data": null
}
```

#### 3.6 获取用户订单

**接口地址**: `GET /api/v1/orders/user/{user_id}`

**权限要求**: 需要认证，只能查看自己的订单或管理员权限

**查询参数**:
- `page`: 页码，默认1
- `page_size`: 每页数量，默认10

**响应示例**:
```json
{
  "code": 200,
  "message": "User orders retrieved successfully",
  "data": {
    "orders": [
      {
        "id": 1,
        "user_id": 1,
        "amount": 299.99,
        "description": "商品订单",
        "status": "pending",
        "created_at": "2024-08-29T10:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "page_size": 10,
      "total": 1
    }
  }
}
```

### 4. 系统接口

#### 4.1 健康检查

**接口地址**: `GET /health`

**说明**: 公开接口，无需认证

**响应示例**:
```json
{
  "code": 200,
  "message": "Health check passed",
  "data": {
    "gateway": "healthy",
    "rpc_clients": "healthy",
    "user_service": "healthy",
    "order_service": "healthy"
  }
}
```

#### 4.2 监控指标

**接口地址**: `GET /metrics`

**说明**: 返回Prometheus格式的监控指标

**响应示例**:
```
# HELP gateway_http_requests_total Total number of HTTP requests
# TYPE gateway_http_requests_total counter
gateway_http_requests_total{method="GET",path="/api/v1/users",status="200"} 100

# HELP gateway_http_request_duration_seconds HTTP request duration in seconds
# TYPE gateway_http_request_duration_seconds histogram
gateway_http_request_duration_seconds_bucket{method="GET",path="/api/v1/users",le="0.1"} 80
```

## 错误处理

### 常见错误码

#### 400 Bad Request
```json
{
  "code": 400,
  "message": "Invalid request parameter: username is required",
  "data": null
}
```

#### 401 Unauthorized
```json
{
  "code": 401,
  "message": "Invalid token",
  "data": null
}
```

#### 403 Forbidden
```json
{
  "code": 403,
  "message": "Permission denied",
  "data": null
}
```

#### 404 Not Found
```json
{
  "code": 404,
  "message": "User not found",
  "data": null
}
```

#### 429 Too Many Requests
```json
{
  "code": 429,
  "message": "Rate limit exceeded",
  "data": null
}
```

#### 500 Internal Server Error
```json
{
  "code": 500,
  "message": "Internal server error",
  "data": null
}
```

## 调用示例

### cURL示例

#### 用户登录
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "password"
  }'
```

#### 创建用户
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "phone": "13800138000",
    "age": 25
  }'
```

#### 获取用户列表
```bash
curl -X GET http://localhost:8080/api/v1/users?page=1&page_size=10 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### JavaScript示例

```javascript
// 登录
const loginResponse = await fetch('http://localhost:8080/api/v1/auth/login', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    username: 'admin',
    password: 'password'
  })
});

const loginData = await loginResponse.json();
const token = loginData.data.access_token;

// 创建用户
const createUserResponse = await fetch('http://localhost:8080/api/v1/users', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`
  },
  body: JSON.stringify({
    username: 'testuser',
    email: 'test@example.com',
    phone: '13800138000',
    age: 25
  })
});

const userData = await createUserResponse.json();
```

## 限流说明

API Gateway实现了基于IP的限流机制：

- **默认限制**: 每个IP每分钟1000次请求
- **限流算法**: 滑动窗口算法
- **超出限制**: 返回429状态码

## 版本控制

当前API版本：`v1`

- API版本通过URL路径标识：`/api/v1/...`
- 向后兼容性：新版本发布时会保持向后兼容
- 废弃通知：废弃的API会提前通知并在响应头中标识

## 测试工具

推荐使用以下工具进行API测试：

1. **Postman**: 导入API集合进行交互式测试
2. **项目测试脚本**: `./scripts/test_gateway.sh`
3. **cURL**: 命令行工具，适合自动化测试

## 更新日志

### v1.0.0 (2024-08-29)
- 初始版本发布
- 支持用户管理和订单管理
- 实现JWT认证和RBAC权限控制
- 提供完整的RESTful API接口