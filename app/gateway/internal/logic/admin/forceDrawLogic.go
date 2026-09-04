package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ForceDrawLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForceDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForceDrawLogic {
	return &ForceDrawLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ForceDrawLogic) ForceDraw(req *types.IdPathReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.ForceDraw(l.ctx, req.Id); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
