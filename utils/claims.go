package utils

import (
	"Go_Project/common/model/request"
	"github.com/gin-gonic/gin"
)

// 解析token
func GetClaim(c *gin.Context) (*request.CustomClaims, error) {
	token := c.Request.Header.Get("x-token")
	j := NewJWT()
	claim, err := j.ParseToken(token)
	if err != nil {
		return nil, err
	} else {
		return claim, nil
	}
}

// 获取用户id
func GetUserID(c *gin.Context) int64 {
	if claims, exists := c.Get("claim"); !exists {
		if cl, err := GetClaim(c); err != nil {
			return 0
		} else {
			return cl.BaseClaims.Id
		}
	} else {
		cl1 := claims.(*request.CustomClaims)
		return cl1.BaseClaims.Id
	}
}

func GetUserAuthorityId(c *gin.Context) uint {
	if claims, exists := c.Get("claim"); !exists {
		if cl, err := GetClaim(c); err != nil {
			return 0
		} else {
			return cl.BaseClaims.AuthorityId
		}
	} else {
		cl1 := claims.(*request.CustomClaims)
		return cl1.AuthorityId
	}
}
