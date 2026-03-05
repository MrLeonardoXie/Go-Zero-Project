package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"
	followrpc "leonardo/application/follow/rpc/follow"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnFollowLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnFollowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnFollowLogic {
	return &UnFollowLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnFollowLogic) UnFollow(req *types.UnFollowRequest) (*types.UnFollowResponse, error) {
	userId, err := l.ctx.Value("userId").(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	_, err = l.svcCtx.FollowRPC.UnFollow(l.ctx, &followrpc.UnFollowRequest{
		UserId:         userId,
		FollowedUserId: req.FollowedUserId,
	})
	if err != nil {
		return nil, err
	}

	return &types.UnFollowResponse{}, nil
}
