package response

import "time"

// CommentVO ── 树状嵌套评论响应完全体
type CommentVO struct {
	ID        int64        `json:"id"`
	VideoID   int64        `json:"video_id"`
	Content   string       `json:"content"`
	Path      string       `json:"path"`
	ReplyToID int64        `json:"reply_to_id"`
	CreatedAt time.Time    `json:"created_at"`
	User      UserCardInfo `json:"user"`     // 🎯 核心注入：发布者的真实社交身份（名字和头像）
	Children  []*CommentVO `json:"children"` // 🎯 核心嵌套：当前评论下的子评论大军（无限裂变树）
}

// UserCardInfo ── 评论区用户信息卡片
type UserCardInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}
