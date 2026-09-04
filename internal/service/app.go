package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"luckygo/internal/types"
	"luckygo/internal/ctxdata"
	"luckygo/internal/engine"
	"luckygo/internal/store"
	"luckygo/internal/tokenkit"
	"luckygo/internal/xerr"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"golang.org/x/crypto/bcrypt"
)

type Conf struct {
	JWTSecret    string
	JWTExpire    int64
	PublicBase   string
}

type App struct {
	Conf Conf
	DB   *store.Store
	Draw engine.RedisDraw
}

func New(c Conf, dsn string, r *redis.Redis) *App {
	return &App{
		Conf: c,
		DB:   store.New(dsn),
		Draw: engine.RedisDraw{R: r},
	}
}

func (a *App) RegisterTenant(ctx context.Context, req *types.RegisterTenantReq) (*types.LoginResp, error) {
	if err := validateAccount(req.Account, req.Password, req.TenantName); err != nil {
		return nil, err
	}
	tid, err := a.DB.InsertTenant(ctx, strings.TrimSpace(req.TenantName))
	if err != nil {
		if isDup(err) {
			return nil, xerr.ErrTenantTaken
		}
		return nil, logInternal(ctx, err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	nick := req.Nickname
	if nick == "" {
		nick = req.Account
	}
	uid, err := a.DB.InsertUser(ctx, store.User{
		TenantID: tid, Role: "admin", Account: req.Account, PasswordHash: string(hash), Nickname: nick,
	})
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	return a.loginResp(uid, tid, "admin", req.Account, nick, req.TenantName)
}

func (a *App) Login(ctx context.Context, req *types.LoginReq) (*types.LoginResp, error) {
	t, err := a.DB.TenantByName(ctx, strings.TrimSpace(req.TenantName))
	if err != nil {
		if store.IsNoRows(err) {
			return nil, xerr.ErrPassword
		}
		return nil, logInternal(ctx, err)
	}
	if t.Status != 1 {
		return nil, xerr.Forbid("租户已停用")
	}
	u, err := a.DB.UserByAccount(ctx, t.ID, req.Account)
	if err != nil {
		if store.IsNoRows(err) {
			return nil, xerr.ErrPassword
		}
		return nil, logInternal(ctx, err)
	}
	if u.Status != 1 || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		return nil, xerr.ErrPassword
	}
	return a.loginResp(u.ID, t.ID, u.Role, u.Account, u.Nickname, t.Name)
}

func (a *App) RegisterUser(ctx context.Context, req *types.UserRegisterReq) (*types.LoginResp, error) {
	if err := validateAccount(req.Account, req.Password, "ok"); err != nil {
		return nil, err
	}
	act, err := a.DB.ActivityByPublicID(ctx, req.PublicId)
	if err != nil {
		if store.IsNoRows(err) {
			return nil, xerr.ErrActivityNotFound
		}
		return nil, logInternal(ctx, err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	nick := req.Nickname
	if nick == "" {
		nick = req.Account
	}
	uid, err := a.DB.InsertUser(ctx, store.User{
		TenantID: act.TenantID, Role: "user", Account: req.Account, PasswordHash: string(hash), Nickname: nick,
	})
	if err != nil {
		if isDup(err) {
			return nil, xerr.ErrAccountTaken
		}
		return nil, logInternal(ctx, err)
	}
	return a.loginResp(uid, act.TenantID, "user", req.Account, nick, "")
}

func (a *App) loginResp(uid, tid uint64, role, account, nick, tenant string) (*types.LoginResp, error) {
	tok, err := tokenkit.Issue(a.Conf.JWTSecret, a.Conf.JWTExpire, uid, tid, role, account)
	if err != nil {
		return nil, xerr.Internal()
	}
	return &types.LoginResp{Token: tok, Role: role, Nickname: nick, Tenant: tenant}, nil
}

func (a *App) CreateActivity(ctx context.Context, req *types.CreateActivityReq) (*types.ActivityBrief, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req.Title == "" || (req.Mode != "instant" && req.Mode != "scheduled") {
		return nil, xerr.ErrInvalidParam
	}
	if req.MaxDrawsPerUser < 1 || req.MaxDrawsPerUser > 100 {
		return nil, xerr.Bad("每人次数须在 1-100")
	}
	if req.EndAt <= req.StartAt {
		return nil, xerr.Bad("结束时间必须晚于开始时间")
	}
	specs := prizeSpecsFromInput(0, req.Prizes)
	if err := engine.ValidatePrizes(specs, req.Mode); err != nil {
		return nil, err
	}
	tz := req.Timezone
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, xerr.Bad("时区不合法")
	}
	pub, err := engine.RandomPublicID()
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	prizes := make([]store.Prize, 0, len(req.Prizes))
	for _, p := range req.Prizes {
		prizes = append(prizes, store.Prize{
			Name: p.Name, Kind: p.Kind, Stock: p.Stock, Weight: p.Weight, ImageURL: p.ImageUrl,
		})
	}
	aid, err := a.DB.CreateActivityTx(ctx, store.Activity{
		TenantID:        id.TenantID,
		PublicID:        pub,
		Title:           req.Title,
		Mode:            req.Mode,
		Status:          "draft",
		Timezone:        tz,
		StartAt:         time.Unix(req.StartAt, 0).UTC(),
		EndAt:           time.Unix(req.EndAt, 0).UTC(),
		MaxDrawsPerUser: req.MaxDrawsPerUser,
		MaxEnrollments:  req.MaxEnrollments,
	}, prizes)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "create_activity", "activity", strconv.FormatUint(aid, 10), req.Title)
	return a.brief(ctx, id.TenantID, aid)
}

func (a *App) ListActivities(ctx context.Context, status string) (*types.ListActivityResp, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	list, err := a.DB.ListActivities(ctx, id.TenantID, status)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	out := make([]types.ActivityBrief, 0, len(list))
	for _, it := range list {
		out = append(out, a.toBrief(it))
	}
	return &types.ListActivityResp{List: out}, nil
}

func (a *App) GetActivityAdmin(ctx context.Context, aid uint64) (*types.ActivityDetail, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	return a.detail(ctx, id.TenantID, aid, true)
}

func (a *App) GetPublic(ctx context.Context, publicID string) (*types.ActivityDetail, error) {
	act, err := a.DB.ActivityByPublicID(ctx, publicID)
	if err != nil {
		if store.IsNoRows(err) {
			return nil, xerr.ErrActivityNotFound
		}
		return nil, logInternal(ctx, err)
	}
	a.maybeFlipStatus(ctx, act)
	return a.detail(ctx, act.TenantID, act.ID, false)
}

func (a *App) Publish(ctx context.Context, aid uint64) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	act, err := a.mustActivity(ctx, id.TenantID, aid)
	if err != nil {
		return err
	}
	if act.Status != "draft" {
		return xerr.ErrNotDraft
	}
	prizes, err := a.DB.Prizes(ctx, act.ID)
	if err != nil {
		return logInternal(ctx, err)
	}
	specs := prizeSpecs(prizes)
	now := time.Now().UTC().Unix()
	status := "published"
	if now >= act.StartAt.Unix() && now < act.EndAt.Unix() {
		status = "running"
	}
	if act.Mode == "instant" {
		if err := engine.ValidatePrizes(specs, "instant"); err != nil {
			return err
		}
		items, err := engine.BuildBucket(specs)
		if err != nil {
			return err
		}
		remain := map[string]int{}
		for _, p := range prizes {
			remain[strconv.FormatUint(p.ID, 10)] = p.Stock
		}
		if err := a.Draw.LoadBucket(ctx, act.ID, engine.Meta{
			Status: status, StartAt: act.StartAt.Unix(), EndAt: act.EndAt.Unix(), MaxDraws: act.MaxDrawsPerUser,
		}, items, remain); err != nil {
			return logInternal(ctx, err)
		}
	} else {
		if err := a.Draw.LoadBucket(ctx, act.ID, engine.Meta{
			Status: status, StartAt: act.StartAt.Unix(), EndAt: act.EndAt.Unix(), MaxDraws: 1,
		}, nil, nil); err != nil {
			return logInternal(ctx, err)
		}
	}
	if err := a.DB.CASStatus(ctx, id.TenantID, act.ID, "draft", status, act.Version, "publish"); err != nil {
		return xerr.ErrBadStatus
	}
	_ = a.Draw.ScheduleJob("start", act.ID, act.StartAt.Unix())
	_ = a.Draw.ScheduleJob("end", act.ID, act.EndAt.Unix())
	a.DB.Audit(ctx, id.TenantID, id.UserID, "publish", "activity", strconv.FormatUint(act.ID, 10), status)
	return nil
}

