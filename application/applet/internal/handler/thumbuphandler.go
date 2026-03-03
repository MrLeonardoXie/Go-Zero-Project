package handler

import (
	"net/http"

	"leonardo/application/applet/internal/logic"
	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ThumbupHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ThumbupRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewThumbupLogic(r.Context(), svcCtx)
		resp, err := l.Thumbup(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
