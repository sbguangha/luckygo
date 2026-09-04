package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportParticipantsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewImportParticipantsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportParticipantsLogic {
	return &ImportParticipantsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ImportParticipantsLogic) ImportParticipants(req *types.ImportParticipantsReq) (*types.ImportResp, error) {
	return l.svcCtx.App.ImportParticipants(l.ctx, req)
}
