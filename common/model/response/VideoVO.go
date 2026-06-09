package response

import "time"

type VideoVO struct {
	ID            int64      `json:"id"`             // 视频ID
	Title         string     `json:"title"`          // 视频标题
	VideoUrl      string     `json:"video_url"`      // 视频播放源
	CoverUrl      string     `json:"cover_url"`      // 封面图
	Duration      int        `json:"duration"`       // 播放时长（秒）
	LikeCount     int64      `json:"like_count"`     // 动态计数的点赞总数（回源自 Redis Hash）
	FavoriteCount int64      `json:"favorite_count"` // 动态计数的收藏总数
	CreatedAt     time.Time  `json:"created_at"`     // 发布时间
	Author        AuthorInfo `json:"author"`
	IsLike        bool       `json:"is_like"`     // 核心痛点：当前刷视频的登录用户，有没有给该视频点赞过？
	IsFavorite    bool       `json:"is_favorite"` // 当前登录用户，有没有收藏过该视频？
	Tags          string     `json:"tags"`
	TargetType    string     `json:"target_type"`
}
