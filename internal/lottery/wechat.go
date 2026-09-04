package lottery

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	cookieOpenID   = "lg_wx_openid"
	cookieNickname = "lg_wx_nickname"
)

var httpClient = &http.Client{Timeout: 6 * time.Second}

func (h *Hub) wechatEnabled() bool {
	return strings.TrimSpace(h.conf.WechatAppId) != "" && strings.TrimSpace(h.conf.WechatAppSecret) != ""
}

func (h *Hub) joinPageURL() string {
	base := h.effectivePublicBase()
	if base == "" {
		base = "http://localhost:5173"
	}
	return base + "/join"
}

func (h *Hub) callbackURL() string {
	base := h.effectivePublicBase()
	if base == "" {
		base = "http://localhost:5173"
	}
	return base + "/api/lottery/wechat/callback"
}

func (h *Hub) handleWechatLogin(c *gin.Context) {
	if !h.wechatEnabled() {
		c.Redirect(http.StatusFound, h.joinPageURL())
		return
	}
	q := url.Values{}
	q.Set("appid", h.conf.WechatAppId)
	q.Set("redirect_uri", h.callbackURL())
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_userinfo")
	q.Set("state", "luckygo")
	c.Redirect(http.StatusFound, "https://open.weixin.qq.com/connect/oauth2/authorize?"+q.Encode()+"#wechat_redirect")
}

func (h *Hub) handleWechatCallback(c *gin.Context) {
	if !h.wechatEnabled() {
		c.Redirect(http.StatusFound, h.joinPageURL())
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.Redirect(http.StatusFound, h.joinPageURL())
		return
	}
	openid, nickname, err := h.exchangeWechat(code)
	if err != nil || openid == "" {
		log.Printf("wechat oauth failed")
		c.Redirect(http.StatusFound, h.joinPageURL()+"?wx=fail")
		return
	}
	h.setWechatCookies(c, openid, nickname)
	c.Redirect(http.StatusFound, h.joinPageURL())
}

func (h *Hub) exchangeWechat(code string) (openid, nickname string, err error) {
	tokenURL := "https://api.weixin.qq.com/sns/oauth2/access_token?" + url.Values{
		"appid":      {h.conf.WechatAppId},
		"secret":     {h.conf.WechatAppSecret},
		"code":       {code},
		"grant_type": {"authorization_code"},
	}.Encode()
	var tok struct {
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err = getJSON(tokenURL, &tok); err != nil {
		return "", "", err
	}
	if tok.ErrCode != 0 || tok.OpenID == "" {
		return "", "", errWechat
	}
	infoURL := "https://api.weixin.qq.com/sns/userinfo?" + url.Values{
		"access_token": {tok.AccessToken},
		"openid":       {tok.OpenID},
		"lang":         {"zh_CN"},
	}.Encode()
	var info struct {
		Nickname string `json:"nickname"`
		OpenID   string `json:"openid"`
	}
	if err = getJSON(infoURL, &info); err != nil {
		return tok.OpenID, "", nil
	}
	nick := strings.TrimSpace(info.Nickname)
	if !validName(nick) {
		nick = ""
	}
	return tok.OpenID, nick, nil
}

func (h *Hub) setWechatCookies(c *gin.Context, openid, nickname string) {
	secure := c.Request.TLS != nil
	http.SetCookie(c.Writer, &http.Cookie{
		Name: cookieOpenID, Value: openid, Path: "/", MaxAge: 86400,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure,
	})
	if nickname != "" {
		http.SetCookie(c.Writer, &http.Cookie{
			Name: cookieNickname, Value: url.QueryEscape(nickname), Path: "/", MaxAge: 86400,
			SameSite: http.SameSiteLaxMode, Secure: secure,
		})
	}
}

func readOpenID(c *gin.Context) string {
	v, err := c.Cookie(cookieOpenID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

func readNicknameCookie(c *gin.Context) string {
	v, err := c.Cookie(cookieNickname)
	if err != nil {
		return ""
	}
	s, e := url.QueryUnescape(v)
	if e != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func getJSON(rawURL string, dest any) error {
	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}
