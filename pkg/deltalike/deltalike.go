package deltalike

import "strings"

const (
	LikeTypeThumbup   = int32(0)
	LikeTypeThumbdown = int32(1)
)

func CalcInsertDelta(likeType int32) (int64, int64) {
	if likeType == LikeTypeThumbup {
		return 1, 0
	}

	return 0, 1
}

func CalcSwitchDelta(fromType, toType int32) (int64, int64) {
	if fromType == toType {
		return 0, 0
	}
	if fromType == LikeTypeThumbup && toType == LikeTypeThumbdown {
		return -1, 1
	}

	return 1, -1
}

/* 防止计数小于0 */
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}

	return b
}

func IsDuplicateEntry(err error) bool {
	//依据MySQL报错：Error 1062 (23000): Duplicate entry 'xxx' for key 'uk_biz_obj_uid
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate entry")
}
