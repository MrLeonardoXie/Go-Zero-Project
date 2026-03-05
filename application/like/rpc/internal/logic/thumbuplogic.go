package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"leonardo/application/like/rpc/internal/model"
	"leonardo/application/like/rpc/internal/svc"
	"leonardo/application/like/rpc/internal/types"
	"leonardo/application/like/rpc/service"
	"leonardo/pkg/deltalike"

	"github.com/zeromicro/go-zero/core/metric"
	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	idemReserveProcessing = int64(1)
	idemReserveDuplicate  = int64(0)
	idemReserveConflict   = int64(2)
	idemProcessingTTL     = 5
	idemDoneTTL           = 120
	countCacheTTL         = 3600
	stateCacheBaseTTL     = 600
	stateCacheJitterSec   = 30
)

var reserveThumbupLua = `
local key = KEYS[1]
local newType = ARGV[1]
local ttl = tonumber(ARGV[2])
local current = redis.call("GET", key)

if not current then
  redis.call("SET", key, "P:" .. newType, "EX", ttl)
  return 1
end

if current == ("P:" .. newType) or current == ("D:" .. newType) then
  return 0
end

return 2
`

var updateCountLua = `
local likeKey = KEYS[1]
local dislikeKey = KEYS[2]
local deltaLike = tonumber(ARGV[1])
local deltaDislike = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])

local currentLike = tonumber(redis.call("GET", likeKey) or "0")
local currentDislike = tonumber(redis.call("GET", dislikeKey) or "0")

local nextLike = currentLike + deltaLike
local nextDislike = currentDislike + deltaDislike
if nextLike < 0 then nextLike = 0 end
if nextDislike < 0 then nextDislike = 0 end

redis.call("SET", likeKey, tostring(nextLike), "EX", ttl)
redis.call("SET", dislikeKey, tostring(nextDislike), "EX", ttl)

return {nextLike, nextDislike}
`

var (
	thumbupStepDur = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: "like_rpc",
		Subsystem: "thumbup",
		Name:      "step_duration_ms",
		Help:      "thumbup step duration in ms",
		Labels:    []string{"step"},
		Buckets:   []float64{1, 2, 5, 10, 20, 50, 100, 250, 500, 1000},
	})
	thumbupStepErr = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: "like_rpc",
		Subsystem: "thumbup",
		Name:      "step_error_total",
		Help:      "thumbup step error count",
		Labels:    []string{"step"},
	})
)

type ThumbupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewThumbupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ThumbupLogic {
	return &ThumbupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ThumbupLogic) Thumbup(in *service.ThumbupRequest) (*service.ThumbupResponse, error) {
	idemKey := thumbupIdemKey(in.BizId, in.ObjId, in.UserId)
	idemState, idemErr := l.reserveThumbupIdem(idemKey, in.LikeType)
	if idemErr != nil {
		l.Logger.Errorf("[Thumbup] reserveThumbupIdem key: %s error: %v", idemKey, idemErr)
	} else if idemState == idemReserveDuplicate {
		return l.queryCountResp(l.ctx, in)
	}

	// 1) 查用户态记录（缓存前置 -> DB兜底）
	var (
		likeDelta, dislikeDelta int64
		oldLikeType            int32
		stateFound             bool
	)

	cacheCtx, cacheSpan := traceStep(l.ctx, "record_state_cache", in)
	cacheStart := time.Now()
	cachedLikeType, hit, cacheErr := l.getStateFromCache(cacheCtx, in.BizId, in.ObjId, in.UserId)
	observeStep("record_state_cache", time.Since(cacheStart), cacheErr)
	if cacheErr != nil {
		cacheSpan.RecordError(cacheErr)
	}
	cacheSpan.End()

	if hit {
		oldLikeType = cachedLikeType
		stateFound = true
	} else {
		recordCtx, recordSpan := traceStep(l.ctx, "record_lookup", in)
		recordStart := time.Now()
		record, err := l.svcCtx.LikeRecordModel.FindOneByBizIdObjIdUserId(recordCtx, in.BizId, in.ObjId, in.UserId)
		recordStepErr := err
		if recordStepErr == model.ErrNotFound {
			recordStepErr = nil
		}
		observeStep("record_lookup", time.Since(recordStart), recordStepErr)
		if err != nil && err != model.ErrNotFound {
			recordSpan.RecordError(err)
			recordSpan.End()
			if idemState == idemReserveProcessing {
				_, _ = l.svcCtx.BizRedis.DelCtx(context.Background(), idemKey)
			}
			return nil, err
		}
		recordSpan.End()

		if err == nil {
			oldLikeType = int32(record.LikeType)
			stateFound = true
		}
	}

	// 2) 幂等短路：同状态重复请求直接返回当前计数（不投递消息）
	if stateFound && oldLikeType == in.LikeType {
		return l.queryCountResp(l.ctx, in)
	}

	// 3) 计算 delta（基于旧状态 -> 新状态）
	if !stateFound {
		likeDelta, dislikeDelta = deltalike.CalcInsertDelta(in.LikeType)
	} else {
		likeDelta, dislikeDelta = deltalike.CalcSwitchDelta(oldLikeType, in.LikeType)
	}

	// 4) 读取当前计数基线（优先 Redis，缺失时回查 DB 并回填）
	state, err := l.queryCountResp(l.ctx, in)
	if err != nil {
		logx.Errorf("query Obj %d thumbup count failed, err:%v", in.ObjId, err)
		if idemState == idemReserveProcessing {
			_, _ = l.svcCtx.BizRedis.DelCtx(context.Background(), idemKey)
		}
		return nil, err
	}

	// 5) 同步 Push：保证“写意图可靠进入 MQ”，否则不要返回成功
	msg := &types.ThumbupMsg{
		BizId:    in.BizId,
		ObjId:    in.ObjId,
		UserId:   in.UserId,
		LikeType: in.LikeType,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		l.Logger.Errorf("[Thumbup] marshal msg: %v error: %v", msg, err)
		return nil, err
	}
	// 不要 GoSafe fire-and-forget：Push 失败要返回错误，让前端别乐观加数（或提示重试）
	pushCtx, pushSpan := traceStep(l.ctx, "kafka_push", in)
	_ = pushCtx
	pushStart := time.Now()
	if err := l.svcCtx.KqPusherClient.Push(string(data)); err != nil {
		observeStep("kafka_push", time.Since(pushStart), err)
		pushSpan.RecordError(err)
		pushSpan.End()
		l.Logger.Errorf("[Thumbup] kq push data: %s error: %v", data, err)
		if idemState == idemReserveProcessing {
			_, _ = l.svcCtx.BizRedis.DelCtx(context.Background(), idemKey)
		}
		return nil, err
	}
	observeStep("kafka_push", time.Since(pushStart), nil)
	pushSpan.End()

	// 回退到最早版本：不在 RPC 侧更新 Redis 计数缓存，避免引入额外一致性链路。
	//newLike, newDislike, err := l.applyCountCacheDelta(in.BizId, in.ObjId, likeDelta, dislikeDelta)
	//if err != nil {
	//	l.Logger.Errorf("[Thumbup] applyCountCacheDelta biz: %s obj: %d error: %v", in.BizId, in.ObjId, err)
	//	newLike = deltalike.MaxInt64(0, state.LikeNum+likeDelta)
	//	newDislike = deltalike.MaxInt64(0, state.DislikeNum+dislikeDelta)
	//}

	if idemState == idemReserveProcessing {
		_ = l.svcCtx.BizRedis.SetexCtx(context.Background(), idemKey, fmt.Sprintf("D:%d", in.LikeType), idemDoneTTL)
	}
	if setErr := l.setStateCache(context.Background(), in.BizId, in.ObjId, in.UserId, in.LikeType); setErr != nil {
		l.Logger.Errorf("[Thumbup] setStateCache biz:%s obj:%d user:%d err:%v", in.BizId, in.ObjId, in.UserId, setErr)
	}

	// 6) 返回“乐观后的计数”（回表基线 + delta）
	return &service.ThumbupResponse{
		BizId:      in.BizId,
		ObjId:      in.ObjId,
		LikeNum:    deltalike.MaxInt64(0, state.LikeNum+likeDelta),
		DislikeNum: deltalike.MaxInt64(0, state.DislikeNum+dislikeDelta),
	}, nil
}

