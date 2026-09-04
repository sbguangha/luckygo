package pub

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublicWinnersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublicWinnersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublicWinnersLogic {
	return &PublicWinnersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PublicWinnersLogic) PublicWinners(req *types.PublicIdReq) (*types.WinnersResp, error) {
	return l.svcCtx.App.WinnersPublic(l.ctx, req.PublicId)
}
