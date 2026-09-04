package ctxdata

import (
	"context"
	"strconv"

	"luckygo/internal/xerr"
)

type Identity struct {
	UserID   uint64
	TenantID uint64
	Role     string
	Account  string
}

func FromCtx(ctx context.Context) (Identity, error) {
	var id Identity
	uid, err := asString(ctx.Value("userId"))
	if err != nil {
		return id, xerr.Unauth("未登录")
	}
	tid, err := asString(ctx.Value("tenantId"))
	if err != nil {
		return id, xerr.Unauth("未登录")
	}
	id.UserID, _ = strconv.ParseUint(uid, 10, 64)
	id.TenantID, _ = strconv.ParseUint(tid, 10, 64)
	id.Role, _ = asString(ctx.Value("role"))
	id.Account, _ = asString(ctx.Value("account"))
	if id.UserID == 0 || id.TenantID == 0 {
		return id, xerr.Unauth("未登录")
	}
	return id, nil
}

func MustAdmin(ctx context.Context) (Identity, error) {
	id, err := FromCtx(ctx)
	if err != nil {
		return id, err
	}
	if id.Role != "admin" {
		return id, xerr.ErrNeedAdmin
	}
	return id, nil
}

func MustUser(ctx context.Context) (Identity, error) {
	id, err := FromCtx(ctx)
	if err != nil {
		return id, err
	}
	if id.Role != "user" {
		return id, xerr.ErrNeedUser
	}
	return id, nil
}

func asString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "", xerr.Unauth("未登录")
		}
		return t, nil
	case float64:
		return strconv.FormatInt(int64(t), 10), nil
	default:
		return "", xerr.Unauth("未登录")
	}
}
