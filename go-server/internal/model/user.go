// model 包定义数据模型
package model

import (
	"time"
)

// User 用户实体，对应数据库 users 表
// 存储用户的 SSH 公钥信息
type User struct {
	// Username 用户名，主键
	Username string `gorm:"primaryKey;column:username;type:text" json:"username"`
	// PublicKey SSH 公钥内容
	PublicKey string `gorm:"column:public_key;type:text;not null" json:"public_key"`
	// Source 来源标识（如 github, internal）
	Source string `gorm:"column:source;type:text" json:"source,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 返回表名
func (User) TableName() string {
	return "users"
}

// AuditLog 审计日志实体，对应数据库 audit_logs 表
// 记录所有公钥操作的审计信息
type AuditLog struct {
	// ID 日志 ID，自增主键
	ID int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// Username 操作的用户名
	Username string `gorm:"column:username;type:text;not null" json:"username"`
	// Action 操作类型: add / update / delete
	Action string `gorm:"column:action;type:text;not null" json:"action"`
	// ClientIP 客户端 IP
	ClientIP string `gorm:"column:client_ip;type:text" json:"client_ip,omitempty"`
	// UserAgent 客户端 User-Agent
	UserAgent string `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

// TableName 返回表名
func (AuditLog) TableName() string {
	return "audit_logs"
}
