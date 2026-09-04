package auth

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterTenantLogic {
	return &RegisterTenantLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RegisterTenantLogic) RegisterTenant(req *types.RegisterTenantReq) (*types.LoginResp, error) {
	return l.svcCtx.App.RegisterTenant(l.ctx, req)
}
