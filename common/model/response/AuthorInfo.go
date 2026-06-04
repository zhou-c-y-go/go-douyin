package response

// AuthorInfo ── 专门吐给前端的作者精简版社交卡片
type AuthorInfo struct {
	ID        int64  `json:"id"`        // 作者ID
	Username  string `json:"username"`  // 作者昵称（回源自用户表/Redis缓存）
	Avatar    string `json:"avatar"`    // 作者头像链接（MinIO 绝对路径）
	Signature string `json:"signature"` // 作者个性签名
}
