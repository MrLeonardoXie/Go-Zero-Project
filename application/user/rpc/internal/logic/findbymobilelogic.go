package logic

import (
	"context"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/svc"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/service"

	"github.com/zeromicro/go-zero/core/logx"
)

type FindByMobileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFindByMobileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FindByMobileLogic {
	return &FindByMobileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FindByMobileLogic) FindByMobile(in *service.FindByMobileRequest) (*service.FindByMobileResponse, error) {
	// todo: add your logic here and delete this line - finished
	user, err := l.svcCtx.UserModel.FindByMobile(l.ctx, in.Mobile)
	if err != nil {
		logx.Errorf("Find by mobile %s error, %b", in.Mobile, err)
	}
	if user == nil {
		return &service.FindByMobileResponse{}, nil
	} //not found

	return &service.FindByMobileResponse{
		UserId:   int64(user.Id),
		Username: user.Username,
		Avatar:   user.Avatar,
	}, nil
}
