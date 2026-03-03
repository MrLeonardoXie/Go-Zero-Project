package logic

import (
	"context"

	"leonardo/application/like/rpc/internal/model"
	"leonardo/application/like/rpc/internal/svc"
	"leonardo/application/like/rpc/service"

	"github.com/zeromicro/go-zero/core/logx"
)

type IsThumbupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIsThumbupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IsThumbupLogic {
	return &IsThumbupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *IsThumbupLogic) IsThumbup(in *service.IsThumbupRequest) (*service.IsThumbupResponse, error) {
	record, err := l.svcCtx.LikeRecordModel.FindOneByBizIdObjIdUserId(l.ctx, in.BizId, in.TargetId, in.UserId)
	if err != nil {
		if err == model.ErrNotFound {
			return &service.IsThumbupResponse{UserThumbups: map[int64]*service.UserThumbup{}}, nil
		}

		return nil, err
	}

	return &service.IsThumbupResponse{
		UserThumbups: map[int64]*service.UserThumbup{
			in.UserId: {
				UserId:      record.UserId,
				ThumbupTime: record.CreateTime.Unix(),
				LikeType:    int32(record.LikeType),
			},
		},
	}, nil
}