func (a *App) Pause(ctx context.Context, aid uint64) error {
	return a.flip(ctx, aid, "running", "paused", "pause")
}

func (a *App) Resume(ctx context.Context, aid uint64) error {
	return a.flip(ctx, aid, "paused", "running", "resume")
}

func (a *App) Cancel(ctx context.Context, aid uint64) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	act, err := a.mustActivity(ctx, id.TenantID, aid)
	if err != nil {
		return err
	}
	if act.Status == "drawn" || act.Status == "cancelled" {
		return xerr.ErrBadStatus
	}
	if err := a.DB.CASStatus(ctx, id.TenantID, act.ID, act.Status, "cancelled", act.Version, ""); err != nil {
		return xerr.ErrBadStatus
	}
	_ = a.Draw.SetStatus(act.ID, "cancelled")
	a.DB.Audit(ctx, id.TenantID, id.UserID, "cancel", "activity", strconv.FormatUint(act.ID, 10), nil)
	return nil
}

func (a *App) flip(ctx context.Context, aid uint64, from, to, action string) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	act, err := a.mustActivity(ctx, id.TenantID, aid)
	if err != nil {
		return err
	}
	if err := a.DB.CASStatus(ctx, id.TenantID, act.ID, from, to, act.Version, ""); err != nil {
		return xerr.ErrBadStatus
	}
	if err := a.Draw.SetStatus(act.ID, to); err != nil {
		return logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, action, "activity", strconv.FormatUint(act.ID, 10), nil)
	return nil
}

