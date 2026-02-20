package logic

import (
	"context"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/svc"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/service"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/code"
	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterLogic) Register(in *service.RegisterRequest) (*service.RegisterResponse, error) {
	// todo: add your logic here and delete this line - finished
	if len(in.Username) == 0 {
		return nil, code.RegisterEmptyName
	}
	
	return &service.RegisterResponse{}, nil
}
