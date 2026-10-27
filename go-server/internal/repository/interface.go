// repository 包定义数据访问接口
package repository

import (
	"github.com/rocuae/importsshkey/go-server/internal/model"
)

// UserKeyRepository 公钥数据仓储接口
type UserKeyRepository interface {
	// ListByUsername 查询用户的全部公钥
	ListByUsername(username string) ([]*model.UserKey, error)

	// Add 添加单把公钥；同一用户下指纹重复时更新来源
	Add(key *model.UserKey) error

	// AddMany 批量添加公钥（单事务）
	AddMany(keys []*model.UserKey) error

	// Replace 全量替换用户的所有公钥（单事务）
	Replace(username string, keys []*model.UserKey) error

	// DeleteByUsername 删除用户的全部公钥，返回删除数量
	DeleteByUsername(username string) (int64, error)

	// DeleteByFingerprint 删除用户指定指纹的公钥，返回删除数量
	DeleteByFingerprint(username, fingerprint string) (int64, error)

	// ListUsers 分页列出用户名，支持模糊搜索
	ListUsers(query string, offset, limit int) ([]string, int64, error)

	// CountUsers 统计不同用户数
	CountUsers() (int64, error)

	// CountKeys 统计公钥总数
	CountKeys() (int64, error)

	// Exists 检查用户是否存在
	Exists(username string) (bool, error)
}

// AuditLogRepository 审计日志仓储接口
type AuditLogRepository interface {
	// Create 创建审计日志
	Create(log *model.AuditLog) error

	// ListByUsername 查询指定用户的审计日志
	ListByUsername(username string, limit int) ([]*model.AuditLog, error)

	// ListRecent 查询最近的审计日志
	ListRecent(limit int) ([]*model.AuditLog, error)
}
