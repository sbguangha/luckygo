package tokenkit

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func Issue(secret string, expireSec int64, userID, tenantID uint64, role, account string) (string, error) {
	now := time.Now().Unix()
	claims := jwt.MapClaims{
		"exp":      now + expireSec,
		"iat":      now,
		"userId":   strconv.FormatUint(userID, 10),
		"tenantId": strconv.FormatUint(tenantID, 10),
		"role":     role,
		"account":  account,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}
