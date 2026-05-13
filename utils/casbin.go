package utils

import (
	"fmt"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"log"
)

type CasbinService struct {
}

func (s CasbinService) InitConfig() *casbin.Enforcer {
	a, err := gormadapter.NewAdapter("mysql", "root:123456@tcp(127.0.0.1:3306)/casbin", true)
	if err != nil {
		fmt.Println(err)
	}
	models := `
		[request_definition]
		r = sub, obj, act
		
		[policy_definition]
		p = sub, obj, act
		
		[role_definition]
		g = _, _
		
		[policy_effect]
		e = some(where (p.eft == allow))
		
		[matchers]
		m = r.sub == p.sub && keyMatch2(r.obj,p.obj) && r.act == p.act`
	m, err := model.NewModelFromString(models)
	policies := [][]string{
		[]string{"888", "/api/v1/user/base/register", "PUT"},
		[]string{"888", "/api/v1/user/base/resetPwd", "PUT"},
		[]string{"888", "/api/v1/user/base/uploadImag/:id", "PUT"},
		[]string{"999", "/api/v1/user/base/:id", "GET"},
		[]string{"999", "/api/v1/user/base/:id", "DELETE"},
		[]string{"999", "/api/v1/user/base/user", "GET"},
		//[]string{"999", "/api/v1/user/base/", ""},
	}
	groupPolicies := [][]string{
		[]string{"user", "888"},
		[]string{"user", "999"},
	}
	if err != nil {
		log.Fatalf("model error %s", err)
	}
	e, err := casbin.NewEnforcer(m, a)
	addPolicies, err := e.AddPolicies(policies)
	if err != nil {
		log.Print(err, addPolicies)
	}
	rule, _ := e.AddGroupingPolicies(groupPolicies)
	if err != nil {
		log.Print(err, rule)
	}
	if err != nil {
		log.Fatalf("policy error %s", err)
	}
	return e
}
