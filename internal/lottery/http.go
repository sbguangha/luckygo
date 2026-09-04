package lottery

import (
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
)

var errWechat = errors.New("wechat oauth failed")

type joinReq struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	StaffNo  string `json:"staff_no"`
}

type drawReq struct {
	N int `json:"n"`
}

func NewEngine(hub *Hub) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/lottery/join", hub.handleJoin)
	r.GET("/api/lottery/participants", hub.handleParticipants)
	r.POST("/api/lottery/draw", hub.handleDraw)
	r.POST("/api/lottery/seed-mock", hub.handleSeedMock)
	r.GET("/api/lottery/session", hub.handleSession)
	r.GET("/api/lottery/me", hub.handleMe)
	r.GET("/api/lottery/wechat/login", hub.handleWechatLogin)
	r.GET("/api/lottery/wechat/callback", hub.handleWechatCallback)
	r.GET("/api/lottery/qr.png", hub.handleQR)
	return r
}

func (h *Hub) handleSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"wechatEnabled": h.wechatEnabled(),
			"nickname":      readNicknameCookie(c),
			"wechatBound":   readOpenID(c) != "",
			"count":         h.Len(),
			"publicJoinUrl": h.publicJoinURL(),
		},
	})
}

func (h *Hub) handleMe(c *gin.Context) {
	uid := strings.TrimSpace(c.Query("user_id"))
	if oid := readOpenID(c); oid != "" {
		uid = "wx:" + oid
	}
	if uid == "" {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{"inPool": false, "won": false, "name": ""},
		})
		return
	}
	m, ok := h.lookupMember(uid)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{"inPool": false, "won": false, "name": ""},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"inPool": m.Status == "active",
			"won":    m.Status == "won",
			"name":   m.Name,
		},
	})
}

func (h *Hub) handleJoin(c *gin.Context) {
	if !h.AllowJoin() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code": 503,
			"msg":  "现在排队的人太多，请稍后再试",
		})
		return
	}
	var req joinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请填写姓名后再参加"})
		return
	}
	name := strings.TrimSpace(req.UserName)
	staff := strings.TrimSpace(req.StaffNo)
	if !validName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请填写 2 到 16 个字的真实姓名"})
		return
	}
	if !validStaffNo(staff) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "工号格式不正确"})
		return
	}

	openid := readOpenID(c)
	userID := strings.TrimSpace(req.UserID)
	source := "form"
	if openid != "" {
		userID = "wx:" + openid
		source = "wechat"
	}
	if userID == "" {
		userID = fmt.Sprintf("web-%d-%04x", time.Now().UnixNano(), rand.Uint32()&0xffff)
	}

	inserted, err := h.JoinMember(Member{
		UserID:  userID,
		Name:    name,
		StaffNo: staff,
		Source:  source,
		OpenID:  openid,
	})
	if errors.Is(err, errAlreadyWon) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "你已经抽过奖了，把机会留给同事吧"})
		return
	}
	if err != nil {
		log.Printf("lottery join persist: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "报名通道繁忙，请稍后再试"})
		return
	}
	msg := "参与成功"
	if !inserted {
		msg = "你已在抽奖名单中"
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  msg,
		"data": gin.H{"already": !inserted, "name": name},
	})
}

func (h *Hub) handleParticipants(c *gin.Context) {
	if err := h.replaceActiveFromDB(); err != nil {
		log.Printf("lottery reload roster: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"names":         h.Names(),
			"count":         h.Len(),
			"publicJoinUrl": h.publicJoinURL(),
		},
	})
}

func (h *Hub) handleDraw(c *gin.Context) {
	var req drawReq
	_ = c.ShouldBindJSON(&req)
	n := req.N
	if n <= 0 {
		n = 3
	}

	h.drawMu.Lock()
	defer h.drawMu.Unlock()

	pool := h.snapshotActive()
	if len(pool) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "还没有人在名单里，请先让同事扫码加入"})
		return
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	winners := make([]string, 0, n)
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		winners = append(winners, pool[i].Name)
		ids = append(ids, pool[i].UserID)
	}
	h.removeWinners(ids)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"winners":   winners,
			"remaining": h.Len(),
		},
	})
}

func (h *Hub) handleSeedMock(c *gin.Context) {
	type seedReq struct {
		Count int `json:"count"`
	}
	var req seedReq
	_ = c.ShouldBindJSON(&req)
	n := req.Count
	if n <= 0 {
		n = 30
	}
	if n > 80 {
		n = 80
	}
	added := 0
	for i := 1; i <= n; i++ {
		name := mockName(i)
		ok, err := h.JoinMember(Member{
			UserID: fmt.Sprintf("mock-%02d", i),
			Name:   name,
			Source: "mock",
		})
		if errors.Is(err, errAlreadyWon) {
			continue
		}
		if err != nil {
			log.Printf("seed mock: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "演示名单写入失败，请稍后再试"})
			return
		}
		if ok {
			added++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "演示名单已就绪",
		"data": gin.H{"added": added, "count": h.Len()},
	})
}

func mockName(i int) string {
	base := []string{
		"张伟", "王芳", "李娜", "刘强", "陈敏", "杨洋", "黄磊", "赵静",
		"周杰", "吴婷", "徐鹏", "孙丽", "马超", "朱琳", "胡军", "郭倩",
		"何勇", "高燕", "林峰", "罗雪", "郑浩", "梁晨", "谢芳", "宋杰",
		"唐伟", "韩雪", "冯俊", "邓丽", "曹阳", "彭涛",
	}
	if i <= len(base) {
		return base[i-1]
	}
	return base[(i-1)%len(base)] + fmt.Sprintf("%d", i)
}

func (h *Hub) handleQR(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("u"))
	if raw == "" {
		raw = h.joinPageURL()
	}
	if !allowedJoinQR(raw, c.Request.Host, c.GetHeader("Origin"), h.effectivePublicBase()) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "二维码地址无效"})
		return
	}
	png, err := qrcode.Encode(raw, qrcode.Medium, 320)
	if err != nil {
		log.Printf("qr encode: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "二维码暂时无法生成"})
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}

func allowedJoinQR(raw, reqHost, origin, publicBase string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Path != "/join" || u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == reqHost {
		return true
	}
	if origin != "" {
		if o, err := url.Parse(origin); err == nil && o.Host != "" && u.Host == o.Host {
			return true
		}
	}
	if pb, err := url.Parse(publicBase); err == nil && pb.Host != "" && u.Host == pb.Host {
		return true
	}
	if isTunnelJoinHost(u.Host) {
		return true
	}
	return isPrivateOrLocal(u.Host)
}

var joinTunnelSuffixes = []string{
	".cpolar.cn", ".cpolar.io", ".cpolar.top", ".cpolar.vip",
	".ngrok-free.app", ".ngrok.io", ".ngrok.app", ".ngrok.dev",
	".trycloudflare.com",
	".loca.lt",
	".serveo.net",
	".pinggy.link", ".pinggy.io",
}

func isTunnelJoinHost(hostport string) bool {
	host := strings.ToLower(hostport)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	for _, suf := range joinTunnelSuffixes {
		if strings.HasSuffix(host, suf) {
			return true
		}
	}
	return false
}

func isPrivateOrLocal(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback()
}
