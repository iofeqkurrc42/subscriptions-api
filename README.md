# 订阅管理系统后端 / Subscription Manager Backend

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-blue" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Gin-1.9.1-green" alt="Gin 1.9.1">
  <img src="https://img.shields.io/badge/SQLite-3-lightgrey" alt="SQLite 3">
  <img src="https://img.shields.io/badge/AI-Generated-purple" alt="AI Generated">
</p>

> **🤖 AI 生成声明 / AI Generated Statement**: 本项目由人工智能助手 (OpenCode AI) 协助生成。
> **This project was generated with assistance from an AI assistant (OpenCode AI).**

---

## 📖 目录 / Table of Contents

- [中文文档](#中文文档)
  - [功能特性](#功能特性)
  - [技术栈](#技术栈)
  - [项目结构](#项目结构)
  - [快速开始](#快速开始)
  - [API 文档](#api-文档)
  - [配置说明](#配置说明)
  - [环境变量](#环境变量)
- [English Documentation](#english-documentation)
  - [Features](#features)
  - [Tech Stack](#tech-stack)
  - [Project Structure](#project-structure)
  - [Quick Start](#quick-start)
  - [API Documentation](#api-documentation)
  - [Configuration](#configuration)
  - [Environment Variables](#environment-variables)

---

## 中文文档

### 功能特性

- 📅 **订阅管理** - 创建、更新、删除订阅服务
- ⏰ **到期提醒** - 自动检测即将过期和已过期的订阅
- 🔔 **多渠道通知** - 支持 Server酱(微信) 和 SMTP 邮件通知
- 📊 **统计分析** - 活跃订阅数和月均消费统计
- 🔐 **JWT 认证** - 安全的登录认证机制
- 📝 **通知日志** - 记录所有发送的通知历史
- 🔄 **续期功能** - 一键续期订阅服务
- ⏱️ **定时任务** - 每天定时自动检查订阅状态

### 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| Go | 1.26 | 编程语言 |
| Gin | 1.9.1 | Web 框架 |
| SQLite3 | 1.14.18 | 数据库 |
| bcrypt | 内置 | 密码加密 |
| JWT | 自定义实现 | 身份认证 |

### 项目结构

```
.
├── cmd/
│   └── init/           # 初始化命令
│       └── main.go     # 配置初始化程序
├── config/
│   ├── config.go       # 数据库初始化
│   └── config.yaml     # 配置文件 (自动生成)
├── data/
│   └── subscriptions.db # SQLite 数据库 (自动生成)
├── handlers/
│   └── handlers.go     # HTTP 请求处理器
├── models/
│   └── subscription.go # 数据模型和数据库操作
├── notify/
│   └── notify.go       # 通知服务 (微信/邮件)
├── go.mod              # Go 模块定义
├── go.sum              # Go 依赖校验
└── main.go             # 应用程序入口
```

### 快速开始

#### 1. 克隆项目

```bash
git clone <repository-url>
cd subscription-manager/backend
```

#### 2. 安装依赖

```bash
go mod download
```

#### 3. 初始化配置

```bash
go run ./cmd/init
```

根据提示设置：
- 登录密码
- JWT Secret
- Server酱 Key (可选，用于微信通知)
- SMTP 配置 (可选，用于邮件通知)
- 定时检查时间

#### 4. 运行服务

```bash
go run main.go
```

服务将在 `http://localhost:8080` 启动

#### 5. 验证服务

```bash
curl http://localhost:8080/health
```

### API 文档

#### 认证

所有受保护的 API 需要在请求头中携带 JWT Token：

```
Authorization: <token>
```

或在 URL 参数中：

```
?token=<token>
```

#### 端点列表

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| POST | `/api/login` | 用户登录 | 否 |
| GET | `/api/stats` | 获取统计信息 | 是 |
| GET | `/api/subscriptions` | 获取订阅列表 | 是 |
| POST | `/api/subscriptions` | 创建订阅 | 是 |
| PUT | `/api/subscriptions/:id` | 更新订阅 | 是 |
| PUT | `/api/subscriptions/:id/toggle` | 切换订阅状态 | 是 |
| PUT | `/api/subscriptions/:id/renew` | 续期订阅 | 是 |
| DELETE | `/api/subscriptions/:id` | 删除订阅 | 是 |
| GET | `/api/notifications` | 获取通知日志 | 是 |
| GET | `/health` | 健康检查 | 否 |

#### 请求示例

**登录**

```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"password": "your_password"}'
```

响应：
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**创建订阅**

```bash
curl -X POST http://localhost:8080/api/subscriptions \
  -H "Content-Type: application/json" \
  -H "Authorization: <token>" \
  -d '{
    "name": "Netflix",
    "remark": "家庭套餐",
    "type": 1,
    "period": 12,
    "price": 198.00,
    "start_date": "2024-01-01",
    "expire_date": "2025-01-01",
    "notify_days": 7
  }'
```

**续期订阅**

```bash
curl -X PUT http://localhost:8080/api/subscriptions/1/renew \
  -H "Content-Type: application/json" \
  -H "Authorization: <token>" \
  -d '{"period": 12}'
```

### 配置说明

配置文件位于 `config/config.yaml`：

```yaml
# 登录密码 (bcrypt 加密)
password: $2a$10$...

# JWT 密钥
jwt_secret: your_secret_key

# Server酱配置 (微信通知)
serverchan:
  key: SCTxxxxxxxxxxxx

# SMTP 配置 (邮件通知)
smtp:
  server: smtp.163.com
  port: 465
  auth_code: your_auth_code
  from: sender@example.com
  to: receiver@example.com

# 定时检查配置
schedule:
  hour: 10      # 每天 10 点
  minute: 30    # 30 分
```

### 环境变量

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `SERVER_CHAN_KEY` | Server酱 Key | `SCT123456789` |
| `NOTIFY_DAYS` | 提前通知天数 | `3` |

---

## English Documentation

### Features

- 📅 **Subscription Management** - Create, update, delete subscription services
- ⏰ **Expiration Alerts** - Automatically detect expiring and expired subscriptions
- 🔔 **Multi-channel Notifications** - Support ServerChan (WeChat) and SMTP email notifications
- 📊 **Statistics** - Active subscriptions count and monthly spending stats
- 🔐 **JWT Authentication** - Secure login authentication mechanism
- 📝 **Notification Logs** - Record all sent notification history
- 🔄 **Renewal Function** - One-click subscription renewal
- ⏱️ **Scheduled Tasks** - Daily automatic subscription status checks

### Tech Stack

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.26 | Programming Language |
| Gin | 1.9.1 | Web Framework |
| SQLite3 | 1.14.18 | Database |
| bcrypt | Built-in | Password Encryption |
| JWT | Custom Implementation | Authentication |

### Project Structure

```
.
├── cmd/
│   └── init/           # Initialization command
│       └── main.go     # Configuration initialization program
├── config/
│   ├── config.go       # Database initialization
│   └── config.yaml     # Configuration file (auto-generated)
├── data/
│   └── subscriptions.db # SQLite database (auto-generated)
├── handlers/
│   └── handlers.go     # HTTP request handlers
├── models/
│   └── subscription.go # Data models and database operations
├── notify/
│   └── notify.go       # Notification service (WeChat/Email)
├── go.mod              # Go module definition
├── go.sum              # Go dependency checksum
└── main.go             # Application entry point
```

### Quick Start

#### 1. Clone the project

```bash
git clone <repository-url>
cd subscription-manager/backend
```

#### 2. Install dependencies

```bash
go mod download
```

#### 3. Initialize configuration

```bash
go run ./cmd/init
```

Follow the prompts to set up:
- Login password
- JWT Secret
- ServerChan Key (optional, for WeChat notifications)
- SMTP configuration (optional, for email notifications)
- Scheduled check time

#### 4. Run the service

```bash
go run main.go
```

The service will start at `http://localhost:8080`

#### 5. Verify the service

```bash
curl http://localhost:8080/health
```

### API Documentation

#### Authentication

All protected APIs require a JWT Token in the request header:

```
Authorization: <token>
```

Or as a URL parameter:

```
?token=<token>
```

#### Endpoints

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| POST | `/api/login` | User login | No |
| GET | `/api/stats` | Get statistics | Yes |
| GET | `/api/subscriptions` | Get subscription list | Yes |
| POST | `/api/subscriptions` | Create subscription | Yes |
| PUT | `/api/subscriptions/:id` | Update subscription | Yes |
| PUT | `/api/subscriptions/:id/toggle` | Toggle subscription status | Yes |
| PUT | `/api/subscriptions/:id/renew` | Renew subscription | Yes |
| DELETE | `/api/subscriptions/:id` | Delete subscription | Yes |
| GET | `/api/notifications` | Get notification logs | Yes |
| GET | `/health` | Health check | No |

#### Request Examples

**Login**

```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"password": "your_password"}'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Create Subscription**

```bash
curl -X POST http://localhost:8080/api/subscriptions \
  -H "Content-Type: application/json" \
  -H "Authorization: <token>" \
  -d '{
    "name": "Netflix",
    "remark": "Family Plan",
    "type": 1,
    "period": 12,
    "price": 198.00,
    "start_date": "2024-01-01",
    "expire_date": "2025-01-01",
    "notify_days": 7
  }'
```

**Renew Subscription**

```bash
curl -X PUT http://localhost:8080/api/subscriptions/1/renew \
  -H "Content-Type: application/json" \
  -H "Authorization: <token>" \
  -d '{"period": 12}'
```

### Configuration

Configuration file is located at `config/config.yaml`:

```yaml
# Login password (bcrypt encrypted)
password: $2a$10$...

# JWT secret
jwt_secret: your_secret_key

# ServerChan configuration (WeChat notifications)
serverchan:
  key: SCTxxxxxxxxxxxx

# SMTP configuration (Email notifications)
smtp:
  server: smtp.163.com
  port: 465
  auth_code: your_auth_code
  from: sender@example.com
  to: receiver@example.com

# Scheduled check configuration
schedule:
  hour: 10      # 10 AM daily
  minute: 30    # 30 minutes
```

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `SERVER_CHAN_KEY` | ServerChan Key | `SCT123456789` |
| `NOTIFY_DAYS` | Days to notify in advance | `3` |

---

## 📄 License / 许可证

This project is for personal use. Feel free to modify and distribute.

本项目仅供个人使用，欢迎修改和分发。

## 🙏 Acknowledgments / 致谢

- Built with [Gin Web Framework](https://gin-gonic.com/)
- Database powered by [SQLite](https://www.sqlite.org/)
- Notifications via [ServerChan](https://sct.ftqq.com/)
