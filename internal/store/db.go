package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

func (s *Store) CreateActivityTx(ctx context.Context, a Activity, prizes []Prize) (uint64, error) {
	var id uint64
	err := s.conn.TransactCtx(ctx, func(_ context.Context, session sqlx.Session) error {
		res, err := session.Exec(`INSERT INTO activities
			(tenant_id,public_id,title,mode,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			a.TenantID, a.PublicID, a.Title, a.Mode, a.Status, a.Timezone, a.StartAt, a.EndAt, a.MaxDrawsPerUser, a.MaxEnrollments)
		if err != nil {
			return err
		}
		lid, _ := res.LastInsertId()
		id = uint64(lid)
		for i, p := range prizes {
			_, err = session.Exec(`INSERT INTO prizes
				(tenant_id,activity_id,name,kind,stock,weight,image_url,sort_order)
				VALUES (?,?,?,?,?,?,?,?)`,
				a.TenantID, id, p.Name, p.Kind, p.Stock, p.Weight, p.ImageURL, i)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (s *Store) ListActivities(ctx context.Context, tenantID uint64, status string) ([]Activity, error) {
	q := `SELECT id,tenant_id,public_id,title,mode,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments,version,published_at,drawn_at,draw_seed
		FROM activities WHERE tenant_id=?`
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
	err := s.conn.QueryRowCtx(ctx, &a, `SELECT id,tenant_id,public_id,title,mode,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments,version,published_at,drawn_at,draw_seed
		FROM activities WHERE id=? AND tenant_id=?`, id, tenantID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ActivityByPublicID(ctx context.Context, publicID string) (*Activity, error) {
	var a Activity
	err := s.conn.QueryRowCtx(ctx, &a, `SELECT id,tenant_id,public_id,title,mode,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments,version,published_at,drawn_at,draw_seed
		FROM activities WHERE public_id=?`, publicID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ActivityByIDOnly(ctx context.Context, id uint64) (*Activity, error) {
	var a Activity
	err := s.conn.QueryRowCtx(ctx, &a, `SELECT id,tenant_id,public_id,title,mode,status,timezone,start_at,end_at,max_draws_per_user,max_enrollments,version,published_at,drawn_at,draw_seed
		FROM activities WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) Prizes(ctx context.Context, activityID uint64) ([]Prize, error) {
	var list []Prize
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT id,tenant_id,activity_id,name,kind,stock,weight,image_url,sort_order
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

func (s *Store) SetDrawn(ctx context.Context, tenantID, id uint64, version int, seed string) error {
	res, err := s.conn.ExecCtx(ctx, `UPDATE activities SET status='drawn', drawn_at=?, draw_seed=?, version=version+1
		WHERE id=? AND tenant_id=? AND version=? AND status IN ('ended','running')`,
		time.Now().UTC(), seed, id, tenantID, version)
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
		(tenant_id,activity_id,user_id,prize_id,prize_token,idempotency_key,kind,status)
		VALUES (?,?,?,?,?,?,?,?)`,
		r.TenantID, r.ActivityID, r.UserID, r.PrizeID, r.PrizeToken, r.IdempotencyKey, r.Kind, r.Status)
	return err
}

func (s *Store) DrawByIdemp(ctx context.Context, activityID, userID uint64, key string) (*DrawRecord, error) {
	var r DrawRecord
	err := s.conn.QueryRowCtx(ctx, &r, `SELECT id,tenant_id,activity_id,user_id,prize_id,prize_token,idempotency_key,kind,status,created_at
		FROM draw_records WHERE activity_id=? AND user_id=? AND idempotency_key=?`, activityID, userID, key)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) DrawByToken(ctx context.Context, tenantID uint64, token string) (*DrawRecord, error) {
	var r DrawRecord
	err := s.conn.QueryRowCtx(ctx, &r, `SELECT id,tenant_id,activity_id,user_id,prize_id,prize_token,idempotency_key,kind,status,created_at
		FROM draw_records WHERE prize_token=? AND tenant_id=?`, token, tenantID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) InsertPersistFailure(ctx context.Context, r DrawRecord, errMsg string) error {
	payload, _ := json.Marshal(r)
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO persist_failures
		(tenant_id,activity_id,user_id,prize_id,prize_token,idempotency_key,kind,payload,retry_count,last_error,next_retry_at)
		VALUES (?,?,?,?,?,?,?,?,0,?,?)
		ON DUPLICATE KEY UPDATE last_error=VALUES(last_error), retry_count=retry_count+1, next_retry_at=VALUES(next_retry_at)`,
		r.TenantID, r.ActivityID, r.UserID, r.PrizeID, r.PrizeToken, r.IdempotencyKey, r.Kind, payload, errMsg, time.Now().UTC().Add(5*time.Second))
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

func (s *Store) InsertEnrollment(ctx context.Context, tenantID, activityID, userID uint64) error {
	_, err := s.conn.ExecCtx(ctx, `INSERT INTO enrollments (tenant_id,activity_id,user_id) VALUES (?,?,?)`, tenantID, activityID, userID)
	return err
}

func (s *Store) CountEnroll(ctx context.Context, activityID uint64) (int64, error) {
	var n int64
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM enrollments WHERE activity_id=?`, activityID)
	return n, err
}

func (s *Store) EnrollUserIDs(ctx context.Context, activityID uint64) ([]uint64, error) {
	var rows []IDRow
	err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT user_id FROM enrollments WHERE activity_id=? ORDER BY id`, activityID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, nil
}

func (s *Store) InsertWinnersTx(ctx context.Context, tenantID, activityID uint64, version int, seed string, wins []struct {
	UserID, PrizeID uint64
	Token, Kind     string
	Rank            int
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
				(tenant_id,activity_id,user_id,prize_id,prize_token,rank_no) VALUES (?,?,?,?,?,?)`,
				tenantID, activityID, w.UserID, w.PrizeID, w.Token, w.Rank); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ScheduledWinners(ctx context.Context, activityID uint64) ([]WinnerRow, error) {
	var list []WinnerRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT u.nickname, p.name AS prize_name, p.kind, w.created_at
		FROM scheduled_winners w
		JOIN users u ON u.id=w.user_id
		JOIN prizes p ON p.id=w.prize_id
		WHERE w.activity_id=? ORDER BY w.rank_no`, activityID)
	return list, err
}

func (s *Store) InstantWinners(ctx context.Context, activityID uint64, limit int) ([]WinnerRow, error) {
	var list []WinnerRow
	err := s.conn.QueryRowsCtx(ctx, &list, `SELECT u.nickname, p.name AS prize_name, d.kind, d.created_at
		FROM draw_records d
		JOIN users u ON u.id=d.user_id
		JOIN prizes p ON p.id=d.prize_id
		WHERE d.activity_id=?
		ORDER BY d.id DESC LIMIT ?`, activityID, limit)
	return list, err
}

func (s *Store) CountDraws(ctx context.Context, activityID uint64) (int64, error) {
	var n int64
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM draw_records WHERE activity_id=?`, activityID)
	return n, err
}

func (s *Store) CountWins(ctx context.Context, activityID uint64) (int64, error) {
	var n int64
	err := s.conn.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM draw_records WHERE activity_id=? AND kind<>'thank_you'`, activityID)
	return n, err
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
		WHERE d.tenant_id=? AND d.user_id=?
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
