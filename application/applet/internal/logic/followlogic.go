package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"
	followrpc "leonardo/application/follow/rpc/follow"

	"github.com/zeromicro/go-zero/core/logx"
)

type FollowLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFollowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FollowLogic {
	return &FollowLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FollowLogic) Follow(req *types.FollowRequest) (*types.FollowResponse, error) {
	userId, err := l.ctx.Value("userId").(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	_, err = l.svcCtx.FollowRPC.Follow(l.ctx, &followrpc.FollowRequest{
		UserId:         userId,
		FollowedUserId: req.FollowedUserId,
	})
	if err != nil {
		return nil, err
	}

	return &types.FollowResponse{}, nil
}
