package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUiConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUiConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUiConfigLogic {
	return &UpdateUiConfigLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateUiConfigLogic) UpdateUiConfig(req *types.UpdateUiConfigReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.UpdateUiConfig(l.ctx, req); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
