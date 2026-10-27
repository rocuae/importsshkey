package repository

import (
	"gorm.io/gorm"

	"github.com/rocuae/importsshkey/go-server/internal/model"
)

// GormUserKeyRepository 基于 GORM 的公钥仓储实现
type GormUserKeyRepository struct {
	db *gorm.DB
}

// NewGormUserKeyRepository 创建 GORM 公钥仓储
func NewGormUserKeyRepository(db *gorm.DB) *GormUserKeyRepository {
	return &GormUserKeyRepository{db: db}
}

// ListByUsername 查询用户的全部公钥
func (r *GormUserKeyRepository) ListByUsername(username string) ([]*model.UserKey, error) {
	var keys []*model.UserKey
	err := r.db.Where("username = ?", username).
		Order("created_at ASC, id ASC").
		Find(&keys).Error
	return keys, err
}

// Add 添加单把公钥；同一用户下指纹重复时更新来源
func (r *GormUserKeyRepository) Add(key *model.UserKey) error {
	return addWithDB(r.db, key)
}

// AddMany 批量添加公钥（单事务）
func (r *GormUserKeyRepository) AddMany(keys []*model.UserKey) error {
	if len(keys) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, key := range keys {
			if err := addWithDB(tx, key); err != nil {
				return err
			}
		}
		return nil
	})
}

// addWithDB 在事务内按 (username,fingerprint) 幂等写入，保留已有 CreatedAt。
func addWithDB(db *gorm.DB, key *model.UserKey) error {
	var existing model.UserKey
	result := db.Where("username = ? AND fingerprint = ?", key.Username, key.Fingerprint).
		Limit(1).Find(&existing)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return db.Create(key).Error
	}
	existing.PublicKey = key.PublicKey
	existing.Source = key.Source
	return db.Save(&existing).Error
}

// Replace 全量替换用户的所有公钥（单事务）
func (r *GormUserKeyRepository) Replace(username string, keys []*model.UserKey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("username = ?", username).Delete(&model.UserKey{}).Error; err != nil {
			return err
		}
		if len(keys) == 0 {
			return nil
		}
		for _, key := range keys {
			key.Username = username
		}
		return tx.CreateInBatches(keys, 100).Error
	})
}

// DeleteByUsername 删除用户的全部公钥
func (r *GormUserKeyRepository) DeleteByUsername(username string) (int64, error) {
	result := r.db.Where("username = ?", username).Delete(&model.UserKey{})
	return result.RowsAffected, result.Error
}

// DeleteByFingerprint 删除用户指定指纹的公钥
func (r *GormUserKeyRepository) DeleteByFingerprint(username, fingerprint string) (int64, error) {
	result := r.db.Where("username = ? AND fingerprint = ?", username, fingerprint).
		Delete(&model.UserKey{})
	return result.RowsAffected, result.Error
}

// ListUsers 分页列出用户名，支持模糊搜索
func (r *GormUserKeyRepository) ListUsers(query string, offset, limit int) ([]string, int64, error) {
	base := r.db.Model(&model.UserKey{})
	if query != "" {
		base = base.Where("username LIKE ?", "%"+query+"%")
	}

	var total int64
	if err := base.Distinct("username").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []string
	if err := base.Distinct("username").
		Order("username ASC").
		Offset(offset).
		Limit(limit).
		Pluck("username", &users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// CountUsers 统计不同用户数
func (r *GormUserKeyRepository) CountUsers() (int64, error) {
	var count int64
	err := r.db.Model(&model.UserKey{}).Distinct("username").Count(&count).Error
	return count, err
}

// CountKeys 统计公钥总数
func (r *GormUserKeyRepository) CountKeys() (int64, error) {
	var count int64
	err := r.db.Model(&model.UserKey{}).Count(&count).Error
	return count, err
}

// Exists 检查用户是否存在
func (r *GormUserKeyRepository) Exists(username string) (bool, error) {
	var count int64
	err := r.db.Model(&model.UserKey{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

// GormAuditLogRepository 基于 GORM 的审计日志仓储实现
type GormAuditLogRepository struct {
	db *gorm.DB
}

// NewGormAuditLogRepository 创建 GORM 审计日志仓储
func NewGormAuditLogRepository(db *gorm.DB) *GormAuditLogRepository {
	return &GormAuditLogRepository{db: db}
}

// Create 创建审计日志
func (r *GormAuditLogRepository) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

// ListByUsername 查询指定用户的审计日志
func (r *GormAuditLogRepository) ListByUsername(username string, limit int) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Where("username = ?", username).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// ListRecent 查询最近的审计日志
func (r *GormAuditLogRepository) ListRecent(limit int) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	err := r.db.Order("created_at DESC, id DESC").Limit(limit).Find(&logs).Error
	return logs, err
}
