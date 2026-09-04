package user

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DrawLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DrawLogic {
	return &DrawLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DrawLogic) Draw(req *types.DrawReq) (*types.DrawResp, error) {
	return l.svcCtx.App.DrawOnce(l.ctx, req)
}
