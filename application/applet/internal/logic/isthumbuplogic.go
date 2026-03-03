package logic

import (
	"context"
	"encoding/json"

	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"
	"leonardo/application/like/rpc/like"

	"github.com/zeromicro/go-zero/core/logx"
)

type IsThumbupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIsThumbupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IsThumbupLogic {
	return &IsThumbupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IsThumbupLogic) IsThumbup(req *types.IsThumbupRequest) (*types.IsThumbupResponse, error) {
	userID, err := l.ctx.Value("userId").(json.Number).Int64()
	if err != nil {
		return nil, err
	}

	resp, err := l.svcCtx.LikeRPC.IsThumbup(l.ctx, &like.IsThumbupRequest{
		BizId:    req.BizId,
		TargetId: req.TargetId,
		UserId:   userID,
	})
	if err != nil {
		return nil, err
	}

	userThumbup, ok := resp.UserThumbups[userID]
	if !ok || userThumbup == nil {
		return &types.IsThumbupResponse{}, nil
	}

	return &types.IsThumbupResponse{
		IsThumbup:   true,
		LikeType:    userThumbup.LikeType,
		ThumbupTime: userThumbup.ThumbupTime,
	}, nil
}
