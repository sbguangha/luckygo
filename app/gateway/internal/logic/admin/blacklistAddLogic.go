package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BlacklistAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBlacklistAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BlacklistAddLogic {
	return &BlacklistAddLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *BlacklistAddLogic) BlacklistAdd(req *types.BlacklistReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Blacklist(l.ctx, req.Account, req.Reason); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
