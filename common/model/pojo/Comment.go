package pojo

import (
	"time"
)

type Comment struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoID   int64     `gorm:"index;not null" json:"video_id"`      // 统一类型：与 Video.ID 的 int64 对齐，防止外键索引类型不一致引发性能降级
	UserID    int64     `gorm:"index;not null" json:"user_id"`       // 统一类型：与 User.ID 对齐
	Content   string    `gorm:"type:text;not null" json:"content"`   // 评论纯文本内容
	Path      string    `gorm:"type:varchar(255);index" json:"path"` // 你的核心亮点：物化路径（如 "1/3/"），利用前缀索引秒级拉取整树
	ReplyToID int64     `gorm:"index;default:0" json:"reply_to_id"`  // 可选：仅仅为了前端直观展示“回复了谁”，不参与递归
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	LikeCount int64     `gorm:"default:0" json:"like_count"`
}
