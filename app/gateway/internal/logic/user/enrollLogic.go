package user

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnrollLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEnrollLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnrollLogic {
	return &EnrollLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *EnrollLogic) Enroll(req *types.EnrollReq) (*types.OkResp, error) {
	if err := l.svcCtx.App.Enroll(l.ctx, req.PublicId); err != nil {
		return nil, err
	}
	return &types.OkResp{Ok: true}, nil
}