func (a *App) DrawOnce(ctx context.Context, req *types.DrawReq) (*types.DrawResp, error) {
	id, err := ctxdata.MustUser(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.IdempotencyKey) < 8 || len(req.IdempotencyKey) > 64 {
		return nil, xerr.ErrIdempotency
	}
	act, err := a.DB.ActivityByPublicID(ctx, req.PublicId)
	if err != nil {
		if store.IsNoRows(err) {
			return nil, xerr.ErrActivityNotFound
		}
		return nil, logInternal(ctx, err)
	}
	if act.TenantID != id.TenantID {
		return nil, xerr.ErrTenantMismatch
	}
	if act.Mode != "instant" {
		return nil, xerr.ErrWrongMode
	}
	a.maybeFlipStatus(ctx, act)
	bl, err := a.DB.Blacklisted(ctx, id.TenantID, id.UserID)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	if bl {
		return nil, xerr.ErrBlacklisted
	}
	res, err := a.Draw.Draw(act.ID, id.UserID, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	defer a.Draw.ClearInflight(act.ID, id.UserID)

	rec := store.DrawRecord{
		TenantID: act.TenantID, ActivityID: act.ID, UserID: id.UserID,
		PrizeID: res.Item.PrizeID, PrizeToken: res.Item.Token,
		IdempotencyKey: req.IdempotencyKey, Kind: res.Item.Kind, Status: "won",
	}
	if persistErr := a.persistDraw(ctx, rec); persistErr != nil {
		logx.WithContext(ctx).Errorf("persist draw token=%s err=%v", rec.PrizeToken, persistErr)
		_ = a.DB.InsertPersistFailure(ctx, rec, persistErr.Error())
	}
	prizes, _ := a.DB.Prizes(ctx, act.ID)
	name := prizeName(prizes, res.Item.PrizeID)
	code := ""
	if !res.Duplicate && res.Item.Kind != "thank_you" {
		code = a.issueRedeem(ctx, rec)
	}
	if res.Duplicate {
		if existing, e := a.DB.DrawByIdemp(ctx, act.ID, id.UserID, req.IdempotencyKey); e == nil && existing != nil {
			name = prizeName(prizes, existing.PrizeID)
		}
	}
	return &types.DrawResp{
		Result:      resultLabel(res.Item.Kind),
		PrizeId:     strconv.FormatUint(res.Item.PrizeID, 10),
		PrizeName:   name,
		Kind:        res.Item.Kind,
		PrizeToken:  res.Item.Token,
		RedeemCode:  code,
		RemainDraws: res.RemainDraws,
	}, nil
}

func (a *App) persistDraw(ctx context.Context, rec store.DrawRecord) error {
	var last error
	for i := 0; i < 3; i++ {
		err := a.DB.InsertDraw(ctx, rec)
		if err == nil || isDup(err) {
			return nil
		}
		last = err
		time.Sleep(time.Duration(i+1) * 40 * time.Millisecond)
	}
	return last
}

func (a *App) issueRedeem(ctx context.Context, rec store.DrawRecord) string {
	code, err := engine.RandomRedeemCode()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(code))
	r := store.Redemption{
		TenantID: rec.TenantID, ActivityID: rec.ActivityID, UserID: rec.UserID,
		PrizeID: rec.PrizeID, DrawRef: rec.PrizeToken,
		CodeHash: hex.EncodeToString(sum[:]), CodePrefix: code[:8], Status: "unused",
	}
	if err := a.DB.InsertRedemption(ctx, r); err != nil {
		if !isDup(err) {
			logx.WithContext(ctx).Errorf("redeem issue: %v", err)
		}
		return ""
	}
	return code
}

