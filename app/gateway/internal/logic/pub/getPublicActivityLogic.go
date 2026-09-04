package pub

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPublicActivityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPublicActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPublicActivityLogic {
	return &GetPublicActivityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetPublicActivityLogic) GetPublicActivity(req *types.PublicIdReq) (*types.ActivityDetail, error) {
	return l.svcCtx.App.GetPublic(l.ctx, req.PublicId)
}
