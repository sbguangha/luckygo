package user

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MyPrizesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMyPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MyPrizesLogic {
	return &MyPrizesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MyPrizesLogic) MyPrizes() (*types.MyPrizesResp, error) {
	return l.svcCtx.App.MyPrizes(l.ctx)
}
