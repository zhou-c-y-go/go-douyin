package service

import (
	"Go_Project/common/model/pojo" //
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/global"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

type CommentService struct {
	commentRepo repo.CommentRepository
}

// GetVideoCommentTreeService ：组装无限嵌套评论树
func (s *CommentService) GetVideoCommentTreeService(ctx context.Context, videoID int64,
	currentUserID int64) ([]*response.CommentVO, error) {
	commentListKey := fmt.Sprintf("Comment:List:%d", videoID)
	var pojoComments []pojo.Comment
	// 从redis中读取数据，没有读到就从mysql中获取
	cacheData, err := global.GVA_REDIS.Get(ctx, commentListKey).Result()
	if err == nil && cacheData != "" {
		_ = json.Unmarshal([]byte(cacheData), &pojoComments)
	} else {
		// 从mysql中获取数据
		pojoComments, err = s.commentRepo.GetCommentsByVideoID(ctx, videoID)
		if err != nil {
			global.LogCtx(ctx).Errorw("GetCommentsByVideoID函数从MySQL中读取数据库失败", "Error", err)
			return nil, err
		}
		if len(pojoComments) == 0 {
			return []*response.CommentVO{}, nil
		}
		jsonData, _ := json.Marshal(pojoComments)
		err = global.GVA_REDIS.Set(ctx, commentListKey, jsonData, time.Hour*24).Err()
		if err != nil {
			global.LogCtx(ctx).Errorf("[%s]写入错误: %s", commentListKey, err.Error())
		}
	}
	// 获取用户ID数组
	userIDs := make([]int64, 0)
	userMapUnique := make(map[int64]bool)
	// 用户ID去重
	for _, c := range pojoComments {
		if !userMapUnique[c.UserID] {
			userMapUnique[c.UserID] = true
			userIDs = append(userIDs, c.UserID)
		}
	}

	userCardMap := make(map[int64]response.UserCardInfo)
	for _, uid := range userIDs {
		var u pojo.User
		userKey := fmt.Sprintf("UserProfile:%d", uid)
		// 将参数绑定到用户u
		_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&u)
		if u.ID == 0 {
			findErr := global.GVA_DB.First(&u, uid).Error
			if findErr != nil {
				global.LogCtx(ctx).Errorw("[GetVideoCommentTreeService] 查找用户失败",
					"uid", uid, "err", findErr)
			}
		}
		userCardMap[uid] = response.UserCardInfo{
			ID:       u.ID,
			Username: u.Username,
			Avatar:   u.HeadImg,
		}
	}

	// 4. 🎯【斩杀冷缓存地雷】构建当前观众的评论点赞全景位图（单次批量 Pluck，零 N+1 轰炸）
	//存储当前登录用户点赞过的所有评论 ID
	likedCommentMap := make(map[int64]bool)
	if currentUserID > 0 {
		userLikeKey := fmt.Sprintf("User:Like:Comments:%d", currentUserID)
		// A. 探测 Redis 里有没有该用户完整的评论点赞账本
		exists, existErr := global.GVA_REDIS.Exists(ctx, userLikeKey).Result()
		if existErr == nil && exists > 0 {
			// ⚡ 缓存击中！获取用户点赞过的所有评论的ID数组
			likedIDsStr, _ := global.GVA_REDIS.SMembers(ctx, userLikeKey).Result()
			for _, idStr := range likedIDsStr {
				if id, parseErr := strconv.ParseInt(idStr, 10, 64); parseErr == nil {
					likedCommentMap[id] = true
				}
			}
		} else {
			// B. 缓存未击中：从mysql中获取
			var likedIDs []int64
			global.GVA_DB.WithContext(ctx).Model(&pojo.LikeRecord{}).
				Where("user_id = ? AND target_type = ? AND status = 1", currentUserID, "comment").
				Pluck("target_id", &likedIDs)

			// 🌟 核心提热：将历史明细顺手夯入 Redis，供下一次刷新光支并网
			if len(likedIDs) > 0 {
				interfaces := make([]interface{}, len(likedIDs))
				for i, id := range likedIDs {
					interfaces[i] = id
					likedCommentMap[id] = true
				}
				global.GVA_REDIS.SAdd(ctx, userLikeKey, interfaces...)
				global.GVA_REDIS.Expire(ctx, userLikeKey, 24*time.Hour) // 赋予 24 小时动态热度寿命
			}
		}
	}

	// 5. ⚡ Pipeline 管道并网：打包获取各个评论的【点赞总数】
	pipe := global.GVA_REDIS.Pipeline()
	countCmds := make(map[int64]*redis.StringCmd)
	for _, pc := range pojoComments {
		countKey := fmt.Sprintf("Like:Count:comment:%d", pc.ID)
		countCmds[pc.ID] = pipe.Get(ctx, countKey)
	}
	_, _ = pipe.Exec(ctx)

	// 6. 🎛️ 终极两轮算法：就地焊接成树
	voMap := make(map[int64]*response.CommentVO)
	rootComments := make([]*response.CommentVO, 0)
	// 【第一轮循环】：创建 VO 对象，填充基础数据
	for _, pc := range pojoComments {
		realtimeLikeCount := pc.LikeCount
		if countCmd, exists := countCmds[pc.ID]; exists {
			if cnt, err := countCmd.Int64(); err == nil {
				realtimeLikeCount = cnt
			}
		}
		vo := &response.CommentVO{
			ID:         pc.ID,
			VideoID:    pc.VideoID,
			Content:    pc.Content,
			Path:       pc.Path,
			ReplyToID:  pc.ReplyToID,
			CreatedAt:  pc.CreatedAt,
			User:       userCardMap[pc.UserID],
			Children:   make([]*response.CommentVO, 0),
			TargetType: "comment",
			LikeCount:  realtimeLikeCount,
			IsLiked:    likedCommentMap[pc.ID],
		}
		voMap[vo.ID] = vo
	}
	// 第二次循环：挂载子评论
	for _, pc := range pojoComments {
		currentVO := voMap[pc.ID]
		if pc.ReplyToID == 0 {
			rootComments = append(rootComments, currentVO)
		} else {
			if parentVO, exists := voMap[pc.ReplyToID]; exists {
				parentVO.Children = append(parentVO.Children, currentVO)
			}
		}
	}
	return rootComments, nil
}

