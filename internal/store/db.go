package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Store struct {
	conn sqlx.SqlConn
}

func New(dsn string) *Store {
	return &Store{conn: sqlx.NewMysql(dsn)}
}

func (s *Store) Ping(ctx context.Context) error {
	var n int
	return s.conn.QueryRowCtx(ctx, &n, "SELECT 1")
}

func (s *Store) InsertTenant(ctx context.Context, name string) (uint64, error) {
	res, err := s.conn.ExecCtx(ctx, "INSERT INTO tenants (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (s *Store) TenantByID(ctx context.Context, id uint64) (*Tenant, error) {
	var t Tenant
	err := s.conn.QueryRowCtx(ctx, &t, "SELECT id,name,status,created_at FROM tenants WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) TenantByName(ctx context.Context, name string) (*Tenant, error) {
	var t Tenant
	err := s.conn.QueryRowCtx(ctx, &t, "SELECT id,name,status,created_at FROM tenants WHERE name=?", name)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) InsertUser(ctx context.Context, u User) (uint64, error) {
	res, err := s.conn.ExecCtx(ctx, `INSERT INTO users (tenant_id,role,account,password_hash,nickname)
		VALUES (?,?,?,?,?)`, u.TenantID, u.Role, u.Account, u.PasswordHash, u.Nickname)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (s *Store) UserByAccount(ctx context.Context, tenantID uint64, account string) (*User, error) {
	var u User
	err := s.conn.QueryRowCtx(ctx, &u, `SELECT id,tenant_id,role,account,password_hash,nickname,status
		FROM users WHERE tenant_id=? AND account=?`, tenantID, account)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByID(ctx context.Context, tenantID, userID uint64) (*User, error) {
	var u User
	err := s.conn.QueryRowCtx(ctx, &u, `SELECT id,tenant_id,role,account,password_hash,nickname,status
		FROM users WHERE tenant_id=? AND id=?`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

const activityCols = `id,tenant_id,public_id,title,mode,roster_source,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments,IFNULL(ui_config,'') AS ui_config,version,published_at,drawn_at,draw_seed`

func (s *Store) CreateActivityTx(ctx context.Context, a Activity, prizes []Prize) (uint64, error) {
	var id uint64
	err := s.conn.TransactCtx(ctx, func(_ context.Context, session sqlx.Session) error {
		res, err := session.Exec(`INSERT INTO activities
			(tenant_id,public_id,title,mode,roster_source,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			a.TenantID, a.PublicID, a.Title, a.Mode, a.RosterSource, a.Status, a.Timezone, a.StartAt, a.EndAt, a.MaxDrawsPerUser, a.MaxEnrollments)
		if err != nil {
			return err
		}
		lid, _ := res.LastInsertId()
		id = uint64(lid)
		for i, p := range prizes {
			_, err = session.Exec(`INSERT INTO prizes
				(tenant_id,activity_id,name,kind,stock,per_round_count,weight,image_url,is_all,sort_order)
				VALUES (?,?,?,?,?,?,?,?,?,?)`,
				a.TenantID, id, p.Name, p.Kind, p.Stock, p.PerRoundCount, p.Weight, p.ImageURL, p.IsAll, i)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

// UpdateActivityTx 仅草稿状态可改：更新基础字段并整体替换奖项。
func (s *Store) UpdateActivityTx(ctx context.Context, a Activity, prizes []Prize) error {
	return s.conn.TransactCtx(ctx, func(_ context.Context, session sqlx.Session) error {
		res, err := session.Exec(`UPDATE activities
			SET title=?, mode=?, roster_source=?, timezone=?, start_at=?, end_at=?, max_enrollments=?, version=version+1
			WHERE id=? AND tenant_id=? AND status='draft'`,
			a.Title, a.Mode, a.RosterSource, a.Timezone, a.StartAt, a.EndAt, a.MaxEnrollments, a.ID, a.TenantID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("cas_failed")
		}
		if _, err = session.Exec(`DELETE FROM prizes WHERE activity_id=?`, a.ID); err != nil {
			return err
		}
		for i, p := range prizes {
			if _, err = session.Exec(`INSERT INTO prizes
				(tenant_id,activity_id,name,kind,stock,per_round_count,weight,image_url,is_all,sort_order)
				VALUES (?,?,?,?,?,?,?,?,?,?)`,
				a.TenantID, a.ID, p.Name, p.Kind, p.Stock, p.PerRoundCount, p.Weight, p.ImageURL, p.IsAll, i); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) UpdateUiConfig(ctx context.Context, tenantID, id uint64, uiConfig string) error {
	_, err := s.conn.ExecCtx(ctx, `UPDATE activities SET ui_config=? WHERE id=? AND tenant_id=?`, uiConfig, id, tenantID)
	return err
}

func (s *Store) ListActivities(ctx context.Context, tenantID uint64, status string) ([]Activity, error) {
	q := `SELECT ` + activityCols + ` FROM activities WHERE tenant_id=?`
	args := []any{tenantID}
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	q += " ORDER BY id DESC"
	var list []Activity
	err := s.conn.QueryRowsCtx(ctx, &list, q, args...)
	return list, err
}

func (s *Store) ActivityByID(ctx context.Context, tenantID, id uint64) (*Activity, error) {
	var a Activity
	err := s.conn.QueryRowCtx(ctx, &a, `SELECT `+activityCols+` FROM activities WHERE id=? AND tenant_id=?`, id, tenantID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ActivityByPublicID(ctx context.Context, publicID string) (*Activity, error) {
	var a Activity
	err := s.conn.QueryRowCtx(ctx, &a, `SELECT `+activityCols+` FROM activities WHERE public_id=?`, publicID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ActivityByIDOnly(ctx context.Context, id uint64) (*Activity, error) {
	var a Activity
	err := s.conn.QueryRowCtx(ctx, &a, `SELECT `+activityCols+` FROM activities WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) Prizes(ctx context.Context, activityID uint64) ([]Prize, error) {
	var list []Prize
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT id,tenant_id,activity_id,name,kind,stock,per_round_count,weight,image_url,is_all,sort_order
		FROM prizes WHERE activity_id=? ORDER BY sort_order,id`, activityID)
	return list, err
}

func (s *Store) CASStatus(ctx context.Context, tenantID, id uint64, from, to string, version int, extra string) error {
	q := `UPDATE activities SET status=?, version=version+1`
	args := []any{to}
	if extra == "publish" {
		q += ", published_at=?"
		args = append(args, time.Now().UTC())
	}
	if extra == "drawn" {
		q += ", drawn_at=?"
		args = append(args, time.Now().UTC())
	}
	q += " WHERE id=? AND tenant_id=? AND status=? AND version=?"
	args = append(args, id, tenantID, from, version)
	res, err := s.conn.ExecCtx(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cas_failed")
	}
	return nil
}

func (s *Store) InsertDraw(ctx context.Context, r DrawRecord) error {
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO draw_records
		(tenant_id,activity_id,user_id,participant_id,prize_id,prize_token,idempotency_key,kind,status)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		r.TenantID, r.ActivityID, r.UserID, r.ParticipantID, r.PrizeID, r.PrizeToken, r.IdempotencyKey, r.Kind, r.Status)
	return err
}

func (s *Store) InsertPersistFailure(ctx context.Context, r DrawRecord, errMsg string) error {
	payload, _ := json.Marshal(r)
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO persist_failures
		(tenant_id,activity_id,user_id,participant_id,prize_id,prize_token,idempotency_key,kind,payload,retry_count,last_error,next_retry_at)
		VALUES (?,?,?,?,?,?,?,?,?,0,?,?)
		ON DUPLICATE KEY UPDATE last_error=VALUES(last_error), retry_count=retry_count+1, next_retry_at=VALUES(next_retry_at)`,
		r.TenantID, r.ActivityID, r.UserID, r.ParticipantID, r.PrizeID, r.PrizeToken, r.IdempotencyKey, r.Kind, payload, errMsg, time.Now().UTC().Add(5*time.Second))
	return err
}

func (s *Store) DuePersistFailures(ctx context.Context, limit int) ([]DrawRecord, error) {
	var rows []struct {
		Payload []byte `db:"payload"`
	}
	err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT payload FROM persist_failures
		WHERE resolved_at IS NULL AND next_retry_at<=? ORDER BY id LIMIT ?`, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]DrawRecord, 0, len(rows))
	for _, row := range rows {
		var r DrawRecord
		if json.Unmarshal(row.Payload, &r) == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) ResolvePersist(ctx context.Context, token string) error {
	_, err := s.conn.ExecCtx(ctx, `UPDATE persist_failures SET resolved_at=? WHERE prize_token=? AND resolved_at IS NULL`, time.Now().UTC(), token)
	return err
}

// MarkLiveUndone 取消某一批大屏抽取：按批次幂等键前缀把 won 记录翻成 undone，返回受影响行数。
func (s *Store) MarkLiveUndone(ctx context.Context, activityID uint64, drawId string) (int64, error) {
	res, err := s.conn.ExecCtx(ctx, `UPDATE draw_records SET status='undone'
		WHERE activity_id=? AND status='won' AND idempotency_key LIKE ?`, activityID, drawId+":%")
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) InsertEnrollment(ctx context.Context, tenantID, activityID, userID uint64) error {
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO enrollments (tenant_id,activity_id,user_id) VALUES (?,?,?)`, tenantID, activityID, userID)
	return err
}

func (s *Store) CountEnroll(ctx context.Context, activityID uint64) (int64, error) {
	var n int64
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM enrollments WHERE activity_id=?`, activityID)
	return n, err
}

// ---------- 参与者名单 ----------

// UpsertParticipant 按 (activity_id, uid) 幂等写入；报名上球与 Excel 导入共用。
func (s *Store) UpsertParticipant(ctx context.Context, p Participant) error {
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO participants
		(tenant_id,activity_id,uid,name,department,identity,avatar_url,source,user_id)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE name=VALUES(name), department=VALUES(department),
			identity=VALUES(identity), avatar_url=IF(VALUES(avatar_url)='', avatar_url, VALUES(avatar_url))`,
		p.TenantID, p.ActivityID, p.Uid, p.Name, p.Department, p.Identity, p.AvatarURL, p.Source, p.UserID)
	return err
}

func (s *Store) ListParticipants(ctx context.Context, activityID uint64) ([]Participant, error) {
	var list []Participant
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT id,tenant_id,activity_id,uid,name,department,identity,avatar_url,source,user_id,created_at
		FROM participants WHERE activity_id=? ORDER BY id`, activityID)
	return list, err
}

func (s *Store) ParticipantsByIDs(ctx context.Context, activityID uint64, ids []uint64) ([]Participant, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	marks := strings.Repeat("?,", len(ids))
	marks = marks[:len(marks)-1]
	args := make([]any, 0, len(ids)+1)
	args = append(args, activityID)
	for _, id := range ids {
		args = append(args, id)
	}
	var list []Participant
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT id,tenant_id,activity_id,uid,name,department,identity,avatar_url,source,user_id,created_at
		FROM participants WHERE activity_id=? AND id IN (`+marks+`)`, args...)
	return list, err
}

// EligibleParticipantIDs 计算某奖项的待抽池：
// isAll=true 排除已中过本奖的；否则排除已中过任何奖的；始终排除有未决落库补偿的（防重复中奖）。
func (s *Store) EligibleParticipantIDs(ctx context.Context, activityID, prizeID uint64, isAll bool) ([]uint64, error) {
	q := `SELECT p.id FROM participants p
		WHERE p.activity_id=?`
	args := []any{activityID}
	if isAll {
		q += ` AND NOT EXISTS (SELECT 1 FROM draw_records d
			WHERE d.activity_id=p.activity_id AND d.participant_id=p.id AND d.status='won' AND d.prize_id=?)`
		args = append(args, prizeID)
	} else {
		q += ` AND NOT EXISTS (SELECT 1 FROM draw_records d
			WHERE d.activity_id=p.activity_id AND d.participant_id=p.id AND d.status='won')`
	}
	q += ` AND NOT EXISTS (SELECT 1 FROM persist_failures f
		WHERE f.activity_id=p.activity_id AND f.participant_id=p.id AND f.resolved_at IS NULL)`
	q += ` ORDER BY p.id`
	var ids []uint64
	err := s.conn.QueryRowsCtx(ctx, &ids, q, args...)
	return ids, err
}

func (s *Store) AllParticipantIDs(ctx context.Context, activityID uint64) ([]uint64, error) {
	var ids []uint64
	err := s.conn.QueryRowsCtx(ctx, &ids, `SELECT id FROM participants WHERE activity_id=? ORDER BY id`, activityID)
	return ids, err
}

// DeleteParticipant 删除未中奖的参与者；已中奖的由 service 层拦截。
func (s *Store) DeleteParticipant(ctx context.Context, tenantID, activityID, participantID uint64) error {
	res, err := s.conn.ExecCtx(ctx, `DELETE FROM participants WHERE id=? AND activity_id=? AND tenant_id=?`,
		participantID, activityID, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("not_found")
	}
	return nil
}

func (s *Store) ParticipantWon(ctx context.Context, activityID, participantID uint64) (bool, error) {
	var n int
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM draw_records
		WHERE activity_id=? AND participant_id=? AND status='won'`, activityID, participantID)
	return n > 0, err
}

func (s *Store) CountParticipants(ctx context.Context, activityID uint64) (int64, error) {
	var n int64
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM participants WHERE activity_id=?`, activityID)
	return n, err
}

// WonParticipantIDs 返回活动内已中奖（won）的参与者 id 集合。
func (s *Store) WonParticipantIDs(ctx context.Context, activityID uint64) (map[uint64]bool, error) {
	var ids []uint64
	err := s.conn.QueryRowsCtx(ctx, &ids, `SELECT DISTINCT participant_id FROM draw_records
		WHERE activity_id=? AND status='won' AND participant_id>0`, activityID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// CountWinsByPrize 统计每个奖项已中出份数（live 模式剩余库存 = stock - 已中）。
func (s *Store) CountWinsByPrize(ctx context.Context, activityID uint64) (map[uint64]int64, error) {
	var rows []struct {
		PrizeID uint64 `db:"prize_id"`
		N       int64  `db:"n"`
	}
	err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT prize_id, COUNT(*) AS n FROM draw_records
		WHERE activity_id=? AND status='won' GROUP BY prize_id`, activityID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]int64, len(rows))
	for _, r := range rows {
		out[r.PrizeID] = r.N
	}
	return out, nil
}

func (s *Store) ScheduledWinCounts(ctx context.Context, activityID uint64) (map[uint64]int64, error) {
	var rows []struct {
		PrizeID uint64 `db:"prize_id"`
		N       int64  `db:"n"`
	}
	err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT prize_id, COUNT(*) AS n FROM scheduled_winners
		WHERE activity_id=? GROUP BY prize_id`, activityID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]int64, len(rows))
	for _, r := range rows {
		out[r.PrizeID] = r.N
	}
	return out, nil
}

func (s *Store) InsertWinnersTx(ctx context.Context, tenantID, activityID uint64, version int, seed string, wins []struct {
	UserID, ParticipantID, PrizeID uint64
	Token, Kind                    string
	Rank                           int
}) error {
	return s.conn.TransactCtx(ctx, func(_ context.Context, session sqlx.Session) error {
		res, err := session.Exec(`UPDATE activities SET status='drawn', drawn_at=?, draw_seed=?, version=version+1
			WHERE id=? AND tenant_id=? AND version=? AND status IN ('ended','running','published')`,
			time.Now().UTC(), seed, activityID, tenantID, version)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("cas_failed")
		}
		for _, w := range wins {
			if _, err = session.Exec(`INSERT INTO scheduled_winners
				(tenant_id,activity_id,user_id,participant_id,prize_id,prize_token,rank_no) VALUES (?,?,?,?,?,?,?)`,
				tenantID, activityID, w.UserID, w.ParticipantID, w.PrizeID, w.Token, w.Rank); err != nil {
				return err
			}
		}
		return nil
	})
}

// ScheduledWinners 中奖名单（名字取自 participants 统一名单）。
func (s *Store) ScheduledWinners(ctx context.Context, activityID uint64) ([]WinnerRow, error) {
	var list []WinnerRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT pt.name AS nickname, p.name AS prize_name, p.kind, w.created_at
		FROM scheduled_winners w
		JOIN participants pt ON pt.id=w.participant_id
		JOIN prizes p ON p.id=w.prize_id
		WHERE w.activity_id=? ORDER BY w.rank_no`, activityID)
	return list, err
}

// LiveWinners 大屏中奖名单（draw_records join participants），按时间倒序。
func (s *Store) LiveWinners(ctx context.Context, activityID uint64, limit int) ([]WinnerRow, error) {
	var list []WinnerRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT pt.name AS nickname, p.name AS prize_name, d.kind, d.created_at
		FROM draw_records d
		JOIN participants pt ON pt.id=d.participant_id
		JOIN prizes p ON p.id=d.prize_id
		WHERE d.activity_id=? AND d.status='won'
		ORDER BY d.id DESC LIMIT ?`, activityID, limit)
	return list, err
}

// AdminWinnerRow 管理端中奖名单（含核销状态）。
type AdminWinnerRow struct {
	ParticipantID uint64    `db:"participant_id"`
	Uid           string    `db:"uid"`
	Name          string    `db:"name"`
	Department    string    `db:"department"`
	PrizeName     string    `db:"prize_name"`
	Kind          string    `db:"kind"`
	PrizeToken    string    `db:"prize_token"`
	Source        string    `db:"source"`
	RedeemStatus  string    `db:"redeem_status"`
	CodePrefix    string    `db:"code_prefix"`
	WonAt         time.Time `db:"created_at"`
}

func (s *Store) AdminLiveWinners(ctx context.Context, activityID uint64) ([]AdminWinnerRow, error) {
	var list []AdminWinnerRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT pt.id AS participant_id, pt.uid, pt.name, pt.department,
		p.name AS prize_name, d.kind, d.prize_token, pt.source,
		IFNULL(r.status,'') AS redeem_status, IFNULL(r.code_prefix,'') AS code_prefix, d.created_at
		FROM draw_records d
		JOIN participants pt ON pt.id=d.participant_id
		JOIN prizes p ON p.id=d.prize_id
		LEFT JOIN redemptions r ON r.draw_ref=d.prize_token
		WHERE d.activity_id=? AND d.status='won'
		ORDER BY d.id DESC`, activityID)
	return list, err
}

func (s *Store) AdminScheduledWinners(ctx context.Context, activityID uint64) ([]AdminWinnerRow, error) {
	var list []AdminWinnerRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT pt.id AS participant_id, pt.uid, pt.name, pt.department,
		p.name AS prize_name, p.kind, w.prize_token, pt.source,
		IFNULL(r.status,'') AS redeem_status, IFNULL(r.code_prefix,'') AS code_prefix, w.created_at
		FROM scheduled_winners w
		JOIN participants pt ON pt.id=w.participant_id
		JOIN prizes p ON p.id=w.prize_id
		LEFT JOIN redemptions r ON r.draw_ref=w.prize_token
		WHERE w.activity_id=?
		ORDER BY w.rank_no`, activityID)
	return list, err
}

// MarkOfflineUsed 导入名单中奖者线下发奖：为该中奖记录补一条"已核销"的核销行（无兑换码流程）。
func (s *Store) MarkOfflineUsed(ctx context.Context, tenantID uint64, prizeToken string, adminID uint64) error {
	res, err := s.conn.ExecCtx(ctx, `INSERT INTO redemptions
		(tenant_id,activity_id,user_id,prize_id,draw_ref,code_hash,code_prefix,status,used_at,used_by)
		SELECT tenant_id,activity_id,user_id,prize_id,prize_token,
			SHA2(CONCAT('OFFLINE:',prize_token),256), 'OFFLINE', 'used', UTC_TIMESTAMP(3), ?
		FROM draw_records WHERE tenant_id=? AND prize_token=? AND status='won'`, adminID, tenantID, prizeToken)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("not_found")
	}
	return nil
}

