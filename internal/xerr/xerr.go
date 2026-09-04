package xerr

import (
	"fmt"
	"net/http"
)

type CodeError struct {
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	HTTPStatus int    `json:"-"`
}

func (e *CodeError) Error() string {
	return fmt.Sprintf("code=%d msg=%s", e.Code, e.Msg)
}

func New(httpStatus, code int, msg string) *CodeError {
	return &CodeError{Code: code, Msg: msg, HTTPStatus: httpStatus}
}

func Bad(msg string) *CodeError {
	return New(http.StatusBadRequest, 40000, msg)
}

func Unauth(msg string) *CodeError {
	return New(http.StatusUnauthorized, 40100, msg)
}

func Forbid(msg string) *CodeError {
	return New(http.StatusForbidden, 40300, msg)
}

func NotFound(msg string) *CodeError {
	return New(http.StatusNotFound, 40400, msg)
}

func Conflict(msg string) *CodeError {
	return New(http.StatusConflict, 40900, msg)
}

func TooMany(msg string) *CodeError {
	return New(http.StatusTooManyRequests, 42900, msg)
}

func Internal() *CodeError {
	return New(http.StatusInternalServerError, 50000, "服务暂时不可用")
}

var (
	ErrInvalidParam     = Bad("参数不合法")
	ErrPassword         = Unauth("账号或密码错误")
	ErrAccountTaken     = Conflict("账号已存在")
	ErrTenantTaken      = Conflict("租户名已存在")
	ErrNeedAdmin        = Forbid("需要管理员权限")
	ErrNeedUser         = Forbid("请使用参与者账号抽奖")
	ErrTenantMismatch   = Forbid("账号不属于该活动")
	ErrActivityNotFound = NotFound("活动不存在")
	ErrNotDraft         = Conflict("仅草稿可执行该操作")
	ErrBadStatus        = Conflict("活动状态不允许该操作")
	ErrNotStarted       = Bad("活动尚未开始")
	ErrEnded            = Bad("活动已结束")
	ErrPaused           = Bad("活动已暂停")
	ErrQuota            = Bad("抽奖次数已用尽")
	ErrEmpty            = Bad("奖品已抽完")
	ErrBusy             = TooMany("请勿重复点击")
	ErrIdempotency      = Bad("缺少幂等键")
	ErrWrongMode        = Bad("当前玩法不支持该操作")
	ErrBlacklisted      = Forbid("账号已被限制参与")
	ErrRedeemed         = Conflict("兑换码已核销或无效")
	ErrEnrolled         = Conflict("已经报名")
	ErrEnrollFull       = Conflict("报名人数已满")
	ErrNotDrawn         = NotFound("尚未开奖")
)
