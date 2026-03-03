package handler

import (
	"net/http"

	"leonardo/application/applet/internal/logic"
	"leonardo/application/applet/internal/svc"
	"leonardo/application/applet/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func IsThumbupHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.IsThumbupRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewIsThumbupLogic(r.Context(), svcCtx)
		resp, err := l.IsThumbup(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