func thumbupIdemKey(bizId string, objId, userId int64) string {
	return fmt.Sprintf("biz#thumbup#idem#%s#%d#%d", bizId, objId, userId)
}

func thumbupStateKey(bizId string, objId, userId int64) string {
	return fmt.Sprintf("biz#thumbup#state#%s#%d#%d", bizId, objId, userId)
}

func (l *ThumbupLogic) getStateFromCache(ctx context.Context, bizId string, objId, userId int64) (int32, bool, error) {
	val, err := l.svcCtx.BizRedis.GetCtx(ctx, thumbupStateKey(bizId, objId, userId))
	if err != nil {
		return 0, false, err
	}
	if val == "" {
		return 0, false, nil
	}
	parsed, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0, false, err
	}

	return int32(parsed), true, nil
}

func (l *ThumbupLogic) setStateCache(ctx context.Context, bizId string, objId, userId int64, likeType int32) error {
	ttl := stateCacheBaseTTL + int(userId%stateCacheJitterSec)
	return l.svcCtx.BizRedis.SetexCtx(ctx, thumbupStateKey(bizId, objId, userId), strconv.FormatInt(int64(likeType), 10), ttl)
}

func (l *ThumbupLogic) reserveThumbupIdem(key string, likeType int32) (int64, error) {
	v, err := l.svcCtx.BizRedis.EvalCtx(l.ctx, reserveThumbupLua, []string{key}, strconv.FormatInt(int64(likeType), 10), strconv.Itoa(idemProcessingTTL))
	if err != nil {
		return idemReserveConflict, err
	}

	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case string:
		parsed, parseErr := strconv.ParseInt(val, 10, 64)
		if parseErr != nil {
			return idemReserveConflict, parseErr
		}
		return parsed, nil
	default:
		return idemReserveConflict, fmt.Errorf("unexpected lua return type %T", v)
	}
}

func (l *ThumbupLogic) queryCountResp(ctx context.Context, in *service.ThumbupRequest) (*service.ThumbupResponse, error) {
	countCtx, countSpan := traceStep(ctx, "query_count", in)
	countStart := time.Now()
	countErr := error(nil)
	defer func() {
		observeStep("query_count", time.Since(countStart), countErr)
		countSpan.End()
	}()

	// 回退到最早版本：返回前直接回表查询 like_count。
	//if cached, ok, err := l.getCountFromCache(in.BizId, in.ObjId); err == nil && ok {
	//	return cached, nil
	//}

	count, err := l.svcCtx.LikeCountModel.FindOneByBizIdObjId(countCtx, in.BizId, in.ObjId)
	if err != nil {
		if err == model.ErrNotFound { //接口返回默认值，不是缓存穿透
			return &service.ThumbupResponse{
				BizId:      in.BizId,
				ObjId:      in.ObjId,
				LikeNum:    0,
				DislikeNum: 0,
			}, nil
		}

		countErr = err
		countSpan.RecordError(err)
		return nil, err
	}
	return &service.ThumbupResponse{
		BizId:      in.BizId,
		ObjId:      in.ObjId,
		LikeNum:    count.LikeNum,
		DislikeNum: count.DislikeNum,
	}, nil
}

