package engine_test

import (
	"context"
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

func TestValidateWeight(t *testing.T) {
	err := engine.ValidatePrizes([]engine.PrizeSpec{
		{ID: 1, Name: "phone", Kind: "physical", Stock: 1, Weight: 100},
		{ID: 2, Name: "thanks", Kind: "thank_you", Stock: 9, Weight: 9900},
	}, "instant")
	if err != nil {
		t.Fatal(err)
	}
	err = engine.ValidatePrizes([]engine.PrizeSpec{
		{ID: 1, Name: "phone", Kind: "physical", Stock: 1, Weight: 1},
	}, "instant")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildBucketLength(t *testing.T) {
	items, err := engine.BuildBucket([]engine.PrizeSpec{
		{ID: 1, Name: "phone", Kind: "physical", Stock: 2, Weight: 2000},
		{ID: 2, Name: "thanks", Kind: "thank_you", Stock: 8, Weight: 8000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 10 {
		t.Fatalf("len=%d", len(items))
	}
}

func TestNoOversell(t *testing.T) {
	d := setup(t)
	ctx := context.Background()
	items, err := engine.BuildBucket([]engine.PrizeSpec{
		{ID: 1, Name: "phone", Kind: "physical", Stock: 3, Weight: 3000},
		{ID: 2, Name: "thanks", Kind: "thank_you", Stock: 7, Weight: 7000},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if err := d.LoadBucket(ctx, 1, engine.Meta{
		Status: "running", StartAt: now - 10, EndAt: now + 3600, MaxDraws: 50,
	}, items, map[string]int{"1": 3, "2": 7}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	count := map[uint64]int{}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := d.Draw(1, uint64(i+1), "k")
			if err != nil {
				if err == xerr.ErrEmpty || err == xerr.ErrBusy {
					return
				}
				t.Errorf("draw: %v", err)
				return
			}
			d.ClearInflight(1, uint64(i+1))
			mu.Lock()
			count[res.Item.PrizeID]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if count[1] != 3 {
		t.Fatalf("prize 1 count=%d want 3 (oversell?)", count[1])
	}
	if count[1]+count[2] != 10 {
		t.Fatalf("total=%d", count[1]+count[2])
	}
}

func TestQuotaAndIdempotency(t *testing.T) {
	d := setup(t)
	ctx := context.Background()
	items, _ := engine.BuildBucket([]engine.PrizeSpec{
		{ID: 1, Name: "p", Kind: "virtual", Stock: 5, Weight: 5000},
		{ID: 2, Name: "t", Kind: "thank_you", Stock: 5, Weight: 5000},
	})
	now := time.Now().Unix()
	_ = d.LoadBucket(ctx, 2, engine.Meta{Status: "running", StartAt: now - 1, EndAt: now + 3600, MaxDraws: 1}, items, map[string]int{"1": 5, "2": 5})
	first, err := d.Draw(2, 9, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	d.ClearInflight(2, 9)
	second, err := d.Draw(2, 9, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate || second.Raw != first.Raw {
		t.Fatalf("idempotency failed: %+v %+v", first, second)
	}
	d.ClearInflight(2, 9)
	_, err = d.Draw(2, 9, "other-key")
	if err != xerr.ErrQuota {
		t.Fatalf("want quota got %v", err)
	}
}

func TestPause(t *testing.T) {
	d := setup(t)
	ctx := context.Background()
	items, _ := engine.BuildBucket([]engine.PrizeSpec{
		{ID: 1, Name: "p", Kind: "virtual", Stock: 1, Weight: 1000},
		{ID: 2, Name: "t", Kind: "thank_you", Stock: 1, Weight: 9000},
	})
	now := time.Now().Unix()
	_ = d.LoadBucket(ctx, 3, engine.Meta{Status: "running", StartAt: now - 1, EndAt: now + 3600, MaxDraws: 3}, items, map[string]int{"1": 1, "2": 1})
	if err := d.SetStatus(3, "paused"); err != nil {
		t.Fatal(err)
	}
	_, err := d.Draw(3, 1, "k1")
	if err != xerr.ErrPaused {
		t.Fatalf("got %v", err)
	}
}

func TestScheduledNoRecycle(t *testing.T) {
	users := []uint64{1, 2}
	prizes := []engine.PrizeSpec{
		{ID: 10, Kind: "physical", Stock: 5, Weight: 5000, Name: "phone"},
		{ID: 11, Kind: "thank_you", Stock: 100, Weight: 5000, Name: "t"},
	}
	wins, err := engine.AssignScheduled(users, prizes, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if len(wins) != 2 {
		t.Fatalf("want 2 winners got %d (should not recycle leftover stock)", len(wins))
	}
}

func TestScheduledIdempotentSeed(t *testing.T) {
	users := []uint64{1, 2, 3, 4, 5}
	prizes := []engine.PrizeSpec{{ID: 1, Kind: "virtual", Stock: 2, Weight: 10000, Name: "c"}}
	seed := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	a, _ := engine.AssignScheduled(users, prizes, seed)
	b, _ := engine.AssignScheduled(users, prizes, seed)
	if len(a) != 2 || len(b) != 2 {
		t.Fatal(len(a), len(b))
	}
	if a[0].UserID != b[0].UserID || a[1].UserID != b[1].UserID {
		t.Fatalf("seed not deterministic")
	}
}
