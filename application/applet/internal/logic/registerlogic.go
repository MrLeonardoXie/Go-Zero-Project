// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2
package logic

import (
	"context"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/applet/internal/svc"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/applet/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	prefixActivation = "biz#activation#%s"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterRequest) (resp *types.RegisterResponse, err error) {
	// todo: add your logic here and delete this line
	return
}

func (l *RegisterLogic) CheckVerificationCode(req *types.RegisterRequest) (resp *types.RegisterResponse, err error) {
	getActivation
}
