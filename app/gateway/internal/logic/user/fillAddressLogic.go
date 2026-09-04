package user

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FillAddressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFillAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FillAddressLogic {
	return &FillAddressLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *FillAddressLogic) FillAddress(req *types.FillAddressReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.FillAddress(l.ctx, req); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