func (a *App) Enroll(ctx context.Context, publicID string) error {
	id, err := ctxdata.MustUser(ctx)
	if err != nil {
		return err
	}
	act, err := a.DB.ActivityByPublicID(ctx, publicID)
	if err != nil {
		if store.IsNoRows(err) {
			return xerr.ErrActivityNotFound
		}
		return logInternal(ctx, err)
	}
	if act.TenantID != id.TenantID {
		return xerr.ErrTenantMismatch
	}
	if act.Mode != "scheduled" {
		return xerr.ErrWrongMode
	}
	a.maybeFlipStatus(ctx, act)
	if act.Status != "running" && act.Status != "published" {
		if time.Now().UTC().Before(act.StartAt) {
			return xerr.ErrNotStarted
		}
		if !time.Now().UTC().Before(act.EndAt) {
			return xerr.ErrEnded
		}
	}
	bl, err := a.DB.Blacklisted(ctx, id.TenantID, id.UserID)
	if err != nil {
		return logInternal(ctx, err)
	}
	if bl {
		return xerr.ErrBlacklisted
	}
	if act.MaxEnrollments > 0 {
		n, err := a.DB.CountEnroll(ctx, act.ID)
		if err != nil {
			return logInternal(ctx, err)
		}
		if n >= int64(act.MaxEnrollments) {
			return xerr.ErrEnrollFull
		}
	}
	if err := a.DB.InsertEnrollment(ctx, id.TenantID, act.ID, id.UserID); err != nil {
		if isDup(err) {
			return xerr.ErrEnrolled
		}
		return logInternal(ctx, err)
	}
	return nil
}

func (a *App) MyPrizes(ctx context.Context) (*types.MyPrizesResp, error) {
	id, err := ctxdata.MustUser(ctx)
	if err != nil {
		return nil, err
	}
	list, err := a.DB.MyPrizes(ctx, id.TenantID, id.UserID)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	out := make([]types.MyPrizeItem, 0, len(list))
	for _, it := range list {
		code := ""
		if it.CodePrefix != "" {
			code = it.CodePrefix + "********"
		}
		out = append(out, types.MyPrizeItem{
			PrizeName: it.PrizeName, Kind: it.Kind, Status: it.Status,
			RedeemCode: code, WonAt: it.WonAt.Unix(), Activity: it.Title,
		})
	}
	return &types.MyPrizesResp{List: out}, nil
}

func (a *App) FillAddress(ctx context.Context, req *types.FillAddressReq) error {
	id, err := ctxdata.MustUser(ctx)
	if err != nil {
		return err
	}
	if req.PrizeToken == "" || req.ContactName == "" || req.Address == "" {
		return xerr.ErrInvalidParam
	}
	if err := a.DB.FillAddress(ctx, id.TenantID, id.UserID, req.PrizeToken, req.ContactName, req.ContactPhone, req.Address); err != nil {
		return xerr.NotFound("中奖记录不存在或已核销")
	}
	return nil
}

func (a *App) Redeem(ctx context.Context, code string) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(strings.ToUpper(code))
	if len(code) < 12 {
		return xerr.ErrRedeemed
	}
	sum := sha256.Sum256([]byte(code))
	if err := a.DB.RedeemCAS(ctx, id.TenantID, id.UserID, hex.EncodeToString(sum[:])); err != nil {
		return xerr.ErrRedeemed
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "redeem", "code", code[:8], nil)
	return nil
}

