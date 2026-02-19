// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/applet/internal/svc"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/applet/internal/types"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserInfoLogic {
	return &UserInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserInfoLogic) UserInfo() (resp *types.UserInfoResponse, err error) {
	// todo: add your logic here and delete this line - finished
	// 从context中获取userid, 并判断是否是json.Number类型, 然后转换为Int64类型
	userId, err := l.ctx.Value(types.UserIdKey).(json.Number).Int64()
	if err != nil {
		logx.Errorf("get user id err : %v", err)
	}
	//该UserInfo不存在
	if userId == 0 {
		return &types.UserInfoResponse{}, nil
	}
	//通过FindById得到userinfo
	userinfo, err := l.svcCtx.UserRPC.FindById(l.ctx, &user.FindByIdRequest{
		UserId: userId,
	})
	if err != nil {
		logx.Errorf("get user info err : %v", err)
		return nil, err
	}
	//包装为UserInfoResponse类型
	return &types.UserInfoResponse{
		UserId:   userinfo.UserId,
		Username: userinfo.Username,
		Avatar:   userinfo.Avatar,
	}, nil
}
