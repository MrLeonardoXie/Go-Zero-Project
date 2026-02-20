package logic

import (
	"context"
	"time"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/model"
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
	ret, err := l.svcCtx.UserModel.Insert(l.ctx, &model.User{
		Username:   in.Username,
		Mobile:     in.Mobile,
		Avatar:     in.Avatar,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	})
	if err != nil {
		logx.Errorf("insert user error, %v", err)
	}
	userid, err := ret.LastInsertId()
	if err != nil {
		logx.Errorf("get userid err: %v", err)
	}

	return &service.RegisterResponse{
		UserId: userid,
	}, nil
}