func (a *App) Blacklist(ctx context.Context, account, reason string) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	u, err := a.DB.UserByAccount(ctx, id.TenantID, account)
	if err != nil {
		return xerr.NotFound("用户不存在")
	}
	if err := a.DB.AddBlacklist(ctx, id.TenantID, u.ID, reason); err != nil {
		return logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "blacklist", "user", account, reason)
	return nil
}

func (a *App) Feed(ctx context.Context, publicID string) (*types.FeedResp, error) {
	act, err := a.DB.ActivityByPublicID(ctx, publicID)
	if err != nil {
		return nil, xerr.ErrActivityNotFound
	}
	list, err := a.DB.InstantWinners(ctx, act.ID, 40)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	return &types.FeedResp{List: mapWinners(list)}, nil
}

func (a *App) WinnersPublic(ctx context.Context, publicID string) (*types.WinnersResp, error) {
	act, err := a.DB.ActivityByPublicID(ctx, publicID)
	if err != nil {
		return nil, xerr.ErrActivityNotFound
	}
	if act.Mode != "scheduled" {
		feed, err := a.Feed(ctx, publicID)
		if err != nil {
			return nil, err
		}
		return &types.WinnersResp{List: feed.List}, nil
	}
	if act.Status != "drawn" {
		return nil, xerr.ErrNotDrawn
	}
	list, err := a.DB.ScheduledWinners(ctx, act.ID)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	return &types.WinnersResp{List: mapWinners(list)}, nil
}

func (a *App) WinnersAdmin(ctx context.Context, aid uint64) (*types.WinnersResp, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	act, err := a.mustActivity(ctx, id.TenantID, aid)
	if err != nil {
		return nil, err
	}
	if act.Mode == "scheduled" {
		list, err := a.DB.ScheduledWinners(ctx, act.ID)
		if err != nil {
			return nil, logInternal(ctx, err)
		}
		return &types.WinnersResp{List: mapWinners(list)}, nil
	}
	list, err := a.DB.InstantWinners(ctx, act.ID, 200)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	return &types.WinnersResp{List: mapWinners(list)}, nil
}

func (a *App) ForceDraw(ctx context.Context, aid uint64) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	act, err := a.mustActivity(ctx, id.TenantID, aid)
	if err != nil {
		return err
	}
	if err := a.RunScheduledDraw(ctx, act.ID); err != nil {
		return err
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "force_draw", "activity", strconv.FormatUint(act.ID, 10), nil)
	return nil
}

func (a *App) RunScheduledDraw(ctx context.Context, activityID uint64) error {
	act, err := a.DB.ActivityByIDOnly(ctx, activityID)
	if err != nil {
		return err
	}
	if act.Mode != "scheduled" {
		return xerr.ErrWrongMode
	}
	if act.Status == "drawn" {
		return nil
	}
	if time.Now().UTC().Add(2 * time.Second).Before(act.EndAt) {
		return xerr.Bad("未到开奖时间")
	}
	lockKey := "lg:drawlock:" + strconv.FormatUint(act.ID, 10)
	ok, err := a.Draw.R.SetnxEx(lockKey, "1", 120)
	if err != nil {
		return err
	}
	if !ok {
		return xerr.ErrBusy
	}
	defer func() { _, _ = a.Draw.R.Del(lockKey) }()

	if act.Status != "ended" {
		_ = a.DB.CASStatus(ctx, act.TenantID, act.ID, act.Status, "ended", act.Version, "")
		fresh, e := a.DB.ActivityByIDOnly(ctx, act.ID)
		if e == nil {
			act = fresh
		}
	}
	users, err := a.DB.EnrollUserIDs(ctx, act.ID)
	if err != nil {
		return err
	}
	prizes, err := a.DB.Prizes(ctx, act.ID)
	if err != nil {
		return err
	}
	seed, err := engine.RandomSeed()
	if err != nil {
		return err
	}
	wins, err := engine.AssignScheduled(users, prizeSpecs(prizes), seed)
	if err != nil {
		return err
	}
	rows := make([]struct {
		UserID, PrizeID uint64
		Token, Kind     string
		Rank            int
	}, 0, len(wins))
	for _, w := range wins {
		rows = append(rows, struct {
			UserID, PrizeID uint64
			Token, Kind     string
			Rank            int
		}{w.UserID, w.PrizeID, w.Token, w.Kind, w.Rank})
	}
	if err := a.DB.InsertWinnersTx(ctx, act.TenantID, act.ID, act.Version, seed, rows); err != nil {
		if strings.Contains(err.Error(), "cas_failed") {
			return nil
		}
		return err
	}
	_ = a.Draw.SetStatus(act.ID, "drawn")
	_ = a.DB.InsertDrawAudit(ctx, act.TenantID, act.ID, seed, users, wins)
	for _, w := range wins {
		if w.Kind == "thank_you" {
			continue
		}
		_ = a.issueRedeem(ctx, store.DrawRecord{
			TenantID: act.TenantID, ActivityID: act.ID, UserID: w.UserID,
			PrizeID: w.PrizeID, PrizeToken: w.Token, Kind: w.Kind,
		})
	}
	return nil
}

