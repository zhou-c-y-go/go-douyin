package utils

import (
	"Go_Project/common/model/request"
	"Go_Project/global"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

// JWT 定义一个jwt对象
type JWT struct {
	// 声明签名信息
	SigningKey []byte
}

// NewJWT 初始化jwt对象
func NewJWT() *JWT {
	return &JWT{
		[]byte(global.GLOB_CONFIG.JWT.SigningKey),
	}
}

func (j *JWT) CreateClaim(baseClaim request.BaseClaims) request.CustomClaims {
	claim := request.CustomClaims{
		BaseClaims: baseClaim,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),                      // 签发时间
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),   // 过期时间
			NotBefore: jwt.NewNumericDate(time.Now().Add(1 * time.Second)), // 在该什么时间，该jwt都是不可用
			Subject:   "login",
		},
	}
	return claim
}

func (j *JWT) CreateToken(claims request.CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(NewJWT().SigningKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (j *JWT) ParseToken(tokenString string) (*request.CustomClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&request.CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return j.SigningKey, nil
		},
	)
	if token != nil {
		if claims, ok := token.Claims.(*request.CustomClaims); ok && token.Valid {
			return claims, nil
		} else {
			return nil, err
		}
	}
	return nil, err
}

func (j *JWT) UpdateToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &request.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	})
	if claim, ok := token.Claims.(*request.CustomClaims); ok && token.Valid {
		claim.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Hour))
		return j.CreateToken(*claim)
	}
	return "", fmt.Errorf("错误 %v", err)
}
func (j *JWT) RenewToken(claims *request.CustomClaims) (string, error) {
	// 若token过期不超过10分钟则给它续签
	if j.withinLimit(claims.IssuedAt.Unix(), 600) {
		return j.CreateToken(*claims)
	}
	return "", errors.New("登录已过期")
}

// 计算过期时间是否超过l
func (j *JWT) withinLimit(s int64, l int64) bool {
	e := time.Now().Unix()
	return e-s < l
}

func (j *JWT) CreateTokenByOldToken(oldToken string, claims request.CustomClaims) (string, error) {
	v, err, _ := global.GLOB_Concurrency_Control.Do("JWT:"+oldToken, func() (interface{}, error) {
		return j.CreateToken(claims)
	})
	return v.(string), err
}
