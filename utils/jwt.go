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

// ParseToken 验证解密函数
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
	token, err := jwt.ParseWithClaims(
		tokenString,
		&request.CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return j.SigningKey, nil
		},
	)
	// 1. 如果连 token 结构体没解析出来，直接断开
	if token == nil {
		return "", fmt.Errorf("令牌结构解析失败: %v", err)
	}
	// 2. 类型断言失败，必须明确返回自定义错误，绝不漏网
	claim, ok := token.Claims.(*request.CustomClaims)
	if !ok {
		return "", fmt.Errorf("令牌数据类型断言失败")
	}
	// 3. 检查令牌是否有效（签名错误或已过期）
	if !token.Valid {
		return "", fmt.Errorf("令牌已失效: %v", err)
	}
	// 4. 安全进行时间充值
	claim.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Hour))
	return j.CreateToken(*claim)
}

func (j *JWT) RenewToken(claims *request.CustomClaims) (string, error) {
	// 若token过期不超过10分钟则给它续签
	if time.Now().Unix()-claims.ExpiresAt.Unix() < 600 {
		return j.CreateToken(*claims)
	}
	return "", errors.New("登录已过期")
}

func (j *JWT) CreateTokenByOldToken(oldToken string, claims request.CustomClaims) (string, error) {
	v, err, _ := global.GLOB_Concurrency_Control.Do("JWT:"+oldToken, func() (interface{}, error) {
		return j.CreateToken(claims)
	})
	return v.(string), err
}
