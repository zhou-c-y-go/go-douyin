package pojo

import (
	"time"
)

type Video struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`          // 视频全局唯一主键ID
	Title         string    `gorm:"type:varchar(255);not null" json:"title"`     // 视频标题/描述
	AuthorID      int64     `gorm:"index;not null" json:"author_id"`             // 存作者ID，加索引，拒绝名字冗余导致的一致性溃败！
	VideoUrl      string    `gorm:"type:varchar(500);not null" json:"video_url"` // 视频播放源（MinIO 或 OSS 地址）
	CoverUrl      string    `gorm:"type:varchar(500);not null" json:"cover_url"` // 封面图片地址
	Duration      int       `gorm:"type:int;default:0" json:"duration"`          // 播放时长（单位：秒），方便前端灵活格式化与后台筛选
	LikeCount     int64     `gorm:"type:bigint;default:0" json:"like_count"`     // 严禁字符串,使用 bigint 确保高性能原子自增与准确热度排序
	FavoriteCount int64     `gorm:"type:bigint;default:0" json:"favorite_count"` // “收藏数”计数器
	Tags          string    `gorm:"type:varchar(255)" json:"tags"`               // 视频标签/分类
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`            // 视频发布时间
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`            // 更新时间
}
