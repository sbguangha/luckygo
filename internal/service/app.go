package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"luckygo/internal/ctxdata"
	"luckygo/internal/engine"
	"luckygo/internal/store"
	"luckygo/internal/tokenkit"
	"luckygo/internal/types"
	"luckygo/internal/xerr"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"golang.org/x/crypto/bcrypt"
)

type Conf struct {
	JWTSecret string
	JWTExpire int64
	PublicBase string
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

// ---------- 认证 ----------

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

// ---------- 活动 CRUD ----------

func validMode(mode string) bool {
	return mode == "live" || mode == "scheduled"
}

func validRosterSource(s string) bool {
	return s == "import" || s == "register" || s == "both"
}

func (a *App) checkActivityInput(title, mode, rosterSource string, startAt, endAt int64, prizes []types.PrizeInput) (string, error) {
	if strings.TrimSpace(title) == "" || !validMode(mode) {
		return "", xerr.ErrInvalidParam
	}
	if rosterSource == "" {
		rosterSource = "both"
	}
	if !validRosterSource(rosterSource) {
		return "", xerr.Bad("名单来源不合法")
	}
	if endAt <= startAt {
		return "", xerr.Bad("结束时间必须晚于开始时间")
	}
	specs := prizeSpecsFromInput(prizes)
	if err := engine.ValidatePrizes(specs, mode); err != nil {
		return "", err
	}
	return rosterSource, nil
}

func storePrizesFromInput(in []types.PrizeInput) []store.Prize {
	prizes := make([]store.Prize, 0, len(in))
	for _, p := range in {
		perRound := p.PerRound
		if perRound <= 0 {
			perRound = 1
		}
		prizes = append(prizes, store.Prize{
			Name: p.Name, Kind: p.Kind, Stock: p.Stock, PerRoundCount: perRound, ImageURL: p.ImageUrl, IsAll: p.IsAll,
		})
	}
	return prizes
}

func (a *App) CreateActivity(ctx context.Context, req *types.CreateActivityReq) (*types.ActivityBrief, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	rosterSource, err := a.checkActivityInput(req.Title, req.Mode, req.RosterSource, req.StartAt, req.EndAt, req.Prizes)
	if err != nil {
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
	aid, err := a.DB.CreateActivityTx(ctx, store.Activity{
		TenantID:       id.TenantID,
		PublicID:       pub,
		Title:          req.Title,
		Mode:           req.Mode,
		RosterSource:   rosterSource,
		Status:         "draft",
		Timezone:       tz,
		StartAt:        time.Unix(req.StartAt, 0).UTC(),
		EndAt:          time.Unix(req.EndAt, 0).UTC(),
		MaxEnrollments: req.MaxEnrollments,
	}, storePrizesFromInput(req.Prizes))
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "create_activity", "activity", strconv.FormatUint(aid, 10), req.Title)
	return a.brief(ctx, id.TenantID, aid)
}

// UpdateActivity 整体更新草稿活动（含奖项替换）；发布后冻结。
func (a *App) UpdateActivity(ctx context.Context, req *types.UpdateActivityReq) (*types.ActivityBrief, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	rosterSource, err := a.checkActivityInput(req.Title, req.Mode, req.RosterSource, req.StartAt, req.EndAt, req.Prizes)
	if err != nil {
		return nil, err
	}
	tz := req.Timezone
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, xerr.Bad("时区不合法")
	}
	act, err := a.mustActivity(ctx, id.TenantID, req.Id)
	if err != nil {
		return nil, err
	}
	if act.Status != "draft" {
		return nil, xerr.ErrNotDraft
	}
	err = a.DB.UpdateActivityTx(ctx, store.Activity{
		ID:             act.ID,
		TenantID:       id.TenantID,
		Title:          req.Title,
		Mode:           req.Mode,
		RosterSource:   rosterSource,
		Timezone:       tz,
		StartAt:        time.Unix(req.StartAt, 0).UTC(),
		EndAt:          time.Unix(req.EndAt, 0).UTC(),
		MaxEnrollments: req.MaxEnrollments,
	}, storePrizesFromInput(req.Prizes))
	if err != nil {
		if strings.Contains(err.Error(), "cas_failed") {
			return nil, xerr.ErrNotDraft
		}
		return nil, logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "update_activity", "activity", strconv.FormatUint(act.ID, 10), req.Title)
	return a.brief(ctx, id.TenantID, act.ID)
}

