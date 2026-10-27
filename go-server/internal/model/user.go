// model 包定义数据模型
package model

import (
	"time"
)

// UserKey 单个用户的 SSH 公钥，对应 user_keys 表
// 一个用户名可以关联多把公钥
type UserKey struct {
	// ID 自增主键
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	// Username 用户名
	Username string `gorm:"column:username;type:text;not null;index;uniqueIndex:idx_user_key_user_fp" json:"username"`
	// PublicKey SSH 公钥内容
	PublicKey string `gorm:"column:public_key;type:text;not null" json:"public_key"`
	// Fingerprint 公钥指纹（SHA256:...）
	Fingerprint string `gorm:"column:fingerprint;type:text;not null;index;uniqueIndex:idx_user_key_user_fp" json:"fingerprint"`
	// Source 来源标识（如 github, internal）
	Source string `gorm:"column:source;type:text" json:"source,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 返回表名
func (UserKey) TableName() string {
	return "user_keys"
}

// User 旧版单公钥模型，仅用于迁移历史数据
type User struct {
	// Username 旧表主键，迁移后不再写入
	Username string `gorm:"primaryKey;column:username;type:text"`
	// PublicKey 旧表单把公钥
	PublicKey string `gorm:"column:public_key;type:text;not null"`
	// Source 旧表来源标识
	Source string `gorm:"column:source;type:text"`
	// CreatedAt 旧表创建时间
	CreatedAt time.Time `gorm:"column:created_at"`
	// UpdatedAt 旧表更新时间
	UpdatedAt time.Time `gorm:"column:updated_at"`
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
	Username string `gorm:"column:username;type:text;not null;index" json:"username"`
	// Action 操作类型: add / replace / delete
	Action string `gorm:"column:action;type:text;not null" json:"action"`
	// ClientIP 客户端 IP
	ClientIP string `gorm:"column:client_ip;type:text" json:"client_ip,omitempty"`
	// UserAgent 客户端 User-Agent
	UserAgent string `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index" json:"created_at"`
}

// TableName 返回表名
func (AuditLog) TableName() string {
	return "audit_logs"
}