func (s *Store) InsertRedemption(ctx context.Context, r Redemption) error {
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO redemptions
		(tenant_id,activity_id,user_id,prize_id,draw_ref,code_hash,code_prefix,status)
		VALUES (?,?,?,?,?,?,?,?)`,
		r.TenantID, r.ActivityID, r.UserID, r.PrizeID, r.DrawRef, r.CodeHash, r.CodePrefix, r.Status)
	return err
}

func (s *Store) RedeemCAS(ctx context.Context, tenantID, adminID uint64, hash string) error {
	res, err := s.conn.ExecCtx(ctx, `UPDATE redemptions SET status='used', used_at=?, used_by=?
		WHERE tenant_id=? AND code_hash=? AND status='unused'`, time.Now().UTC(), adminID, tenantID, hash)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("redeem_failed")
	}
	return nil
}

func (s *Store) FillAddress(ctx context.Context, tenantID, userID uint64, token, name, phone, addr string) error {
	res, err := s.conn.ExecCtx(ctx, `UPDATE redemptions SET contact_name=?, contact_phone=?, address=?
		WHERE tenant_id=? AND user_id=? AND draw_ref=? AND status='unused'`, name, phone, addr, tenantID, userID, token)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("not_found")
	}
	return nil
}

func (s *Store) MyPrizes(ctx context.Context, tenantID, userID uint64) ([]MyPrizeRow, error) {
	var list []MyPrizeRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT p.name AS prize_name, d.kind,
		IFNULL(r.status,'won') AS status, IFNULL(r.code_prefix,'') AS code_prefix,
		d.created_at, a.title
		FROM draw_records d
		JOIN prizes p ON p.id=d.prize_id
		JOIN activities a ON a.id=d.activity_id
		LEFT JOIN redemptions r ON r.draw_ref=d.prize_token
		WHERE d.tenant_id=? AND d.user_id=? AND d.status='won'
		ORDER BY d.id DESC`, tenantID, userID)
	return list, err
}

