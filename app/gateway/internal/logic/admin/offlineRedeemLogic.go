package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OfflineRedeemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOfflineRedeemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OfflineRedeemLogic {
	return &OfflineRedeemLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OfflineRedeemLogic) OfflineRedeem(req *types.OfflineRedeemReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.OfflineRedeem(l.ctx, req.Id, req.PrizeToken); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
