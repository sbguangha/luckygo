package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LiveDrawUndoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLiveDrawUndoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LiveDrawUndoLogic {
	return &LiveDrawUndoLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *LiveDrawUndoLogic) LiveDrawUndo(req *types.LiveUndoReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.UndoLiveDraw(l.ctx, req); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
