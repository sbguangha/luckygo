package lottery

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNamesJoinOrderStable(t *testing.T) {
	h := NewHubWithRate(10000, 10000)
	h.JoinMember(Member{UserID: "u2", Name: "李四", JoinedAt: 20})
	h.JoinMember(Member{UserID: "u1", Name: "张三", JoinedAt: 10})
	h.JoinMember(Member{UserID: "u3", Name: "王五", JoinedAt: 30})
	got := strings.Join(h.Names(), ",")
	if got != "张三,李四,王五" {
		t.Fatalf("want join order, got %s", got)
	}
	if strings.Join(h.Names(), ",") != got {
		t.Fatal("names order should stay the same across calls")
	}
}

func TestJoinDedupeAndNames(t *testing.T) {
	h := NewHubWithRate(10000, 10000)
	if !h.Join("u1", "张三") {
		t.Fatal("first join should insert")
	}
	if h.Join("u1", "张三") {
		t.Fatal("duplicate user_id should be ignored")
	}
	h.Join("u2", "李四")
	names := h.Names()
	if len(names) != 2 {
		t.Fatalf("got %d names: %v", len(names), names)
	}
}

func TestConcurrentJoin(t *testing.T) {
	h := NewHubWithRate(100000, 100000)
	const n = 500
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			h.Join(fmt.Sprintf("u-%d", i), fmt.Sprintf("员工%02d", i))
		}(i)
	}
	wg.Wait()
	if h.Len() != n {
		t.Fatalf("want %d got %d", n, h.Len())
	}
}

func TestDrawRemovesWinnersOnly(t *testing.T) {
	h := NewHubWithRate(10000, 10000)
	h.Join("a", "王五")
	h.Join("b", "赵六")
	h.Join("c", "钱七")
	h.Join("d", "孙八")

	engine := NewEngine(h)
	w := performJSON(engine, "POST", "/api/lottery/draw", `{"n":3}`)
	if w.Code != 200 {
		t.Fatalf("draw status %d body %s", w.Code, w.Body.String())
	}
	if h.Len() != 1 {
		t.Fatalf("remaining pool should be 1, got %d", h.Len())
	}
	if !strings.Contains(w.Body.String(), `"remaining":1`) {
		t.Fatalf("draw should report remaining: %s", w.Body.String())
	}
	left := map[string]bool{}
	for _, n := range h.Names() {
		left[n] = true
	}
	if left["王五"] && left["赵六"] && left["钱七"] && left["孙八"] {
		t.Fatal("winners should have left the pool")
	}
}

func TestValidName(t *testing.T) {
	if !validName("张三") || validName("A") || validName("<script>") {
		t.Fatal("name validation")
	}
}

func TestJoinRateLimit(t *testing.T) {
	h := NewHubWithRate(50, 50)
	engine := NewEngine(h)
	var limited atomic.Int32
	var ok atomic.Int32
	var wg sync.WaitGroup
	const n = 400
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"user_id":"u-%d","user_name":"员工%02d"}`, i, i)
			w := performJSON(engine, "POST", "/api/lottery/join", body)
			switch w.Code {
			case 200:
				ok.Add(1)
			case 503:
				limited.Add(1)
			default:
				t.Errorf("unexpected status %d body %s", w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()
	if limited.Load() == 0 {
		t.Fatalf("expected some 503 under burst, ok=%d limited=%d", ok.Load(), limited.Load())
	}
	if ok.Load() == 0 {
		t.Fatal("expected some successful joins")
	}
}
