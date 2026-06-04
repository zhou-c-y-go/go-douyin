package response

// UserProfileResponse 定义返回给前端的专属数据结构，做好字段名映射
type UserProfileResponse struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Avatar          string `json:"avatar"` // 对应后端的 HeadImg
	BackgroundImage string `json:"background_image"`
	Signature       string `json:"signature"`
	TotalLiked      int64  `json:"total_liked"` // 对应前端的 total_liked (后端 TotalFavorited)
	WorkCount       int64  `json:"work_count"`
	FavoriteCount   int64  `json:"favorite_count"`
	Gender          string `json:"gender"`
}
