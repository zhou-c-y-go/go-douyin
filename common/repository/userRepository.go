package repository

import (
	"Go_Project/common/model/pojo"
	"Go_Project/global"
	"Go_Project/utils"
	"errors"
	"fmt"
)

type UserRepository struct {
}

// QueryByID 持久化id查询
func (s *UserRepository) QueryByID(id int) pojo.User {
	var user pojo.User
	err := global.GVA_DB.Find(&user, id).Error
	if err != nil || user == (pojo.User{}) {
		return pojo.User{}
	} else {
		return user
	}
}

// QueryList 持久化用户列表查询
func (s *UserRepository) QueryList() []pojo.User {
	var users []pojo.User
	err := global.GVA_DB.Find(&users).Error
	if err != nil && users != nil {
		return nil
	} else {
		return users
	}
}

// Delete 删除用户
func (s *UserRepository) Delete(id int) int64 {
	var user pojo.User
	tx := global.GVA_DB.Unscoped().Where("id = ?", id).Delete(&user)
	if tx.Error != nil || user == (pojo.User{}) {
		global.SugaredLogger.
			Warn(tx.Error)
		return 0
	} else {
		return tx.RowsAffected
	}
}

// Validate 验证密码
func (s *UserRepository) Validate(u *pojo.User) (*pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.Where("username = ?", u.Username).Find(&user).Error
	if err != nil || user == (pojo.User{}) {
		global.SugaredLogger.Errorf("query error: username is %s, password is %s", u.Username)
		return nil, err
	} else {
		if ok := utils.BcryptCheck(u.Password, user.Password); ok {
			fmt.Println(ok)
			return &user, nil
		} else {
			return nil, errors.New("密码错误")
		}
	}
}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}