func (a *App) HandleDueJobs(ctx context.Context) {
	jobs, err := a.Draw.DueJobs(time.Now().Unix(), 50)
	if err != nil {
		logx.Errorf("due jobs: %v", err)
		return
	}
	for _, job := range jobs {
		kind, id, ok := splitJob(job)
		if !ok {
			_ = a.Draw.RemoveJob(job)
			continue
		}
		act, err := a.DB.ActivityByIDOnly(ctx, id)
		if err != nil {
			continue
		}
		switch kind {
		case "start":
			if act.Status == "published" && !time.Now().UTC().Before(act.StartAt) {
				_ = a.DB.CASStatus(ctx, act.TenantID, act.ID, "published", "running", act.Version, "")
				_ = a.Draw.SetStatus(act.ID, "running")
			}
		case "end":
			if time.Now().UTC().Before(act.EndAt.Add(-2 * time.Second)) {
				continue
			}
			if act.Status == "running" || act.Status == "published" || act.Status == "paused" {
				_ = a.DB.CASStatus(ctx, act.TenantID, act.ID, act.Status, "ended", act.Version, "")
				_ = a.Draw.SetStatus(act.ID, "ended")
			}
			if act.Mode == "scheduled" {
				_ = a.RunScheduledDraw(ctx, act.ID)
			}
		}
		_ = a.Draw.RemoveJob(job)
	}
	fails, err := a.DB.DuePersistFailures(ctx, 20)
	if err != nil {
		return
	}
	for _, rec := range fails {
		if a.persistDraw(ctx, rec) == nil {
			_ = a.DB.ResolvePersist(ctx, rec.PrizeToken)
		}
	}
}

func (a *App) maybeFlipStatus(ctx context.Context, act *store.Activity) {
	now := time.Now().UTC()
	if act.Status == "published" && !now.Before(act.StartAt) && now.Before(act.EndAt) {
		if a.DB.CASStatus(ctx, act.TenantID, act.ID, "published", "running", act.Version, "") == nil {
			_ = a.Draw.SetStatus(act.ID, "running")
			act.Status = "running"
			act.Version++
		}
	}
	if (act.Status == "running" || act.Status == "published") && !now.Before(act.EndAt) {
		if a.DB.CASStatus(ctx, act.TenantID, act.ID, act.Status, "ended", act.Version, "") == nil {
			_ = a.Draw.SetStatus(act.ID, "ended")
			act.Status = "ended"
		}
	}
}

func (a *App) mustActivity(ctx context.Context, tenantID, id uint64) (*store.Activity, error) {
	act, err := a.DB.ActivityByID(ctx, tenantID, id)
	if err != nil {
		if store.IsNoRows(err) {
			return nil, xerr.ErrActivityNotFound
		}
		return nil, logInternal(ctx, err)
	}
	return act, nil
}

