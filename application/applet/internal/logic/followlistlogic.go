package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"
	followrpc "leonardo/application/follow/rpc/follow"

	"github.com/zeromicro/go-zero/core/logx"
)

type FollowListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFollowListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FollowListLogic {
	return &FollowListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FollowListLogic) FollowList(req *types.FollowListRequest) (*types.FollowListResponse, error) {
	userId, err := l.ctx.Value("userId").(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	resp, err := l.svcCtx.FollowRPC.FollowList(l.ctx, &followrpc.FollowListRequest{
		UserId:   userId,
		Cursor:   req.Cursor,
		PageSize: req.PageSize,
		Id:       req.Id,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.FollowItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, types.FollowItem{
			Id:             item.Id,
			FollowedUserId: item.FollowedUserId,
			FansCount:      item.FansCount,
			CreateTime:     item.CreateTime,
		})
	}

	return &types.FollowListResponse{
		Items:  items,
		Cursor: resp.Cursor,
		IsEnd:  resp.IsEnd,
		Id:     resp.Id,
	}, nil
}
