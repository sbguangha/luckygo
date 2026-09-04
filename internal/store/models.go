package store

import "time"

type Tenant struct {
	ID        uint64    `db:"id"`
	Name      string    `db:"name"`
	Status    int64     `db:"status"`
	CreatedAt time.Time `db:"created_at"`
}

type User struct {
	ID           uint64    `db:"id"`
	TenantID     uint64    `db:"tenant_id"`
	Role         string    `db:"role"`
	Account      string    `db:"account"`
	PasswordHash string    `db:"password_hash"`
	Nickname     string    `db:"nickname"`
	Status       int64     `db:"status"`
}

type Activity struct {
	ID               uint64     `db:"id"`
	TenantID         uint64     `db:"tenant_id"`
	PublicID         string     `db:"public_id"`
	Title            string     `db:"title"`
	Mode             string     `db:"mode"`
	Status           string     `db:"status"`
	Timezone         string     `db:"timezone"`
	StartAt          time.Time  `db:"start_at"`
	EndAt            time.Time  `db:"end_at"`
	MaxDrawsPerUser  int        `db:"max_draws_per_user"`
	MaxEnrollments   int        `db:"max_enrollments"`
	Version          int        `db:"version"`
	PublishedAt      *time.Time `db:"published_at"`
	DrawnAt          *time.Time `db:"drawn_at"`
	DrawSeed         *string    `db:"draw_seed"`
}

type Prize struct {
	ID         uint64 `db:"id"`
	TenantID   uint64 `db:"tenant_id"`
	ActivityID uint64 `db:"activity_id"`
	Name       string `db:"name"`
	Kind       string `db:"kind"`
	Stock      int    `db:"stock"`
	Weight     int    `db:"weight"`
	ImageURL   string `db:"image_url"`
	SortOrder  int    `db:"sort_order"`
}

type DrawRecord struct {
	ID             uint64    `db:"id"`
	TenantID       uint64    `db:"tenant_id"`
	ActivityID     uint64    `db:"activity_id"`
	UserID         uint64    `db:"user_id"`
	PrizeID        uint64    `db:"prize_id"`
	PrizeToken     string    `db:"prize_token"`
	IdempotencyKey string    `db:"idempotency_key"`
	Kind           string    `db:"kind"`
	Status         string    `db:"status"`
	CreatedAt      time.Time `db:"created_at"`
}

type Redemption struct {
	ID           uint64     `db:"id"`
	TenantID     uint64     `db:"tenant_id"`
	ActivityID   uint64     `db:"activity_id"`
	UserID       uint64     `db:"user_id"`
	PrizeID      uint64     `db:"prize_id"`
	DrawRef      string     `db:"draw_ref"`
	CodeHash     string     `db:"code_hash"`
	CodePrefix   string     `db:"code_prefix"`
	Status       string     `db:"status"`
	Address      string     `db:"address"`
	ContactName  string     `db:"contact_name"`
	ContactPhone string     `db:"contact_phone"`
	UsedAt       *time.Time `db:"used_at"`
	UsedBy       *uint64    `db:"used_by"`
}

type WinnerRow struct {
	Nickname  string    `db:"nickname"`
	PrizeName string    `db:"prize_name"`
	Kind      string    `db:"kind"`
	WonAt     time.Time `db:"created_at"`
}

type MyPrizeRow struct {
	PrizeName  string    `db:"prize_name"`
	Kind       string    `db:"kind"`
	Status     string    `db:"status"`
	CodePrefix string    `db:"code_prefix"`
	WonAt      time.Time `db:"created_at"`
	Title      string    `db:"title"`
}

type IDRow struct {
	ID uint64 `db:"user_id"`
}
