package admin

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListActivitiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListActivitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListActivitiesLogic {
	return &ListActivitiesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListActivitiesLogic) ListActivities(req *types.ListActivityReq) (*types.ListActivityResp, error) {
	return l.svcCtx.App.ListActivities(l.ctx, req.Status)
}
