package pojo

import "gorm.io/gorm"

type Comment struct {
	gorm.Model
	VideoID   uint   `gorm:"index;not null"` // 属于哪个视频
	UserID    uint   `gorm:"index;not null"` // 谁发的
	Content   string `gorm:"type:text;not null"`
	Path      string `gorm:"type:varchar(255);index"` // 核心亮点：物化路径，加索引！
	ReplyToID uint   // (可选) 仅仅为了前端展示“回复了谁”，不参与递归查询
}