func (s *Store) AddBlacklist(ctx context.Context, tenantID, userID uint64, reason string) error {
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO blacklist (tenant_id,user_id,reason) VALUES (?,?,?)
		ON DUPLICATE KEY UPDATE reason=VALUES(reason)`, tenantID, userID, reason)
	return err
}

func (s *Store) Blacklisted(ctx context.Context, tenantID, userID uint64) (bool, error) {
	var n int
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM blacklist WHERE tenant_id=? AND user_id=?`, tenantID, userID)
	return n > 0, err
}

func (s *Store) Audit(ctx context.Context, tenantID, actor uint64, action, targetType, targetID string, detail any) {
	b, _ := json.Marshal(detail)
	_, _ = s.conn.ExecCtx(ctx, `INSERT INTO audit_logs (tenant_id,actor_id,action,target_type,target_id,detail)
		VALUES (?,?,?,?,?,?)`, tenantID, actor, action, targetType, targetID, b)
}

func (s *Store) InsertDrawAudit(ctx context.Context, tenantID, activityID uint64, seed string, participants, winners any) error {
	p, _ := json.Marshal(participants)
	w, _ := json.Marshal(winners)
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO draw_audits (tenant_id,activity_id,seed,participant_snapshot,winner_snapshot)
		VALUES (?,?,?,?,?)`, tenantID, activityID, seed, p, w)
	return err
}

func IsNoRows(err error) bool {
	return err == sql.ErrNoRows || err == sqlx.ErrNotFound
}
