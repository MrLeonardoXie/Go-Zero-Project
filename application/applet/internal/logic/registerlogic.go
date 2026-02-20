// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2
package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/MrLeonardoXie/Go-Zero-Project/application/applet/internal/svc"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/applet/internal/types"
	"github.com/MrLeonardoXie/Go-Zero-Project/application/user/rpc/user"
	"github.com/MrLeonardoXie/Go-Zero-Project/pkg/encrypt"
	"github.com/MrLeonardoXie/Go-Zero-Project/pkg/jwt"
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
	//1.trimspace and detect empty
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) == 0 {
		return nil, errors.New("name cannot be empty")
	}
	req.Mobile = strings.TrimSpace(req.Mobile)
	if len(req.Mobile) == 0 {
		return nil, errors.New("mobile cannot be empty")
	}
	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) == 0 {
		return nil, errors.New("password cannot be empty")
	}
	req.VerificationCode = strings.TrimSpace(req.VerificationCode)
	if len(req.VerificationCode) == 0 {
		return nil, errors.New("verification code cannot be empty")
	}

	//2.check verification code
	err = l.CheckVerificationCode(req, req.VerificationCode)
	if err != nil {
		return nil, err
	}

	//3.encrypy the mobile
	mobile, err := encrypt.EncMobile(req.Mobile)
	if err != nil {
		logx.Error("Encrypt mobile %s, error: %v", req.Mobile, err)
		return nil, err
	}
	//4.Get userinfo in database
	userinfo, err := l.svcCtx.UserRPC.FindByMobile(l.ctx, &user.FindByMobileRequest{Mobile: mobile}) //未找到返回userinfo.id = 0
	if err != nil {
		logx.Error("UserRPC.FindByMobile %s, error: %v", mobile, err)
		return nil, err
	}
	//5.check whether mobile has been used
	if userinfo != nil && userinfo.UserId != 0 {
		return nil, errors.New("this mobile is already registered")
	}
	//6.register
	regRet, err := l.svcCtx.UserRPC.Register(l.ctx, &user.RegisterRequest{
		Username: req.Name,
		Mobile:   req.Mobile,
		Password: req.Password,
	})
	//7.build tokens using userid, which will be used in log in
	token, err := jwt.BuildTokens(jwt.TokenOptions{
		AccessSecret: l.svcCtx.Config.Auth.AccessSecret,
		AccessExpire: l.svcCtx.Config.Auth.AccessExpire,
		Fields: map[string]interface{}{
			"userid": regRet.UserId,
		},
	})
	//8.delete verification codecache
	err = delActivationCache(req.Mobile, l.svcCtx.BizRedis)
	if err != nil {
		logx.Errorf("delActivationCache %s, error: %v", req.Mobile, err)
	}
	//9.return
	return &types.RegisterResponse{
		UserId: regRet.UserId,
		Token: types.Token{
			AccessToken:  token.AccessToken,
			AccessExpire: token.AccessExpire,
		},
	}, nil
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
