package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"luckygo/internal/xerr"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type RedisDraw struct {
	R *redis.Redis
}

func metaKey(activityID uint64) string {
	return "lg:meta:" + strconv.FormatUint(activityID, 10)
}

func delayKey() string {
	return "lg:delay:jobs"
}

func LivePoolKey(activityID, prizeID uint64) string {
	return fmt.Sprintf("lg:live:pool:%d:%d", activityID, prizeID)
}

func livePoolBuiltKey(activityID, prizeID uint64) string {
	return fmt.Sprintf("lg:live:poolbuilt:%d:%d", activityID, prizeID)
}

func liveRosterVerKey(activityID uint64) string {
	return "lg:live:rosterver:" + strconv.FormatUint(activityID, 10)
}

func liveIdempKey(activityID uint64, key string) string {
	return fmt.Sprintf("lg:live:idemp:%d:%s", activityID, key)
}

type Meta struct {
	Status  string
	StartAt int64
	EndAt   int64
}

func (d RedisDraw) LoadMeta(_ context.Context, activityID uint64, meta Meta) error {
	return d.R.Hmset(metaKey(activityID), map[string]string{
		"status":   meta.Status,
		"start_at": strconv.FormatInt(meta.StartAt, 10),
		"end_at":   strconv.FormatInt(meta.EndAt, 10),
	})
}

func (d RedisDraw) SetStatus(activityID uint64, status string) error {
	_, err := d.R.Eval(metaLua, []string{metaKey(activityID)}, status)
	return err
}

// ---------- 现场大屏抽取（live） ----------

type LiveDrawResult struct {
	Duplicate bool   // 幂等重放
	Undone    bool   // 重放的是一批已被取消的结果
	DrawId    string // 批次号（= 客户端幂等键）
	WinnerIDs []uint64
	PoolSize  int // INSUFFICIENT 时的剩余可抽人数
}

// RosterVersion 当前名单版本（无则视为 0）。名单变更时由 BumpRosterVersion 递增。
func (d RedisDraw) RosterVersion(activityID uint64) (string, error) {
	v, err := d.R.Get(liveRosterVerKey(activityID))
	if err != nil {
		return "", err
	}
	if v == "" {
		return "0", nil
	}
	return v, nil
}

func (d RedisDraw) BumpRosterVersion(activityID uint64) error {
	_, err := d.R.Incr(liveRosterVerKey(activityID))
	return err
}

// LiveRebuildPool 用数据库算出的可抽名单重建某奖项的待抽池，并记录构建版本。
func (d RedisDraw) LiveRebuildPool(activityID, prizeID uint64, rosterVer string, ids []uint64) error {
	pool := LivePoolKey(activityID, prizeID)
	if _, err := d.R.Del(pool); err != nil {
		return err
	}
	if len(ids) > 0 {
		args := make([]any, 0, len(ids))
		for _, id := range ids {
			args = append(args, id)
		}
		if _, err := d.R.Sadd(pool, args...); err != nil {
			return err
		}
	}
	return d.R.Set(livePoolBuiltKey(activityID, prizeID), rosterVer)
}

// LiveDraw 从奖项待抽池原子弹出 count 个参与者。名单版本不匹配返回 xerr.ErrStalePool，
// 由调用方重建池后重试。
func (d RedisDraw) LiveDraw(activityID, prizeID uint64, idempotency string, count int, rosterVer string) (LiveDrawResult, error) {
	var out LiveDrawResult
	if idempotency == "" {
		return out, xerr.ErrIdempotency
	}
	result, err := d.R.Eval(
		liveDrawLua,
		[]string{
			LivePoolKey(activityID, prizeID),
			metaKey(activityID),
			liveIdempKey(activityID, idempotency),
			livePoolBuiltKey(activityID, prizeID),
		},
		time.Now().Unix(),
		86400*7,
		count,
		rosterVer,
		idempotency,
	)
	if err != nil {
		return out, err
	}
	arr, ok := toAnySlice(result)
	if !ok || len(arr) == 0 {
		return out, xerr.Internal()
	}
	switch code := redisString(arr[0]); code {
	case "IDEM":
		out.DrawId = idempotency
		out.WinnerIDs = parseLiveIdempValue(redisString(arr[1]), idempotency, &out.Undone)
		out.Duplicate = true
		return out, nil
	case "OK":
		out.DrawId = redisString(arr[1])
		for _, v := range arr[2:] {
			id, err := strconv.ParseUint(redisString(v), 10, 64)
			if err != nil {
				return out, xerr.Internal()
			}
			out.WinnerIDs = append(out.WinnerIDs, id)
		}
		return out, nil
	case "STALE":
		return out, xerr.ErrStalePool
	case "INSUFFICIENT":
		if len(arr) > 1 {
			out.PoolSize, _ = strconv.Atoi(redisString(arr[1]))
		}
		return out, xerr.ErrInsufficient
	case "PAUSED":
		return out, xerr.ErrPaused
	case "NOT_STARTED":
		return out, xerr.ErrNotStarted
	case "ENDED":
		return out, xerr.ErrEnded
	case "STATUS":
		return out, xerr.ErrBadStatus
	default:
		return out, xerr.Internal()
	}
}

// LiveUndo 取消一批已抽取结果：原子校验批次存在且未取消，随后把幂等值标记为 UNDONE（保留 TTL）。
// 参与者回池靠调用方 BumpRosterVersion 触发池重建（MySQL 中记录已翻为 undone）。
func (d RedisDraw) LiveUndo(activityID uint64, drawId string) error {
	result, err := d.R.Eval(liveUndoLua, []string{liveIdempKey(activityID, drawId)}, drawId)
	if err != nil {
		return err
	}
	arr, ok := toAnySlice(result)
	if !ok || len(arr) == 0 {
		return xerr.Internal()
	}
	switch redisString(arr[0]) {
	case "OK":
		return nil
	case "NONE", "MISMATCH":
		return xerr.NotFound("该批次不存在或已过期")
	case "UNDONE":
		return xerr.ErrUndone
	default:
		return xerr.Internal()
	}
}

// parseLiveIdempValue 解析幂等值 "drawId:id1,id2,..."；若已被取消（drawId:UNDONE）置 undone。
func parseLiveIdempValue(v, drawId string, undone *bool) []uint64 {
	rest := strings.TrimPrefix(v, drawId+":")
	if rest == "UNDONE" {
		*undone = true
		return nil
	}
	if rest == v || rest == "" {
		return nil
	}
	var ids []uint64
	for _, s := range strings.Split(rest, ",") {
		if id, err := strconv.ParseUint(s, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// ---------- 延迟任务（状态翻转/到期开奖） ----------

func (d RedisDraw) ScheduleJob(kind string, activityID uint64, atUnix int64) error {
	member := fmt.Sprintf("%s:%d", kind, activityID)
	_, err := d.R.Zadd(delayKey(), atUnix, member)
	return err
}

func (d RedisDraw) DueJobs(now int64, limit int) ([]string, error) {
	pairs, err := d.R.ZrangebyscoreWithScoresAndLimit(delayKey(), 0, now, 0, limit)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.Key)
	}
	return out, nil
}

func (d RedisDraw) RemoveJob(member string) error {
	_, err := d.R.Zrem(delayKey(), member)
	return err
}

func redisString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func toAnySlice(v any) ([]any, bool) {
	arr, ok := v.([]any)
	return arr, ok
}
