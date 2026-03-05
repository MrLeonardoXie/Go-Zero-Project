package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"
	followrpc "leonardo/application/follow/rpc/follow"

	"github.com/zeromicro/go-zero/core/logx"
)

type FansListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFansListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FansListLogic {
	return &FansListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FansListLogic) FansList(req *types.FansListRequest) (*types.FansListResponse, error) {
	userId, err := l.ctx.Value("userId").(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	resp, err := l.svcCtx.FollowRPC.FansList(l.ctx, &followrpc.FansListRequest{
		UserId:   userId,
		Cursor:   req.Cursor,
		PageSize: req.PageSize,
		Id:       req.Id,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.FansItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, types.FansItem{
			UserId:      item.UserId,
			FansUserId:  item.FansUserId,
			FollowCount: item.FollowCount,
			FansCount:   item.FansCount,
			CreateTime:  item.CreateTime,
		})
	}

	return &types.FansListResponse{
		Items:  items,
		Cursor: resp.Cursor,
		IsEnd:  resp.IsEnd,
		Id:     resp.Id,
	}, nil
}
