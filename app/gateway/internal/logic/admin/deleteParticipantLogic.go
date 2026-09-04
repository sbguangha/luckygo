package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteParticipantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteParticipantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteParticipantLogic {
	return &DeleteParticipantLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteParticipantLogic) DeleteParticipant(req *types.ParticipantPathReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.DeleteParticipant(l.ctx, req.Id, req.Pid); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
