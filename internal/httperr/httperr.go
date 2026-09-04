package httperr

import (
	"context"
	"errors"
	"net/http"

	"luckygo/internal/xerr"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func Register() {
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		var ce *xerr.CodeError
		if errors.As(err, &ce) {
			status := ce.HTTPStatus
			if status == 0 {
				status = http.StatusBadRequest
			}
			return status, map[string]any{"code": ce.Code, "msg": ce.Msg}
		}
		return http.StatusInternalServerError, map[string]any{"code": 50000, "msg": "服务暂时不可用"}
	})
	httpx.SetOkHandler(func(ctx context.Context, v any) any {
		if v == nil {
			return map[string]any{"code": 0, "msg": "ok"}
		}
		return map[string]any{"code": 0, "msg": "ok", "data": v}
	})
}
