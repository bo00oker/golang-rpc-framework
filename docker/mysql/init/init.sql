# MySQL初始化脚本
CREATE DATABASE IF NOT EXISTS `rpc_framework` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS `rpc_framework_test` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS `nacos` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 创建用户并授权
CREATE USER IF NOT EXISTS 'rpc_user'@'%' IDENTIFIED BY 'rpc_pass123';
GRANT ALL PRIVILEGES ON `rpc_framework`.* TO 'rpc_user'@'%';
GRANT ALL PRIVILEGES ON `rpc_framework_test`.* TO 'rpc_user'@'%';
GRANT ALL PRIVILEGES ON `nacos`.* TO 'rpc_user'@'%';

FLUSH PRIVILEGES;

# 初始化业务表
USE rpc_framework;

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  `email` varchar(255) NOT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `status` tinyint(1) DEFAULT 1,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_email` (`email`),
  KEY `idx_name` (`name`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 订单表
CREATE TABLE IF NOT EXISTS `orders` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) NOT NULL,
  `order_no` varchar(50) NOT NULL,
  `amount` decimal(10,2) NOT NULL,
  `status` varchar(20) DEFAULT 'pending',
  `description` text,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 初始化测试数据
INSERT INTO `users` (`name`, `email`, `phone`) VALUES 
('张三', 'zhangsan@example.com', '13800138000'),
('李四', 'lisi@example.com', '13800138001'),
('王五', 'wangwu@example.com', '13800138002');

INSERT INTO `orders` (`user_id`, `order_no`, `amount`, `status`, `description`) VALUES 
(1, 'ORD001', 99.99, 'completed', '测试订单1'),
(2, 'ORD002', 199.99, 'pending', '测试订单2'),
(3, 'ORD003', 299.99, 'processing', '测试订单3');