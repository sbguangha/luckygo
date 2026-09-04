package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishActivityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishActivityLogic {
	return &PublishActivityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PublishActivityLogic) PublishActivity(req *types.IdPathReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Publish(l.ctx, req.Id); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
