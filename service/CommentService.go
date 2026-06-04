package service

import (
	"Go_Project/common/model/pojo" //
	"Go_Project/common/model/response"
	repo "Go_Project/common/repository"
	"Go_Project/global"
	"context"
	"errors"
	"fmt"
)

type CommentService struct {
	commentRepo repo.CommentRepository
}

// GetVideoCommentTreeService ：组装无限嵌套评论树
func (s *CommentService) GetVideoCommentTreeService(ctx context.Context, videoID int64) ([]*response.CommentVO, error) {
	// 1. 物理层召回：单查询击穿原始切片
	pojoComments, err := s.commentRepo.GetCommentsByVideoID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if len(pojoComments) == 0 {
		return []*response.CommentVO{}, nil
	}

	// 2. 🛡️ 高并发洗数：批量收集所有发帖人的用户 ID，规避 N+1 悲剧
	userIDs := make([]int64, 0)
	userMapUnique := make(map[int64]bool)
	for _, c := range pojoComments {
		if !userMapUnique[c.UserID] {
			userMapUnique[c.UserID] = true
			userIDs = append(userIDs, c.UserID)
		}
	}

	// 3. 社交并网：批量把这批 user_id 的用户名和头像抓出来（复用宝宝之前的单人卡片逻辑）
	userCardMap := make(map[int64]response.UserCardInfo)
	for _, uid := range userIDs {
		var u pojo.User
		userKey := fmt.Sprintf("UserProfile:%d", uid) //
		// 优先踩中高吞吐量的 Redis 动态哈希阵列
		_ = global.GVA_REDIS.HGetAll(ctx, userKey).Scan(&u)
		if u.ID == 0 {
			// Redis 穿透后兜底 MySQL
			global.GVA_DB.First(&u, uid)
		}
		userCardMap[uid] = response.UserCardInfo{
			ID:       u.ID,
			Username: u.Username,
			Avatar:   u.HeadImg, // 自动对齐 MinIO 桶头像真实路径
		}
	}

	// 4. 🎛️ 终极两轮算法：两阶段就地焊接，实现 0 递归树桩建立
	// 建立全局内存指针坐标索引 Map
	voMap := make(map[int64]*response.CommentVO)
	rootComments := make([]*response.CommentVO, 0)

	// 【第一轮循环】：把所有 pojo 规整进化为 VO，并注入名字和头像，率先铺平塞入 Map
	for _, pc := range pojoComments {
		vo := &response.CommentVO{
			ID:        pc.ID,        //
			VideoID:   pc.VideoID,   //
			Content:   pc.Content,   //
			Path:      pc.Path,      //
			ReplyToID: pc.ReplyToID, //
			CreatedAt: pc.CreatedAt, //
			User:      userCardMap[pc.UserID],
			Children:  make([]*response.CommentVO, 0),
		}
		voMap[vo.ID] = vo
	}

	// 【第二轮循环】：就地咬合关系链
	for _, pc := range pojoComments {
		currentVO := voMap[pc.ID]

		// 🎯 如果 ReplyToID == 0，代表这是一级根评论（比如直接评论视频的楼层）
		if pc.ReplyToID == 0 { //
			rootComments = append(rootComments, currentVO)
		} else {
			// 🎯 如果 ReplyToID > 0，说明它是子孙评论，立刻去 Map 里找出它的顶头上司（父级评论）
			if parentVO, exists := voMap[pc.ReplyToID]; exists { //
				// 顺水推舟，直接推入父级评论的 Children 大军中，指针级同步变更！
				parentVO.Children = append(parentVO.Children, currentVO)
			}
		}
	}

	// 最终只返回根评论列表，里面的 Children 已经层层嵌套好了无限裂变子孙！
	return rootComments, nil
}

// PublishCommentService ── 核心写入管道：带物化路径（Path）自增繁衍的评论发布引擎
func (s *CommentService) PublishCommentService(ctx context.Context, videoID int64, userID int64, content string, replyToID int64) error {
	// 1. 初始化基础实体（此时 Path 为空，等待 MySQL 赐予 ID 后组装）
	newComment := &pojo.Comment{
		VideoID:   videoID,
		UserID:    userID,
		Content:   content,
		ReplyToID: replyToID,
	}

	// 2. ⚡ 第一次落盘：强行敲门，拿到 MySQL 赐予的全新自增主键 ID
	err := s.commentRepo.CreateComment(ctx, newComment)
	if err != nil {
		return errors.New("评论发射失败，数据库拒绝接收")
	}

	// 3. 🌳 物化路径 (Materialized Path) 核心繁衍算法
	var newPath string
	if replyToID == 0 {
		// 🎯 一级根评论：家谱源头，没有爹，路径就是它自己的 ID 加上斜杠
		newPath = fmt.Sprintf("%d/", newComment.ID)
	} else {
		// 🎯 子孙评论：必须去数据库查出它爹的 Path，继承并延续家谱
		var parentComment pojo.Comment
		err := global.GVA_DB.WithContext(ctx).Where("id = ?", replyToID).First(&parentComment).Error
		if err != nil {
			// 如果极小概率下找不到爹（爹刚被删了），降级为一级评论，保证系统不崩
			global.LogCtx(ctx).Warnf("⚠️ 评论 [%d] 找不到父级 [%d]，强行降级为根评论", newComment.ID, replyToID)
			newPath = fmt.Sprintf("%d/", newComment.ID)
		} else {
			// 完美继承：爹的路径 + 自己的ID + / （例如爹是 1/，自己是 3，拼出来就是 1/3/）
			newPath = fmt.Sprintf("%s%d/", parentComment.Path, newComment.ID)
		}
	}

	// 4. ⚡ 第二次落盘：将算好的无敌家谱路径反写回 MySQL 的当前行
	err = global.GVA_DB.WithContext(ctx).Model(newComment).Update("path", newPath).Error
	if err != nil {
		global.LogCtx(ctx).Errorf("❌ [Comment] 评论 [%d] 物化路径组装大翻车: %v", newComment.ID, err)
		return errors.New("评论树路径装配发生异常")
	}

	global.LogCtx(ctx).Infof("✅ [Comment] 用户 [%d] 在视频 [%d] 下发表了评论，Path: %s", userID, videoID, newPath)
	return nil
}
