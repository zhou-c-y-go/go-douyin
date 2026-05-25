package request

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"mime/multipart"
)

type BaseClaims struct {
	AuthorityId uint // 用户角色ID
	UUID        uuid.UUID
	Id          int64
	UserName    string
}
type CustomClaims struct {
	BaseClaims
	jwt.RegisteredClaims
}
type Login struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	//AuthCode string `json:"authCode" binding:"authCode"`
	//AuCodeID int    `json:"auCodeID"`
}
type Register struct {
	Username    string               `form:"username" json:"username" binding:"required,min=1,max=20"`
	Password    string               `form:"password" json:"password" binding:"required,min=6,max=50"`
	ID          int64                `json:"id" form:"id"`
	Email       string               `form:"email" json:"email" binding:"required,email"`
	Telephone   string               `form:"telephone" json:"telephone" binding:"required,verifyMobileFormat"`
	UUID        uuid.UUID            `json:"uuid" form:"uuid"`
	Status      int64                `json:"status" form:"status"`
	HeadImg     multipart.FileHeader `json:"head-img" form:"head-img"`
	AuthorityId uint                 `json:"authorityId" form:"authorityId"`
}
