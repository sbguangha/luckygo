package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResumeActivityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResumeActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResumeActivityLogic {
	return &ResumeActivityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ResumeActivityLogic) ResumeActivity(req *types.IdPathReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Resume(l.ctx, req.Id); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
