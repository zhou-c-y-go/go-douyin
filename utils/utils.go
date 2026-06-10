package utils

import "fmt"

func GetLikeSetKey(targetType string, userID int64) string {
	// 首字母大写
	switch targetType {
	case "video":
		return fmt.Sprintf("User:Like:Videos:%d", userID)
	case "article":
		return fmt.Sprintf("User:Like:Articles:%d", userID)
	case "image_text":
		return fmt.Sprintf("User:Like:ImageTexts:%d", userID)
	default:
		return ""
	}
}

func GetFavoriteSetKey(targetType string, userID int64) string {
	// 首字母大写
	switch targetType {
	case "video":
		return fmt.Sprintf("User:Favorite:Videos:%d", userID)
	case "article":
		return fmt.Sprintf("User:Favorite:Articles:%d", userID)
	case "image_text":
		return fmt.Sprintf("User:Favorite:ImageTexts:%d", userID)
	default:
		return ""
	}
}
