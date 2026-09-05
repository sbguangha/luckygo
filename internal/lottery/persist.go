package lottery

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const rosterDDL = `CREATE TABLE IF NOT EXISTS live_roster (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	user_id VARCHAR(64) NOT NULL,
	name VARCHAR(32) NOT NULL,
	staff_no VARCHAR(32) NOT NULL DEFAULT '',
	source VARCHAR(16) NOT NULL DEFAULT 'form',
	openid VARCHAR(64) NOT NULL DEFAULT '',
	status VARCHAR(16) NOT NULL DEFAULT 'active',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	won_at DATETIME(3) NULL,
	PRIMARY KEY (id),
	UNIQUE KEY uk_live_roster_user (user_id),
	KEY idx_live_roster_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`

var errAlreadyWon = errors.New("already_won")

type rosterDB struct {
	db *sql.DB
}

func NewHubFromDSN(dsn string, conf Conf) (*Hub, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, rosterDDL); err != nil {
		_ = db.Close()
		return nil, err
	}
	h := NewHub()
	h.persist = &rosterDB{db: db}
	h.conf = conf
	if err := h.loadActive(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return h, nil
}

func (h *Hub) loadActive() error {
	return h.replaceActiveFromDB()
}

func (h *Hub) replaceActiveFromDB() error {
	if h.persist == nil {
		return nil
	}
	h.drawMu.Lock()
	defer h.drawMu.Unlock()
	return h.replaceActiveFromDBLocked()
}

func (h *Hub) replaceActiveFromDBLocked() error {
	list, err := h.persist.listActive()
	if err != nil {
		return err
	}
	fresh := make(map[string]Member, len(list))
	for _, m := range list {
		m.Status = "active"
		fresh[m.UserID] = m
	}
	h.members.Range(func(k, _ any) bool {
		id, _ := k.(string)
		if _, ok := fresh[id]; !ok {
			h.members.Delete(id)
		}
		return true
	})
	for id, m := range fresh {
		h.members.Store(id, m)
	}
	return nil
}

func (p *rosterDB) listActive() ([]Member, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	rows, err := p.db.QueryContext(ctx, `SELECT user_id,name,staff_no,source,openid,CAST(UNIX_TIMESTAMP(created_at) AS UNSIGNED)
		FROM live_roster WHERE status='active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Member, 0, 64)
	for rows.Next() {
		var m Member
		var joined float64
		if err := rows.Scan(&m.UserID, &m.Name, &m.StaffNo, &m.Source, &m.OpenID, &joined); err != nil {
			return nil, err
		}
		m.JoinedAt = int64(joined)
		m.Status = "active"
		out = append(out, m)
	}
	return out, rows.Err()
}

func (p *rosterDB) upsertActive(m Member) (inserted bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := p.db.ExecContext(ctx, `INSERT INTO live_roster (user_id,name,staff_no,source,openid,status)
		VALUES (?,?,?,?,?,'active')
		ON DUPLICATE KEY UPDATE name=name`,
		m.UserID, m.Name, m.StaffNo, m.Source, m.OpenID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	// MySQL: insert=1, no-op update=0, actual update=2
	return n == 1, nil
}

func (p *rosterDB) isWon(userID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var status string
	err := p.db.QueryRowContext(ctx, `SELECT status FROM live_roster WHERE user_id=?`, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return status == "won", err
}

func (p *rosterDB) get(userID string) (Member, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var m Member
	err := p.db.QueryRowContext(ctx, `SELECT user_id,name,staff_no,source,openid,status
		FROM live_roster WHERE user_id=?`, userID).Scan(
		&m.UserID, &m.Name, &m.StaffNo, &m.Source, &m.OpenID, &m.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, false, nil
	}
	return m, err == nil, err
}

func (p *rosterDB) markWon(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `UPDATE live_roster SET status='won', won_at=UTC_TIMESTAMP(3) WHERE user_id=? AND status='active'`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
