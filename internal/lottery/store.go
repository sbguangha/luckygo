package lottery

import (
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"
)

type Member struct {
	UserID   string
	Name     string
	StaffNo  string
	Source   string // form | wechat | mock
	OpenID   string
	JoinedAt int64
	Status   string // active | won
}

type Conf struct {
	PublicBase      string
	WechatAppId     string
	WechatAppSecret string
}

type Hub struct {
	members sync.Map
	limiter *rate.Limiter
	drawMu  sync.Mutex
	persist *rosterDB
	conf    Conf
}

func NewHub() *Hub {
	return NewHubWithRate(100, 100)
}

func NewHubWithRate(perSecond, burst int) *Hub {
	if perSecond <= 0 {
		perSecond = 100
	}
	if burst <= 0 {
		burst = perSecond
	}
	return &Hub{
		limiter: rate.NewLimiter(rate.Limit(perSecond), burst),
		conf:    Conf{PublicBase: "http://localhost:5173"},
	}
}

func (h *Hub) SetConf(c Conf) {
	h.conf = c
}

func (h *Hub) AllowJoin() bool {
	return h.limiter.Allow()
}

func (h *Hub) Join(userID, name string) bool {
	ok, _ := h.JoinMember(Member{UserID: userID, Name: name, Source: "form"})
	return ok
}

func (h *Hub) JoinMember(m Member) (inserted bool, err error) {
	m.UserID = strings.TrimSpace(m.UserID)
	m.Name = strings.TrimSpace(m.Name)
	m.StaffNo = strings.TrimSpace(m.StaffNo)
	if m.Source == "" {
		m.Source = "form"
	}
	if m.Status == "" {
		m.Status = "active"
	}
	if m.JoinedAt == 0 {
		m.JoinedAt = time.Now().Unix()
	}
	if cur, loaded := h.members.Load(m.UserID); loaded {
		exist, _ := cur.(Member)
		if exist.Status == "won" {
			return false, errAlreadyWon
		}
		return false, nil
	}
	if h.persist != nil {
		inserted, err = h.persist.upsertActive(m)
		if err != nil {
			return false, err
		}
		if !inserted {
			if won, _ := h.persist.isWon(m.UserID); won {
				return false, errAlreadyWon
			}
			if existing, ok, e := h.persist.get(m.UserID); e == nil && ok {
				h.members.Store(m.UserID, existing)
			}
			return false, nil
		}
	}
	h.members.Store(m.UserID, m)
	return true, nil
}

func (h *Hub) lookupMember(userID string) (Member, bool) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Member{}, false
	}
	if v, ok := h.members.Load(userID); ok {
		m, _ := v.(Member)
		return m, true
	}
	if h.persist == nil {
		return Member{}, false
	}
	m, ok, err := h.persist.get(userID)
	if err != nil || !ok {
		return Member{}, false
	}
	return m, true
}

func (h *Hub) Names() []string {
	ms := h.orderedActive()
	names := make([]string, 0, len(ms))
	for _, m := range ms {
		names = append(names, m.Name)
	}
	return names
}

func (h *Hub) orderedActive() []Member {
	out := h.snapshotActive()
	sort.Slice(out, func(i, j int) bool {
		if out[i].JoinedAt != out[j].JoinedAt {
			return out[i].JoinedAt < out[j].JoinedAt
		}
		if out[i].UserID != out[j].UserID {
			return out[i].UserID < out[j].UserID
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (h *Hub) Len() int {
	n := 0
	h.members.Range(func(_, v any) bool {
		m, ok := v.(Member)
		if ok && m.Status != "won" {
			n++
		}
		return true
	})
	return n
}

func (h *Hub) snapshotActive() []Member {
	out := make([]Member, 0, 64)
	h.members.Range(func(_, v any) bool {
		m, ok := v.(Member)
		if ok && m.Status != "won" && m.Name != "" {
			out = append(out, m)
		}
		return true
	})
	return out
}

func (h *Hub) removeWinners(ids []string) {
	for _, id := range ids {
		h.members.Delete(id)
	}
	if h.persist != nil {
		_ = h.persist.markWon(ids)
	}
}

func validName(name string) bool {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 2 || n > 16 {
		return false
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "http") || strings.ContainsAny(name, "<>\"'") {
		return false
	}
	return true
}

func validStaffNo(s string) bool {
	if s == "" {
		return true
	}
	if utf8.RuneCountInString(s) > 20 {
		return false
	}
	return !strings.ContainsAny(s, "<>\"'")
}
