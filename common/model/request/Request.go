package request

import (
	"github.com/google/uuid"
)

type BaseClaims struct {
	AuthorityId uint // 用户角色ID
	UUID        uuid.UUID
	Id          int64
	UserName    string
}
