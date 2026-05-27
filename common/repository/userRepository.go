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

// FindUserByIdentifier 验证密码
func (s *UserRepository) FindUserByIdentifier(u *pojo.User) (*pojo.User, error) {
	var user pojo.User
	err := global.GVA_DB.Where("username = ? OR telephone = ? OR email = ?", u.Username, u.Username, u.Username).First(&user).Error
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

// Update 更新用户信息
func (s *UserRepository) Update(user *pojo.User) int64 {
	// 使用 Updates 传入结构体，GORM 会自动忽略零值，只更新有值的字段
	tx := global.GVA_DB.Model(&pojo.User{}).Where("id = ?", user.ID).Updates(user)

	if tx.Error != nil {
		global.SugaredLogger.Error(tx.Error)
		return 0
	} else {
		return tx.RowsAffected
	}
}

func (s *UserRepository) UpdateHeadImag(u *pojo.User, headerUrl string) int64 {
	tx := global.GVA_DB.Model(u).Where("id = ?", u.ID).Update("head_img", headerUrl)
	if tx.Error != nil {
		global.SugaredLogger.Error(tx.Error)
		return 0
	} else {
		return tx.RowsAffected
	}
}
