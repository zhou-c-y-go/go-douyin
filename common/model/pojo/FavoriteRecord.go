package pojo

import "time"

type FavoriteRecord struct {
	ID         int64     `gorm:"primarykey;autoIncrement;comment:主键"`
	UserID     int64     `gorm:"type:bigint;not null;uniqueIndex:idx_user_biz;comment:点赞用户ID"`
	TargetID   int64     `gorm:"type:bigint;not null;uniqueIndex:idx_user_biz;comment:被点赞业务ID(视频/文章/)"`
	TargetType string    `gorm:"type:varchar(32);not null;uniqueIndex:idx_user_biz;comment:业务类型(video/article)"`
	Status     int8      `gorm:"type:tinyint;not null;default:1;comment:状态(1:已点赞, 0:已取消)"`
	CreatedAt  time.Time `gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime;comment:更新时间"`
}
