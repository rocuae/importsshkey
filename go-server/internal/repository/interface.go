// repository 包定义数据访问接口
package repository

import (
	"github.com/importsshkey/iskey-server/internal/model"
)

// UserRepository 用户数据仓储接口
// 定义用户公钥的 CRUD 操作
type UserRepository interface {
	// CreateOrUpdate 创建或更新用户公钥
	// 参数：
	//   - user: 用户实体
	// 返回：
	//   - error: 数据库错误
	CreateOrUpdate(user *model.User) error

	// GetByUsername 根据用户名查询用户
	// 参数：
	//   - username: 用户名
	// 返回：
	//   - *model.User: 用户实体（不存在返回 nil）
	//   - error: 数据库错误
	GetByUsername(username string) (*model.User, error)

	// DeleteByUsername 根据用户名删除用户
	// 参数：
	//   - username: 用户名
	// 返回：
	//   - error: 数据库错误
	DeleteByUsername(username string) error

	// ListAll 查询所有用户
	// 返回：
	//   - []*model.User: 用户列表
	//   - error: 数据库错误
	ListAll() ([]*model.User, error)

	// Exists 检查用户是否存在
	// 参数：
	//   - username: 用户名
	// 返回：
	//   - bool: 是否存在
	//   - error: 数据库错误
	Exists(username string) (bool, error)
}

// AuditLogRepository 审计日志仓储接口
type AuditLogRepository interface {
	// Create 创建审计日志
	// 参数：
	//   - log: 审计日志实体
	// 返回：
	//   - error: 数据库错误
	Create(log *model.AuditLog) error

	// ListByUsername 查询指定用户的审计日志
	// 参数：
	//   - username: 用户名
	//   - limit: 返回数量限制
	// 返回：
	//   - []*model.AuditLog: 日志列表
	//   - error: 数据库错误
	ListByUsername(username string, limit int) ([]*model.AuditLog, error)
}
