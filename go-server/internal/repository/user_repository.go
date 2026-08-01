package repository

import (
	"errors"

	"gorm.io/gorm"
	"github.com/rocuae/importsshkey/go-server/internal/model"
)

// GormUserRepository 基于 GORM 的用户仓储实现
type GormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository 创建 GORM 用户仓储
// 参数：
//   - db: GORM 数据库实例
// 返回：
//   - *GormUserRepository: 仓储实例
func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

// CreateOrUpdate 创建或更新用户公钥
// 参数：
//   - user: 用户实体
// 返回：
//   - error: 数据库错误
func (r *GormUserRepository) CreateOrUpdate(user *model.User) error {
	return r.db.Save(user).Error
}

// GetByUsername 根据用户名查询用户
// 参数：
//   - username: 用户名
// 返回：
//   - *model.User: 用户实体（不存在返回 nil）
//   - error: 数据库错误
func (r *GormUserRepository) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

// DeleteByUsername 根据用户名删除用户
// 参数：
//   - username: 用户名
// 返回：
//   - error: 数据库错误
func (r *GormUserRepository) DeleteByUsername(username string) error {
	return r.db.Where("username = ?", username).Delete(&model.User{}).Error
}

// ListAll 查询所有用户
// 返回：
//   - []*model.User: 用户列表
//   - error: 数据库错误
func (r *GormUserRepository) ListAll() ([]*model.User, error) {
	var users []*model.User
	err := r.db.Order("username ASC").Find(&users).Error
	return users, err
}

// Exists 检查用户是否存在
// 参数：
//   - username: 用户名
// 返回：
//   - bool: 是否存在
//   - error: 数据库错误
func (r *GormUserRepository) Exists(username string) (bool, error) {
	var count int64
	err := r.db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// GormAuditLogRepository 基于 GORM 的审计日志仓储实现
type GormAuditLogRepository struct {
	db *gorm.DB
}

// NewGormAuditLogRepository 创建 GORM 审计日志仓储
// 参数：
//   - db: GORM 数据库实例
// 返回：
//   - *GormAuditLogRepository: 仓储实例
func NewGormAuditLogRepository(db *gorm.DB) *GormAuditLogRepository {
	return &GormAuditLogRepository{db: db}
}

// Create 创建审计日志
// 参数：
//   - log: 审计日志实体
// 返回：
//   - error: 数据库错误
func (r *GormAuditLogRepository) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

// ListByUsername 查询指定用户的审计日志
// 参数：
//   - username: 用户名
//   - limit: 返回数量限制
// 返回：
//   - []*model.AuditLog: 日志列表
//   - error: 数据库错误
func (r *GormAuditLogRepository) ListByUsername(username string, limit int) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Where("username = ?", username).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
