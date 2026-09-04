package pub

import (
	"context"

	"luckygo/app/gateway/internal/svc"
	"luckygo/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublicFeedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublicFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublicFeedLogic {
	return &PublicFeedLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PublicFeedLogic) PublicFeed(req *types.PublicIdReq) (*types.FeedResp, error) {
	return l.svcCtx.App.Feed(l.ctx, req.PublicId)
}
