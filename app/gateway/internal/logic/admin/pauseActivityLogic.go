package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PauseActivityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPauseActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PauseActivityLogic {
	return &PauseActivityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PauseActivityLogic) PauseActivity(req *types.IdPathReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Pause(l.ctx, req.Id); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
