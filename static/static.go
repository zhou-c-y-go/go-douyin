package static

import "time"

const Jwt_time = 7 * 24 * time.Hour

var likeList = []string{"video", "article", "imageText"}
var favoriteList = []string{"video", "article", "imageText"}

func GetLikeList() []string {
	// 拷贝一份返回，隔离读写
	res := make([]string, len(likeList))
	copy(res, likeList)
	return res
}

func GetFavoriteList() []string {
	// 拷贝一份返回，隔离读写
	res := make([]string, len(favoriteList))
	copy(res, favoriteList)
	return res
}
