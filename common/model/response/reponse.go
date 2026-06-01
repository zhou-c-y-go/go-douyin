package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type BusinessCode = int32

const (
	OK    BusinessCode = 1
	ERROR BusinessCode = 0
)

type Response struct {
	Code    int32       `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"error-message"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		OK,
		data,
		"ok",
	})
}

func Fail(c *gin.Context, _ BusinessCode, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		ERROR,
		nil,
		msg,
	})
}

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

// AuthorInfo ── 专门吐给前端的作者精简版社交卡片
type AuthorInfo struct {
	ID        int64  `json:"id"`        // 作者ID
	Username  string `json:"username"`  // 作者昵称（回源自用户表/Redis缓存）
	Avatar    string `json:"avatar"`    // 作者头像链接（MinIO 绝对路径）
	Signature string `json:"signature"` // 作者个性签名
}

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
}
