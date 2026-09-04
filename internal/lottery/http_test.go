package lottery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func performJSON(engine http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestParticipantsAndJoinHTTP(t *testing.T) {
	h := NewHubWithRate(10000, 10000)
	engine := NewEngine(h)

	w := performJSON(engine, "POST", "/api/lottery/join", `{"user_id":"e01","user_name":"周九"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"参与成功"`) {
		t.Fatalf("join: %d %s", w.Code, w.Body.String())
	}

	w = performJSON(engine, "GET", "/api/lottery/participants", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "周九") {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
}

func TestMeInPool(t *testing.T) {
	h := NewHubWithRate(10000, 10000)
	h.Join("u-me", "周九")
	w := performJSON(NewEngine(h), "GET", "/api/lottery/me?user_id=u-me", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"inPool":true`) || !strings.Contains(w.Body.String(), "周九") {
		t.Fatalf("me: %d %s", w.Code, w.Body.String())
	}
	w = performJSON(NewEngine(h), "GET", "/api/lottery/me?user_id=nobody", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"inPool":false`) {
		t.Fatalf("me missing: %d %s", w.Code, w.Body.String())
	}
}

func TestAllowedJoinQR(t *testing.T) {
	if !allowedJoinQR("http://192.168.1.3:5173/join", "127.0.0.1:8888", "http://192.168.1.3:5173", "http://localhost:5173") {
		t.Fatal("lan join url should be allowed")
	}
	if allowedJoinQR("https://evil.example/join", "127.0.0.1:8888", "http://localhost:5173", "http://localhost:5173") {
		t.Fatal("foreign host should be rejected")
	}
	if !allowedJoinQR("https://abc-xyz.trycloudflare.com/join", "127.0.0.1:8888", "http://192.168.1.3:5173", "") {
		t.Fatal("cloudflare tunnel join url should be allowed")
	}
	if !allowedJoinQR("https://foo.cpolar.cn/join", "127.0.0.1:8888", "", "https://foo.cpolar.cn") {
		t.Fatal("cpolar join url should be allowed")
	}
}

func TestSessionPublicJoinURLFromEnv(t *testing.T) {
	t.Setenv("LUCKYGO_PUBLIC_BASE", "https://abc-xyz.trycloudflare.com")
	h := NewHub()
	w := performJSON(NewEngine(h), "GET", "/api/lottery/session", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"publicJoinUrl":"https://abc-xyz.trycloudflare.com/join"`) {
		t.Fatalf("session public join: %d %s", w.Code, w.Body.String())
	}
	w = performJSON(NewEngine(h), "GET", "/api/lottery/participants", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"publicJoinUrl":"https://abc-xyz.trycloudflare.com/join"`) {
		t.Fatalf("participants public join: %d %s", w.Code, w.Body.String())
	}
}