// PublishCommentService ── 核心写入管道：带物化路径（Path）自增的评论发布引擎
func (s *CommentService) PublishCommentService(
	ctx context.Context,
	videoID int64,
	userID int64,
	content string,
	replyToID int64) error {
	// 1. 初始化基础实体
	newComment := &pojo.Comment{
		VideoID:   videoID,
		UserID:    userID,
		Content:   content,
		ReplyToID: replyToID,
	}

	// 2. ⚡ 第一次落盘：拿到 MySQL 的全新自增主键 ID
	err := s.commentRepo.CreateComment(ctx, newComment)
	if err != nil {
		global.LogCtx(ctx).Errorf("[PublishCommentService]: 创建新评论失败\tError: %s\t"+"对应的视频[%d]",
			err.Error(), videoID)
		return errors.New("评论发射失败，数据库接收端流产")
	}

	// 3. 🌳 物化路径 (Materialized Path) 繁衍算法
	var newPath string
	// 新根评论
	if replyToID == 0 {
		newPath = fmt.Sprintf("%d/", newComment.ID)
	} else {
		//新的子评论
		var parentComment pojo.Comment
		err = global.GVA_DB.WithContext(ctx).Where("id = ?", replyToID).First(&parentComment).Error
		if err != nil {
			global.LogCtx(ctx).Warnf("⚠️ 评论 [%d] 找不到父级 [%d]，强行降级为根评论", newComment.ID, replyToID)
			newPath = fmt.Sprintf("%d/", newComment.ID)
		} else {
			newPath = fmt.Sprintf("%s%d/", parentComment.Path, newComment.ID)
		}
	}

	// 4. ⚡ 第二次落盘：更新评论的path字段
	err = global.GVA_DB.WithContext(ctx).Model(newComment).Update("path", newPath).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Comment] 评论 [%d] 物化路径组装大翻车: %v", newComment.ID, err)
		return errors.New("评论树路径装配发生异常")
	}

	// 5. 🎯【高并发并网核心】主动预热防线
	commentListKey := fmt.Sprintf("Comment:List:%d", videoID)
	// A. 首先将老旧的快照立刻核平抹去（保证短暂的极端并发下不读到脏数据）
	global.GVA_REDIS.Del(ctx, commentListKey)
	// B. 🚀 开启异步预热协程：后台主动召回最新的全量原始评论切片压入 Redis，绝不让用户的读请求承担穿透代价！
	go func(vID int64) {
		// 衍生独立不受外部取消影响的全新上下文环境，给予 5 秒超时保护
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// 在后台线程偷偷把最新数据捞好并塞回 Redis宇宙中
		freshComments, freshErr := s.commentRepo.GetCommentsByVideoID(bgCtx, vID)
		if freshErr == nil && len(freshComments) > 0 {
			jsonData, _ := json.Marshal(freshComments)
			// 赋予 24 小时动态热度寿命
			global.GVA_REDIS.Set(bgCtx, commentListKey, jsonData, time.Hour*24)
			global.LogCtx(bgCtx).Infof("🔄 [Async-Prewarm] 🎉 成功在后台为视频 [%d] 完成全量评论树列表的冷启动主动预热缓存！", vID)
		}
	}(videoID)
	global.LogCtx(ctx).Infof("✅ [Comment] 用户 [%d] 成功发表评论，Path: %s，异步已启动", userID, newPath)
	return nil
}
