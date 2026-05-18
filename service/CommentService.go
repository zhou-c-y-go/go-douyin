package service

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
)

// 伪代码演示
func AddReply(videoID uint, userID uint, content string, parentComment pojo.Comment) error {
	newComment := pojo.Comment{
		VideoID: videoID,
		UserID:  userID,
		Content: content,
		// 核心：新路径 = 父路径 + 新生成的ID + "/"
		// 注意：实际开发中，需要先插入获取ID，再更新Path，或者使用雪花算法预先生成ID
	}
	return global.GVA_DB.Create(&newComment).Error
}
