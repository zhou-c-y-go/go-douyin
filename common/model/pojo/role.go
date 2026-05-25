package pojo

import (
	"github.com/google/uuid"
	"time"
)

type User struct {
	Username    string    `form:"username" json:"username" gorm:"column:username"`
	Password    string    `form:"password" json:"password" gorm:"column:password"`
	ID          int64     `json:"id" gorm:"primaryKey;column:id" form:"id"`
	Email       string    `form:"email" json:"email" gorm:"column:email"`
	Telephone   string    `form:"telephone" json:"telephone" gorm:"column:telephone"`
	UUID        uuid.UUID `json:"uuid" gorm:"column:uuid"`
	Status      int64     `json:"status" gorm:"column:status" form:"status"` // 用户是否被冻结
	HeadImg     string    `json:"headImg" form:"head-img"`
	AuthorityId uint      `json:"authorityId" gorm:"default:888;comment:用户角色ID"` // 角色ID
	Authority   string    `json:"authority" gorm:"default:user;comment:用户角色"`
	GetLike     int       `json:"get_like" gorm:"default:0;comment:收到的点赞数"`
}

type Admin struct {
	Name        string    `json:"name"`
	Username    string    `json:"username"`
	Telephone   string    `json:"telephone"`
	AuthorityId uint      `json:"authority-id" gorm:"default:999"`
	Authority   string    `json:"authority" gorm:"default:admin"`
	Id          int       `json:"id" gorm:"primaryKey"`
	UpdateTime  time.Time `json:"updateTime" gorm:"autoUpdateTime"`
	CreatTime   time.Time `json:"creatTime" gorm:"autoCreateTime"`
}
