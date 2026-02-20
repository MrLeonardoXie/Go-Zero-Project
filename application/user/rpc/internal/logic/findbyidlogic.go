package logic

import (
	"context"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/internal/svc"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/service"
	"github.com/zeromicro/go-zero/core/logx"
)

type FindByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFindByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FindByIdLogic {
	return &FindByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FindByIdLogic) FindById(in *service.FindByIdRequest) (*service.FindByIdResponse, error) {
	// todo: add your logic here and delete this line - finished
	user, err := l.svcCtx.UserModel.FindOne(l.ctx, uint64(in.UserId))
	if err != nil {
		logx.Error("FindByIdLogic Id %s, FindOne error %v", in.UserId, err)
	}
	if user == nil {
		return &service.FindByIdResponse{}, nil
	}

	return &service.FindByIdResponse{
		UserId:   int64(user.Id),
		Username: user.Username,
		Avatar:   user.Avatar,
	}, nil
}
