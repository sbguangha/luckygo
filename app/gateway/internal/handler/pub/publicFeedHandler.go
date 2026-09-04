// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pub

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"luckygo/app/gateway/internal/logic/pub"
	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"
)

func PublicFeedHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PublicIdReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := pub.NewPublicFeedLogic(r.Context(), svcCtx)
		resp, err := l.PublicFeed(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
