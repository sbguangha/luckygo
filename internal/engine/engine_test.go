package engine_test

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"luckygo/internal/engine"
	"luckygo/internal/xerr"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func setup(t *testing.T) engine.RedisDraw {
	t.Helper()
	s := miniredis.RunT(t)
	r, err := redis.NewRedis(redis.RedisConf{Host: s.Addr(), Type: "node"})
	if err != nil {
		t.Fatal(err)
	}
	return engine.RedisDraw{R: r}
}

func loadRunningMeta(t *testing.T, d engine.RedisDraw, activityID uint64) {
	t.Helper()
	now := time.Now().Unix()
	if err := d.LoadMeta(context.Background(), activityID, engine.Meta{
		Status: "running", StartAt: now - 10, EndAt: now + 3600,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePrizes(t *testing.T) {
	ok := []engine.PrizeSpec{
		{ID: 1, Name: "一等奖", Kind: "physical", Stock: 1, PerRound: 1},
		{ID: 2, Name: "三等奖", Kind: "virtual", Stock: 10, PerRound: 5},
	}
	if err := engine.ValidatePrizes(ok, "live"); err != nil {
		t.Fatal(err)
	}
	if err := engine.ValidatePrizes(nil, "live"); err == nil {
		t.Fatal("empty prizes should fail")
	}
	if err := engine.ValidatePrizes([]engine.PrizeSpec{
		{ID: 1, Name: "x", Kind: "thank_you", Stock: 1, PerRound: 1},
	}, "live"); err == nil {
		t.Fatal("thank_you should be rejected in live mode")
	}
	if err := engine.ValidatePrizes([]engine.PrizeSpec{
		{ID: 1, Name: "x", Kind: "virtual", Stock: 2, PerRound: 5},
	}, "live"); err == nil {
		t.Fatal("perRound > stock should fail")
	}
}

func TestLiveDrawStaleThenRebuild(t *testing.T) {
	d := setup(t)
	loadRunningMeta(t, d, 1)
	// 池未构建（版本不匹配）-> STALE
	ver, err := d.RosterVersion(1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.LiveDraw(1, 100, "k1", 2, ver)
	if err != xerr.ErrStalePool {
		t.Fatalf("want stale got %v", err)
	}
	if err := d.LiveRebuildPool(1, 100, ver, []uint64{11, 12, 13, 14, 15}); err != nil {
		t.Fatal(err)
	}
	res, err := d.LiveDraw(1, 100, "k1", 2, ver)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.WinnerIDs) != 2 || res.Duplicate {
		t.Fatalf("bad result %+v", res)
	}
	// 名单版本变化 -> STALE
	if err := d.BumpRosterVersion(1); err != nil {
		t.Fatal(err)
	}
	ver2, _ := d.RosterVersion(1)
	if ver2 == ver {
		t.Fatalf("version should change after bump: %s", ver)
	}
	if _, err := d.LiveDraw(1, 100, "k2", 1, ver2); err != xerr.ErrStalePool {
		t.Fatalf("want stale after bump got %v", err)
	}
}

func TestLiveDrawConcurrentNoRepeat(t *testing.T) {
	d := setup(t)
	loadRunningMeta(t, d, 2)
	var pool []uint64
	for i := 1; i <= 20; i++ {
		pool = append(pool, uint64(i))
	}
	if err := d.LiveRebuildPool(2, 100, "1", pool); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	got := map[uint64]int{}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := d.LiveDraw(2, 100, "batch-"+string(rune('a'+i)), 2, "1")
			if err != nil {
				t.Errorf("draw: %v", err)
				return
			}
			mu.Lock()
			for _, id := range res.WinnerIDs {
				got[id]++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(got) != 20 {
		t.Fatalf("want 20 distinct winners got %d (repeat or loss?)", len(got))
	}
	for id, n := range got {
		if n != 1 {
			t.Fatalf("participant %d won %d times", id, n)
		}
	}
	// 池已空
	_, err := d.LiveDraw(2, 100, "batch-z", 1, "1")
	if err != xerr.ErrInsufficient {
		t.Fatalf("want insufficient got %v", err)
	}
}

func TestLiveDrawIdempotentReplay(t *testing.T) {
	d := setup(t)
	loadRunningMeta(t, d, 3)
	_ = d.LiveRebuildPool(3, 100, "1", []uint64{7, 8, 9})
	first, err := d.LiveDraw(3, 100, "same-key", 2, "1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.LiveDraw(3, 100, "same-key", 2, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate {
		t.Fatal("replay should be marked duplicate")
	}
	sort.Slice(first.WinnerIDs, func(i, j int) bool { return first.WinnerIDs[i] < first.WinnerIDs[j] })
	sort.Slice(second.WinnerIDs, func(i, j int) bool { return second.WinnerIDs[i] < second.WinnerIDs[j] })
	if len(first.WinnerIDs) != len(second.WinnerIDs) {
		t.Fatal("replay winner count mismatch")
	}
	for i := range first.WinnerIDs {
		if first.WinnerIDs[i] != second.WinnerIDs[i] {
			t.Fatalf("replay winners mismatch: %v vs %v", first.WinnerIDs, second.WinnerIDs)
		}
	}
}

func TestLiveDrawStatusGuards(t *testing.T) {
	d := setup(t)
	loadRunningMeta(t, d, 4)
	_ = d.LiveRebuildPool(4, 100, "1", []uint64{1, 2, 3})
	if err := d.SetStatus(4, "paused"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.LiveDraw(4, 100, "k1", 1, "1"); err != xerr.ErrPaused {
		t.Fatalf("want paused got %v", err)
	}
	_ = d.SetStatus(4, "ended")
	if _, err := d.LiveDraw(4, 100, "k2", 1, "1"); err != xerr.ErrEnded {
		t.Fatalf("want ended got %v", err)
	}
}

func TestLiveUndo(t *testing.T) {
	d := setup(t)
	loadRunningMeta(t, d, 5)
	_ = d.LiveRebuildPool(5, 100, "1", []uint64{1, 2, 3, 4})
	res, err := d.LiveDraw(5, 100, "batch-1", 2, "1")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.LiveUndo(5, res.DrawId); err != nil {
		t.Fatal(err)
	}
	// 重放同幂等键：能拿到原结果但标记为已取消
	replay, err := d.LiveDraw(5, 100, "batch-1", 2, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Duplicate || !replay.Undone {
		t.Fatalf("want undone replay got %+v", replay)
	}
	// 重复取消报错
	if err := d.LiveUndo(5, res.DrawId); err != xerr.ErrUndone {
		t.Fatalf("want undone err got %v", err)
	}
	// 不存在的批次
	if err := d.LiveUndo(5, "no-such"); err == nil {
		t.Fatal("want not found for unknown batch")
	}
}

func TestScheduledNoRecycle(t *testing.T) {
	participants := []uint64{1, 2}
	prizes := []engine.PrizeSpec{
		{ID: 10, Kind: "physical", Stock: 5, Name: "phone"},
	}
	wins, err := engine.AssignScheduled(participants, prizes, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if len(wins) != 2 {
		t.Fatalf("want 2 winners got %d (should not recycle leftover stock)", len(wins))
	}
}

func TestScheduledIdempotentSeed(t *testing.T) {
	participants := []uint64{1, 2, 3, 4, 5}
	prizes := []engine.PrizeSpec{{ID: 1, Kind: "virtual", Stock: 2, Name: "c"}}
	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	a, _ := engine.AssignScheduled(participants, prizes, seed)
	b, _ := engine.AssignScheduled(participants, prizes, seed)
	if len(a) != 2 || len(b) != 2 {
		t.Fatal(len(a), len(b))
	}
	if a[0].ParticipantID != b[0].ParticipantID || a[1].ParticipantID != b[1].ParticipantID {
		t.Fatalf("seed not deterministic")
	}
}
