package pojo

import "time"

type FollowRelation struct {
	ID int64 `gorm:"primaryKey;column:id"`
	// 🎯 核心微操：建立联合唯一索引 idx_user_target，死锁防护第一步
	UserID    int64     `gorm:"uniqueIndex:idx_user_target;column:user_id;comment:关注者ID(粉丝)"`
	TargetID  int64     `gorm:"uniqueIndex:idx_user_target;column:target_id;comment:被关注者ID(UP主)"`
	Status    int8      `gorm:"column:status;default:1;comment:1:关注, 0:取消关注"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}
