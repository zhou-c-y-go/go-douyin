package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
)

// 直接用 LIKE 走索引查询，拒绝递归！
func GetCommentsByVideo(videoID uint) ([]pojo.Comment, error) {
	var comments []pojo.Comment
	err := global.GVA_DB.Where("video_id = ?", videoID).
		Order("path ASC, created_at ASC"). // 按路径排序，自然呈现树状
		Find(&comments).Error
	return comments, err
}
