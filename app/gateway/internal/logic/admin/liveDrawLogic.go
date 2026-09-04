package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LiveDrawLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLiveDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LiveDrawLogic {
	return &LiveDrawLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *LiveDrawLogic) LiveDraw(req *types.LiveDrawReq) (*types.LiveDrawResp, error) {
	return l.svcCtx.App.LiveDraw(l.ctx, req)
}
