package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"
	"leonardo/application/like/rpc/like"

	"github.com/zeromicro/go-zero/core/logx"
)

type ThumbupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewThumbupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ThumbupLogic {
	return &ThumbupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ThumbupLogic) Thumbup(req *types.ThumbupRequest) (*types.ThumbupResponse, error) {
	userID, err := l.ctx.Value("userId").(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	resp, err := l.svcCtx.LikeRPC.Thumbup(l.ctx, &like.ThumbupRequest{
		BizId:    req.BizId,
		ObjId:    req.ObjId,
		UserId:   userID,
		LikeType: req.LikeType,
	})
	if err != nil {
		return nil, err
	}

	return &types.ThumbupResponse{
		BizId:      resp.BizId,
		ObjId:      resp.ObjId,
		LikeNum:    resp.LikeNum,
		DislikeNum: resp.DislikeNum,
	}, nil
}
