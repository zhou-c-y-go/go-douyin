package pojo

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	AuthorID uint   `gorm:"index;not null" json:"author_id"`
	PlayURL  string `gorm:"type:varchar(255);not null" json:"play_url"`  // 视频播放地址 (Minio)
	CoverURL string `gorm:"type:varchar(255);not null" json:"cover_url"` // 封面图地址
	Title    string `gorm:"type:varchar(100);not null" json:"title"`
}
