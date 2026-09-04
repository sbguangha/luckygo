package engine

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"luckygo/internal/xerr"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

type RedisDraw struct {
	R *redis.Redis
}

func Keys(activityID uint64) (bucket, quotaPrefix, meta, remain string) {
	id := strconv.FormatUint(activityID, 10)
	bucket = "lg:bucket:" + id
	quotaPrefix = "lg:quota:" + id + ":"
	meta = "lg:meta:" + id
	remain = "lg:remain:" + id
	return
}

func quotaKey(activityID, userID uint64) string {
	return fmt.Sprintf("lg:quota:%d:%d", activityID, userID)
}

func idempKey(activityID, userID uint64, key string) string {
	return fmt.Sprintf("lg:idemp:%d:%d:%s", activityID, userID, key)
}

func inflightKey(activityID, userID uint64) string {
	return fmt.Sprintf("lg:inflight:%d:%d", activityID, userID)
}

func delayKey() string {
	return "lg:delay:jobs"
}

type Meta struct {
	Status   string
	StartAt  int64
	EndAt    int64
	MaxDraws int
}

type DrawResult struct {
	Duplicate   bool
	Item        BucketItem
	Raw         string
	RemainDraws int
}

func (d RedisDraw) LoadBucket(_ context.Context, activityID uint64, meta Meta, items []string, remain map[string]int) error {
	bucket, _, metaKey, remainKey := Keys(activityID)
	if _, err := d.R.Del(bucket); err != nil {
		return err
	}
	if len(items) > 0 {
		args := make([]any, 0, len(items))
		for _, it := range items {
			args = append(args, it)
		}
		if _, err := d.R.Rpush(bucket, args...); err != nil {
			return err
		}
	}
	if err := d.R.Hmset(metaKey, map[string]string{
		"status":    meta.Status,
		"start_at":  strconv.FormatInt(meta.StartAt, 10),
		"end_at":    strconv.FormatInt(meta.EndAt, 10),
		"max_draws": strconv.Itoa(meta.MaxDraws),
	}); err != nil {
		return err
	}
	if _, err := d.R.Del(remainKey); err != nil {
		return err
	}
	if len(remain) > 0 {
		fields := make(map[string]string, len(remain))
		for k, v := range remain {
			fields[k] = strconv.Itoa(v)
		}
		if err := d.R.Hmset(remainKey, fields); err != nil {
			return err
		}
	}
	return nil
}

func (d RedisDraw) SetStatus(activityID uint64, status string) error {
	_, _, metaKey, _ := Keys(activityID)
	_, err := d.R.Eval(metaLua, []string{metaKey}, status)
	return err
}

func (d RedisDraw) Remain(activityID uint64) (map[uint64]int, error) {
	_, _, _, remainKey := Keys(activityID)
	m, err := d.R.Hgetall(remainKey)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]int, len(m))
	for k, v := range m {
		id, _ := strconv.ParseUint(k, 10, 64)
		n, _ := strconv.Atoi(v)
		out[id] = n
	}
	return out, nil
}

func (d RedisDraw) Draw(activityID, userID uint64, idempotency string) (DrawResult, error) {
	var out DrawResult
	if idempotency == "" {
		return out, xerr.ErrIdempotency
	}
	bucket, _, metaKey, remainKey := Keys(activityID)
	result, err := d.R.Eval(
		drawLua,
		[]string{
			bucket,
			quotaKey(activityID, userID),
			metaKey,
			idempKey(activityID, userID, idempotency),
			inflightKey(activityID, userID),
			remainKey,
		},
		time.Now().Unix(),
		86400*7,
		5,
	)
	if err != nil {
		return out, err
	}
	arr, ok := toAnySlice(result)
	if !ok || len(arr) == 0 {
		return out, xerr.Internal()
	}
	code := redisString(arr[0])
	switch code {
	case "IDEM":
		raw := redisString(arr[1])
		item, err := DecodeItem(raw)
		if err != nil {
			return out, err
		}
		out.Duplicate = true
		out.Item = item
		out.Raw = raw
		return out, nil
	case "OK":
		raw := redisString(arr[1])
		item, err := DecodeItem(raw)
		if err != nil {
			return out, err
		}
		remain := 0
		if len(arr) > 2 {
			remain, _ = strconv.Atoi(fmt.Sprint(arr[2]))
		}
		out.Item = item
		out.Raw = raw
		out.RemainDraws = remain
		return out, nil
	case "BUSY":
		return out, xerr.ErrBusy
	case "PAUSED":
		return out, xerr.ErrPaused
	case "NOT_STARTED":
		return out, xerr.ErrNotStarted
	case "ENDED":
		return out, xerr.ErrEnded
	case "QUOTA":
		return out, xerr.ErrQuota
	case "EMPTY":
		return out, xerr.ErrEmpty
	case "STATUS":
		return out, xerr.ErrBadStatus
	default:
		return out, xerr.Internal()
	}
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

func (d RedisDraw) ClearInflight(activityID, userID uint64) {
	_, _ = d.R.Del(inflightKey(activityID, userID))
}

func (d RedisDraw) Undo(activityID, userID uint64, idempotency, raw string) error {
	bucket, _, _, remainKey := Keys(activityID)
	_, err := d.R.Eval(
		undoLua,
		[]string{bucket, quotaKey(activityID, userID), idempKey(activityID, userID, idempotency), remainKey},
		raw,
	)
	return err
}

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