// UpdateUiConfig 装修配置任何状态可改（不影响抽奖公正性）。
func (a *App) UpdateUiConfig(ctx context.Context, req *types.UpdateUiConfigReq) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	if _, err := a.mustActivity(ctx, id.TenantID, req.Id); err != nil {
		return err
	}
	if req.Config.RowCount < 0 || req.Config.RowCount > 100 {
		return xerr.Bad("列数须在 1-100")
	}
	b, err := json.Marshal(req.Config)
	if err != nil || len(b) > 8192 {
		return xerr.ErrInvalidParam
	}
	if err := a.DB.UpdateUiConfig(ctx, id.TenantID, req.Id, string(b)); err != nil {
		return logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "ui_config", "activity", strconv.FormatUint(req.Id, 10), nil)
	return nil
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
	if err := engine.ValidatePrizes(prizeSpecs(prizes), act.Mode); err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	status := "published"
	if now >= act.StartAt.Unix() && now < act.EndAt.Unix() {
		status = "running"
	}
	if err := a.Draw.LoadMeta(ctx, act.ID, engine.Meta{
		Status: status, StartAt: act.StartAt.Unix(), EndAt: act.EndAt.Unix(),
	}); err != nil {
		return logInternal(ctx, err)
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

// ---------- 参与者名单 ----------

func (a *App) ListParticipants(ctx context.Context, aid uint64) (*types.ParticipantsResp, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := a.mustActivity(ctx, id.TenantID, aid); err != nil {
		return nil, err
	}
	list, err := a.DB.ListParticipants(ctx, aid)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	won, err := a.DB.WonParticipantIDs(ctx, aid)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	out := make([]types.ParticipantItem, 0, len(list))
	for _, p := range list {
		out = append(out, types.ParticipantItem{
			Id: p.ID, Uid: p.Uid, Name: p.Name, Department: p.Department, Identity: p.Identity,
			AvatarUrl: p.AvatarURL, Source: p.Source, IsWin: won[p.ID], CreatedAt: p.CreatedAt.Unix(),
		})
	}
	return &types.ParticipantsResp{List: out}, nil
}

func (a *App) ImportParticipants(ctx context.Context, req *types.ImportParticipantsReq) (*types.ImportResp, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := a.mustActivity(ctx, id.TenantID, req.Id); err != nil {
		return nil, err
	}
	if len(req.Rows) == 0 || len(req.Rows) > 5000 {
		return nil, xerr.Bad("导入行数须为 1-5000")
	}
	seen := make(map[string]bool, len(req.Rows))
	failed := 0
	for _, r := range req.Rows {
		uid := strings.TrimSpace(r.Uid)
		name := strings.TrimSpace(r.Name)
		if uid == "" || name == "" || seen[uid] {
			failed++
			continue
		}
		seen[uid] = true
		if err := a.DB.UpsertParticipant(ctx, store.Participant{
			TenantID: id.TenantID, ActivityID: req.Id, Uid: uid, Name: name,
			Department: strings.TrimSpace(r.Department), Identity: strings.TrimSpace(r.Identity),
			AvatarURL: strings.TrimSpace(r.AvatarUrl), Source: "import",
		}); err != nil {
			logx.WithContext(ctx).Errorf("import participant uid=%s: %v", uid, err)
			failed++
		}
	}
	if err := a.Draw.BumpRosterVersion(req.Id); err != nil {
		logx.WithContext(ctx).Errorf("bump roster version: %v", err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "import_participants", "activity", strconv.FormatUint(req.Id, 10),
		fmt.Sprintf("total=%d failed=%d", len(req.Rows), failed))
	return &types.ImportResp{Total: len(req.Rows), Failed: failed}, nil
}

func (a *App) DeleteParticipant(ctx context.Context, aid, pid uint64) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	if _, err := a.mustActivity(ctx, id.TenantID, aid); err != nil {
		return err
	}
	won, err := a.DB.ParticipantWon(ctx, aid, pid)
	if err != nil {
		return logInternal(ctx, err)
	}
	if won {
		return xerr.Bad("该参与者已中奖，不可删除")
	}
	if err := a.DB.DeleteParticipant(ctx, id.TenantID, aid, pid); err != nil {
		if strings.Contains(err.Error(), "not_found") {
			return xerr.NotFound("参与者不存在")
		}
		return logInternal(ctx, err)
	}
	if err := a.Draw.BumpRosterVersion(aid); err != nil {
		logx.WithContext(ctx).Errorf("bump roster version: %v", err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "delete_participant", "participant", strconv.FormatUint(pid, 10), nil)
	return nil
}

// ---------- 现场大屏抽取 ----------

// LiveDraw 主持人点「停止」时调用：从奖项待抽池原子弹出 N 人并落库。
// 人数 = min(奖项单次抽取个数, 奖项剩余名额)。
func (a *App) LiveDraw(ctx context.Context, req *types.LiveDrawReq) (*types.LiveDrawResp, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.IdempotencyKey) < 8 || len(req.IdempotencyKey) > 64 {
		return nil, xerr.ErrIdempotency
	}
	act, err := a.mustActivity(ctx, id.TenantID, req.Id)
	if err != nil {
		return nil, err
	}
	if act.Mode != "live" {
		return nil, xerr.ErrWrongMode
	}
	a.maybeFlipStatus(ctx, act)
	prizes, err := a.DB.Prizes(ctx, act.ID)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	var prize *store.Prize
	for i := range prizes {
		if prizes[i].ID == req.PrizeId {
			prize = &prizes[i]
			break
		}
	}
	if prize == nil {
		return nil, xerr.NotFound("奖项不存在")
	}
	wonCounts, err := a.DB.CountWinsByPrize(ctx, act.ID)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	remaining := prize.Stock - int(wonCounts[prize.ID])
	if remaining <= 0 {
		return nil, xerr.Bad("该奖项已抽完")
	}
	count := prize.PerRoundCount
	if count > remaining {
		count = remaining
	}

	res, err := a.liveDrawWithRetry(ctx, act.ID, prize, count, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if res.Undone {
		return nil, xerr.ErrUndone
	}
	winners, userOf, err := a.liveWinnerViews(ctx, act, res.WinnerIDs)
	if err != nil {
		return nil, err
	}
	resp := &types.LiveDrawResp{
		DrawId: res.DrawId, PrizeId: prize.ID, PrizeName: prize.Name, Kind: prize.Kind,
		Winners: winners, Remain: remaining - len(res.WinnerIDs),
	}
	if res.Duplicate {
		return resp, nil
	}

	// 落库（重试 3 次，失败进补偿表；参与者已出池，与现有"不回池"策略一致）
	for _, w := range winners {
		tok, err := engine.RandomToken()
		if err != nil {
			return nil, logInternal(ctx, err)
		}
		rec := store.DrawRecord{
			TenantID: act.TenantID, ActivityID: act.ID, UserID: userOf[w.ParticipantId], ParticipantID: w.ParticipantId,
			PrizeID: prize.ID, PrizeToken: tok,
			IdempotencyKey: res.DrawId + ":" + strconv.FormatUint(w.ParticipantId, 10),
			Kind:           prize.Kind, Status: "won",
		}
		if persistErr := a.persistDraw(ctx, rec); persistErr != nil {
			logx.WithContext(ctx).Errorf("persist live draw token=%s err=%v", rec.PrizeToken, persistErr)
			_ = a.DB.InsertPersistFailure(ctx, rec, persistErr.Error())
		}
		// 注册来源中奖者签发兑换码（虚拟/实物奖）；导入名单无账号，现场线下核销
		if userOf[w.ParticipantId] > 0 {
			_ = a.issueRedeem(ctx, rec)
		}
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "live_draw", "activity", strconv.FormatUint(act.ID, 10),
		fmt.Sprintf("prize=%s count=%d drawId=%s", prize.Name, len(winners), res.DrawId))
	return resp, nil
}

// liveDrawWithRetry 名单池版本过期时从 MySQL 重建一次再重试。
func (a *App) liveDrawWithRetry(ctx context.Context, activityID uint64, prize *store.Prize, count int, idemKey string) (engine.LiveDrawResult, error) {
	ver, err := a.Draw.RosterVersion(activityID)
	if err != nil {
		return engine.LiveDrawResult{}, logInternal(ctx, err)
	}
	res, err := a.Draw.LiveDraw(activityID, prize.ID, idemKey, count, ver)
	if err != xerr.ErrStalePool {
		if err == xerr.ErrInsufficient && res.PoolSize > 0 {
			return res, xerr.Bad(fmt.Sprintf("可抽人数不足：剩 %d 人，本次需 %d 人", res.PoolSize, count))
		}
		return res, err
	}
	ids, err := a.DB.EligibleParticipantIDs(ctx, activityID, prize.ID, prize.IsAll)
	if err != nil {
		return res, logInternal(ctx, err)
	}
	if err := a.Draw.LiveRebuildPool(activityID, prize.ID, ver, ids); err != nil {
		return res, logInternal(ctx, err)
	}
	res, err = a.Draw.LiveDraw(activityID, prize.ID, idemKey, count, ver)
	if err == xerr.ErrInsufficient {
		return res, xerr.Bad(fmt.Sprintf("可抽人数不足：剩 %d 人，本次需 %d 人", res.PoolSize, count))
	}
	return res, err
}

// liveWinnerViews 按参与者 id 加载展示信息，并返回 participantId -> userId 映射（用于兑换码签发）。
func (a *App) liveWinnerViews(ctx context.Context, act *store.Activity, ids []uint64) ([]types.LiveWinner, map[uint64]uint64, error) {
	parts, err := a.DB.ParticipantsByIDs(ctx, act.ID, ids)
	if err != nil {
		return nil, nil, logInternal(ctx, err)
	}
	if len(parts) != len(ids) {
		return nil, nil, logInternal(ctx, fmt.Errorf("participants mismatch: got %d want %d", len(parts), len(ids)))
	}
	byID := make(map[uint64]store.Participant, len(parts))
	userOf := make(map[uint64]uint64, len(parts))
	for _, p := range parts {
		byID[p.ID] = p
		userOf[p.ID] = p.UserID
	}
	out := make([]types.LiveWinner, 0, len(ids))
	for _, id := range ids {
		p := byID[id]
		out = append(out, types.LiveWinner{
			ParticipantId: p.ID, Uid: p.Uid, Name: p.Name, Department: p.Department,
			Identity: p.Identity, AvatarUrl: p.AvatarURL,
		})
	}
	return out, userOf, nil
}

// UndoLiveDraw 主持人点「取消」：该批中奖记录翻为 undone，名单池随版本号重建自动回池。幂等。
func (a *App) UndoLiveDraw(ctx context.Context, req *types.LiveUndoReq) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	act, err := a.mustActivity(ctx, id.TenantID, req.Id)
	if err != nil {
		return err
	}
	if act.Mode != "live" {
		return xerr.ErrWrongMode
	}
	if req.DrawId == "" {
		return xerr.ErrIdempotency
	}
	if err := a.Draw.LiveUndo(act.ID, req.DrawId); err != nil && err != xerr.ErrUndone {
		return err
	}
	if _, err := a.DB.MarkLiveUndone(ctx, act.ID, req.DrawId); err != nil {
		return logInternal(ctx, err)
	}
	if err := a.Draw.BumpRosterVersion(act.ID); err != nil {
		logx.WithContext(ctx).Errorf("bump roster version: %v", err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "live_undo", "activity", strconv.FormatUint(act.ID, 10), req.DrawId)
	return nil
}

// ---------- 报名（C 端用户上球） ----------

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
	switch act.Mode {
	case "scheduled":
	case "live":
		if act.RosterSource == "import" {
			return xerr.Bad("该活动为名单导入制，无需报名")
		}
	default:
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
	// 报名即上球：写入统一名单
	u, err := a.DB.UserByID(ctx, id.TenantID, id.UserID)
	if err != nil {
		return logInternal(ctx, err)
	}
	if err := a.DB.UpsertParticipant(ctx, store.Participant{
		TenantID: id.TenantID, ActivityID: act.ID, Uid: fmt.Sprintf("U%d", id.UserID),
		Name: u.Nickname, Source: "register", UserID: id.UserID,
	}); err != nil {
		return logInternal(ctx, err)
	}
	if err := a.Draw.BumpRosterVersion(act.ID); err != nil {
		logx.WithContext(ctx).Errorf("bump roster version: %v", err)
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

// ---------- 核销 ----------

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

// OfflineRedeem 导入名单中奖者线下发奖：直接补一条已核销记录。
func (a *App) OfflineRedeem(ctx context.Context, aid uint64, prizeToken string) error {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return err
	}
	if _, err := a.mustActivity(ctx, id.TenantID, aid); err != nil {
		return err
	}
	if strings.TrimSpace(prizeToken) == "" {
		return xerr.ErrInvalidParam
	}
	if err := a.DB.MarkOfflineUsed(ctx, id.TenantID, prizeToken, id.UserID); err != nil {
		if isDup(err) {
			return xerr.ErrRedeemed
		}
		if strings.Contains(err.Error(), "not_found") {
			return xerr.NotFound("中奖记录不存在")
		}
		return logInternal(ctx, err)
	}
	a.DB.Audit(ctx, id.TenantID, id.UserID, "offline_redeem", "activity", strconv.FormatUint(aid, 10), prizeToken[:8])
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

// ---------- 播报与名单 ----------

func (a *App) Feed(ctx context.Context, publicID string) (*types.FeedResp, error) {
	act, err := a.DB.ActivityByPublicID(ctx, publicID)
	if err != nil {
		return nil, xerr.ErrActivityNotFound
	}
	if act.Mode == "scheduled" && act.Status != "drawn" {
		return &types.FeedResp{List: []types.WinnerItem{}}, nil
	}
	list, err := a.winnersByMode(ctx, act, 40)
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
	if act.Mode == "scheduled" && act.Status != "drawn" {
		return nil, xerr.ErrNotDrawn
	}
	list, err := a.winnersByMode(ctx, act, 200)
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	return &types.WinnersResp{List: mapWinners(list)}, nil
}

func (a *App) winnersByMode(ctx context.Context, act *store.Activity, limit int) ([]store.WinnerRow, error) {
	if act.Mode == "scheduled" {
		return a.DB.ScheduledWinners(ctx, act.ID)
	}
	return a.DB.LiveWinners(ctx, act.ID, limit)
}

// WinnersAdmin 管理端中奖名单（含工号/部门/核销状态，支持导出与线下核销）。
func (a *App) WinnersAdmin(ctx context.Context, aid uint64) (*types.AdminWinnersResp, error) {
	id, err := ctxdata.MustAdmin(ctx)
	if err != nil {
		return nil, err
	}
	act, err := a.mustActivity(ctx, id.TenantID, aid)
	if err != nil {
		return nil, err
	}
	var list []store.AdminWinnerRow
	if act.Mode == "scheduled" {
		list, err = a.DB.AdminScheduledWinners(ctx, act.ID)
	} else {
		list, err = a.DB.AdminLiveWinners(ctx, act.ID)
	}
	if err != nil {
		return nil, logInternal(ctx, err)
	}
	out := make([]types.AdminWinnerItem, 0, len(list))
	for _, w := range list {
		out = append(out, types.AdminWinnerItem{
			ParticipantId: w.ParticipantID, Uid: w.Uid, Name: w.Name, Department: w.Department,
			PrizeName: w.PrizeName, Kind: w.Kind, PrizeToken: w.PrizeToken, Source: w.Source,
			RedeemStatus: w.RedeemStatus, WonAt: w.WonAt.Unix(),
		})
	}
	return &types.AdminWinnersResp{List: out}, nil
}

// ---------- 定时开奖 ----------

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
	// 统一名单源：import 导入 + register 报名都在 participants 表
	participantIDs, err := a.DB.AllParticipantIDs(ctx, act.ID)
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
	wins, err := engine.AssignScheduled(participantIDs, prizeSpecs(prizes), seed)
	if err != nil {
		return err
	}
	parts, err := a.DB.ParticipantsByIDs(ctx, act.ID, participantIDs)
	if err != nil {
		return err
	}
	userOf := make(map[uint64]uint64, len(parts))
	for _, p := range parts {
		userOf[p.ID] = p.UserID
	}
	rows := make([]struct {
		UserID, ParticipantID, PrizeID uint64
		Token, Kind                    string
		Rank                           int
	}, 0, len(wins))
	for _, w := range wins {
		rows = append(rows, struct {
			UserID, ParticipantID, PrizeID uint64
			Token, Kind                    string
			Rank                           int
		}{userOf[w.ParticipantID], w.ParticipantID, w.PrizeID, w.Token, w.Kind, w.Rank})
	}
	if err := a.DB.InsertWinnersTx(ctx, act.TenantID, act.ID, act.Version, seed, rows); err != nil {
		if strings.Contains(err.Error(), "cas_failed") {
			return nil
		}
		return err
	}
	_ = a.Draw.SetStatus(act.ID, "drawn")
	_ = a.DB.InsertDrawAudit(ctx, act.TenantID, act.ID, seed, participantIDs, wins)
	for _, w := range wins {
		if uid := userOf[w.ParticipantID]; uid > 0 {
			_ = a.issueRedeem(ctx, store.DrawRecord{
				TenantID: act.TenantID, ActivityID: act.ID, UserID: uid, ParticipantID: w.ParticipantID,
				PrizeID: w.PrizeID, PrizeToken: w.Token, Kind: w.Kind,
			})
		}
	}
	return nil
}

// ---------- 后台任务 ----------

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

// ---------- 内部工具 ----------

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
		Id: act.ID, PublicId: act.PublicID, Title: act.Title, Mode: act.Mode, RosterSource: act.RosterSource,
		Status: act.Status, StartAt: act.StartAt.Unix(), EndAt: act.EndAt.Unix(),
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
	// 剩余名额：live 取中奖记录，scheduled 开奖后取名单
	wonByPrize := map[uint64]int64{}
	if act.Mode == "live" {
		wonByPrize, _ = a.DB.CountWinsByPrize(ctx, act.ID)
	} else if act.Status == "drawn" {
		wonByPrize, _ = a.DB.ScheduledWinCounts(ctx, act.ID)
	}
	views := make([]types.PrizeView, 0, len(prizes))
	for _, p := range prizes {
		views = append(views, types.PrizeView{
			Id: p.ID, Name: p.Name, Kind: p.Kind, Stock: p.Stock, PerRound: p.PerRoundCount,
			IsAll: p.IsAll, ImageUrl: p.ImageURL, Remain: p.Stock - int(wonByPrize[p.ID]),
		})
	}
	pn, _ := a.DB.CountParticipants(ctx, act.ID)
	var wn int64
	for _, n := range wonByPrize {
		wn += n
	}
	var ui *types.UiConfig
	if act.UiConfig != "" {
		var c types.UiConfig
		if json.Unmarshal([]byte(act.UiConfig), &c) == nil {
			ui = &c
		}
	}
	return &types.ActivityDetail{
		ActivityBrief: a.toBrief(*act),
		Prizes:        views,
		ParticipantN:  pn,
		WinN:          wn,
		UiConfig:      ui,
	}, nil
}

func prizeSpecs(prizes []store.Prize) []engine.PrizeSpec {
	out := make([]engine.PrizeSpec, 0, len(prizes))
	for _, p := range prizes {
		out = append(out, engine.PrizeSpec{ID: p.ID, Name: p.Name, Kind: p.Kind, Stock: p.Stock, PerRound: p.PerRoundCount, IsAll: p.IsAll})
	}
	return out
}

func prizeSpecsFromInput(in []types.PrizeInput) []engine.PrizeSpec {
	out := make([]engine.PrizeSpec, 0, len(in))
	for i, p := range in {
		perRound := p.PerRound
		if perRound <= 0 {
			perRound = 1
		}
		out = append(out, engine.PrizeSpec{ID: uint64(i) + 1, Name: p.Name, Kind: p.Kind, Stock: p.Stock, PerRound: perRound, IsAll: p.IsAll})
	}
	return out
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