func likeCountCacheKey(bizId string, objId int64) string {
	return fmt.Sprintf("biz#thumbup#count#like#%s#%d", bizId, objId)
}

func dislikeCountCacheKey(bizId string, objId int64) string {
	return fmt.Sprintf("biz#thumbup#count#dislike#%s#%d", bizId, objId)
}

func (l *ThumbupLogic) getCountFromCache(bizId string, objId int64) (*service.ThumbupResponse, bool, error) {
	keys := []string{likeCountCacheKey(bizId, objId), dislikeCountCacheKey(bizId, objId)}
	vals, err := l.svcCtx.BizRedis.MgetCtx(l.ctx, keys...)
	if err != nil {
		return nil, false, err
	}
	if len(vals) != 2 || vals[0] == "" || vals[1] == "" {
		return nil, false, nil
	}

	likeNum, err := strconv.ParseInt(vals[0], 10, 64)
	if err != nil {
		return nil, false, err
	}
	dislikeNum, err := strconv.ParseInt(vals[1], 10, 64)
	if err != nil {
		return nil, false, err
	}

	return &service.ThumbupResponse{
		BizId:      bizId,
		ObjId:      objId,
		LikeNum:    likeNum,
		DislikeNum: dislikeNum,
	}, true, nil
}

func (l *ThumbupLogic) setCountCache(bizId string, objId, likeNum, dislikeNum int64) error {
	likeKey := likeCountCacheKey(bizId, objId)
	dislikeKey := dislikeCountCacheKey(bizId, objId)
	if err := l.svcCtx.BizRedis.SetexCtx(l.ctx, likeKey, strconv.FormatInt(deltalike.MaxInt64(0, likeNum), 10), countCacheTTL); err != nil {
		return err
	}

	return l.svcCtx.BizRedis.SetexCtx(l.ctx, dislikeKey, strconv.FormatInt(deltalike.MaxInt64(0, dislikeNum), 10), countCacheTTL)
}

func (l *ThumbupLogic) applyCountCacheDelta(bizId string, objId, likeDelta, dislikeDelta int64) (int64, int64, error) {
	keys := []string{likeCountCacheKey(bizId, objId), dislikeCountCacheKey(bizId, objId)}
	v, err := l.svcCtx.BizRedis.EvalCtx(
		l.ctx,
		updateCountLua,
		keys,
		strconv.FormatInt(likeDelta, 10),
		strconv.FormatInt(dislikeDelta, 10),
		strconv.Itoa(countCacheTTL),
	)
	if err != nil {
		return 0, 0, err
	}

	vals, ok := v.([]any)
	if !ok || len(vals) != 2 {
		return 0, 0, fmt.Errorf("unexpected count lua return type %T", v)
	}

	likeNum, err := redisLuaInt64(vals[0])
	if err != nil {
		return 0, 0, err
	}
	dislikeNum, err := redisLuaInt64(vals[1])
	if err != nil {
		return 0, 0, err
	}

	return likeNum, dislikeNum, nil
}

func redisLuaInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, fmt.Errorf("unexpected lua number type %T", v)
	}
}

func traceStep(ctx context.Context, step string, in *service.ThumbupRequest) (context.Context, oteltrace.Span) {
	tracer := oteltrace.SpanFromContext(ctx).TracerProvider().Tracer("like-rpc-thumbup")
	stepCtx, span := tracer.Start(ctx, "like.rpc.thumbup."+step, oteltrace.WithSpanKind(oteltrace.SpanKindClient))
	span.SetAttributes(
		attribute.String("step", step),
		attribute.String("biz_id", in.BizId),
		attribute.Int64("obj_id", in.ObjId),
		attribute.Int64("user_id", in.UserId),
	)
	return stepCtx, span
}

func observeStep(step string, cost time.Duration, err error) {
	thumbupStepDur.Observe(cost.Milliseconds(), step)
	if err != nil {
		thumbupStepErr.Inc(step)
	}
}