func (a *App) brief(ctx context.Context, tenantID, id uint64) (*types.ActivityBrief, error) {
	act, err := a.mustActivity(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	b := a.toBrief(*act)
	return &b, nil
}

func (a *App) toBrief(act store.Activity) types.ActivityBrief {
	name := ""
	if t, err := a.DB.TenantByID(context.Background(), act.TenantID); err == nil {
		name = t.Name
	}
	return types.ActivityBrief{
		Id: act.ID, PublicId: act.PublicID, Title: act.Title, Mode: act.Mode, Status: act.Status,
		StartAt: act.StartAt.Unix(), EndAt: act.EndAt.Unix(), MaxDrawsPerUser: act.MaxDrawsPerUser,
		PlayUrl: strings.TrimRight(a.Conf.PublicBase, "/") + "/p/" + act.PublicID, TenantName: name,
	}
}

func (a *App) detail(ctx context.Context, tenantID, id uint64, admin bool) (*types.ActivityDetail, error) {
	act, err := a.mustActivity(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	prizes, err := a.DB.Prizes(ctx, act.ID)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	remain, _ := a.Draw.Remain(act.ID)
	views := make([]types.PrizeView, 0, len(prizes))
	for _, p := range prizes {
		r := p.Stock
		if n, ok := remain[p.ID]; ok {
			r = n
		}
		if !admin && p.Kind == "thank_you" {
			views = append(views, types.PrizeView{Id: p.ID, Name: p.Name, Kind: p.Kind, Stock: 0, Weight: 0, ImageUrl: p.ImageURL, Remain: 0})
			continue
		}
		views = append(views, types.PrizeView{
			Id: p.ID, Name: p.Name, Kind: p.Kind, Stock: p.Stock, Weight: p.Weight, ImageUrl: p.ImageURL, Remain: r,
		})
	}
	var pn, wn int64
	if act.Mode == "scheduled" {
		pn, _ = a.DB.CountEnroll(ctx, act.ID)
		if act.Status == "drawn" {
			w, _ := a.DB.ScheduledWinners(ctx, act.ID)
			wn = int64(len(w))
		}
	} else {
		pn, _ = a.DB.CountDraws(ctx, act.ID)
		wn, _ = a.DB.CountWins(ctx, act.ID)
	}
	return &types.ActivityDetail{
		ActivityBrief: a.toBrief(*act),
		Prizes:        views,
		ParticipantN:  pn,
		WinN:          wn,
	}, nil
}

func prizeSpecs(prizes []store.Prize) []engine.PrizeSpec {
	out := make([]engine.PrizeSpec, 0, len(prizes))
	for _, p := range prizes {
		out = append(out, engine.PrizeSpec{ID: p.ID, Name: p.Name, Kind: p.Kind, Stock: p.Stock, Weight: p.Weight})
	}
	return out
}

func prizeSpecsFromInput(startID uint64, in []types.PrizeInput) []engine.PrizeSpec {
	out := make([]engine.PrizeSpec, 0, len(in))
	for i, p := range in {
		out = append(out, engine.PrizeSpec{ID: startID + uint64(i) + 1, Name: p.Name, Kind: p.Kind, Stock: p.Stock, Weight: p.Weight})
	}
	return out
}

func prizeName(prizes []store.Prize, id uint64) string {
	for _, p := range prizes {
		if p.ID == id {
			return p.Name
		}
	}
	return ""
}

func resultLabel(kind string) string {
	if kind == "thank_you" {
		return "thanks"
	}
	return "win"
}

func mapWinners(list []store.WinnerRow) []types.WinnerItem {
	out := make([]types.WinnerItem, 0, len(list))
	for _, it := range list {
		out = append(out, types.WinnerItem{Nickname: maskName(it.Nickname), PrizeName: it.PrizeName, Kind: it.Kind, WonAt: it.WonAt.Unix()})
	}
	return out
}

func maskName(s string) string {
	rs := []rune(s)
	if len(rs) <= 1 {
		return "*"
	}
	return string(rs[0]) + "*"
}

func splitJob(job string) (string, uint64, bool) {
	i := strings.LastIndex(job, ":")
	if i <= 0 {
		return "", 0, false
	}
	id, err := strconv.ParseUint(job[i+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return job[:i], id, true
}

func validateAccount(account, password, tenant string) error {
	if len(strings.TrimSpace(tenant)) < 2 {
		return xerr.Bad("租户名太短")
	}
	if len(account) < 3 || len(account) > 32 {
		return xerr.Bad("账号长度 3-32")
	}
	if len(password) < 6 {
		return xerr.Bad("密码至少 6 位")
	}
	return nil
}

func isDup(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func logInternal(ctx context.Context, err error) error {
	logx.WithContext(ctx).Errorf("internal: %v", err)
	return xerr.Internal()
}
