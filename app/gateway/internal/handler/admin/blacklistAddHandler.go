// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package admin

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"luckygo/app/gateway/internal/logic/admin"
	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"
)

func BlacklistAddHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlacklistReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := admin.NewBlacklistAddLogic(r.Context(), svcCtx)
		resp, err := l.BlacklistAdd(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
