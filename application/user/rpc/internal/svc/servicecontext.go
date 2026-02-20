package svc

import (
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/config"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/model"
)

type ServiceContext struct {
	Config    config.Config
	UserModel model.UserModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
