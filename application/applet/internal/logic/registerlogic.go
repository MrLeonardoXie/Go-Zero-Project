// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2
package logic

import (
	"context"
	"errors"

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

}

func (l *RegisterLogic) CheckVerificationCode(req *types.RegisterRequest, code string) error {
	cachecode, err := getActivationCache(req.Mobile, l.svcCtx.BizRedis)
	if err != nil {
		logx.Errorf("getActivationCache mobile %s err:%v", req.Mobile, err)
	}
	if cachecode == "" {
		return errors.New("previous verification code is expired")
	}
	if code != cachecode {
		return errors.New("previous verification code not match")
	}

	return nil
}
