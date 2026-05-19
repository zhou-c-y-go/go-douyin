package pojo

import (
	"gorm.io/gorm"
	"time"
)

type Video struct {
	gorm.Model
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Title      string    `gorm:"type:varchar(255);not null" json:"title"`      // 视频标题/描述
	AuthorName string    `gorm:"type:varchar(50);not null" json:"author_name"` // 作者昵称
	VideoUrl   string    `gorm:"type:varchar(500);not null" json:"video_url"`  // 视频播放源
	CoverUrl   string    `gorm:"type:varchar(500);not null" json:"cover_url"`  // 封面图片
	Duration   string    `gorm:"type:varchar(20)" json:"duration"`             // 播放时长
	LikeCount  string    `gorm:"type:varchar(20)" json:"like_count" `          // 点赞数
	Tags       string    `gorm:"type:varchar(255)" json:"tags"`                // 标签
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
}
