package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RedeemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRedeemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RedeemLogic {
	return &RedeemLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RedeemLogic) Redeem(req *types.RedeemReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Redeem(l.ctx, req.Code); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
