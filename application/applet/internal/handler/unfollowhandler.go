package handler

import (
	"net/http"

	"leonardo/application/applet/internal/logic"
	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func UnFollowHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UnFollowRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewUnFollowLogic(r.Context(), svcCtx)
		resp, err := l.UnFollow(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
