package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelActivityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCancelActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelActivityLogic {
	return &CancelActivityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CancelActivityLogic) CancelActivity(req *types.IdPathReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Cancel(l.ctx, req.Id); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
