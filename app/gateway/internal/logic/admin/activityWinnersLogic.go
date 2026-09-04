package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ActivityWinnersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewActivityWinnersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActivityWinnersLogic {
	return &ActivityWinnersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ActivityWinnersLogic) ActivityWinners(req *types.IdPathReq) (*types.WinnersResp, error) {
	return l.svcCtx.App.WinnersAdmin(l.ctx, req.Id)
}
