package types

type ActivityBrief struct {
	Id           uint64 `json:"id,string"`
	PublicId     string `json:"publicId"`
	Title        string `json:"title"`
	Mode         string `json:"mode"`
	RosterSource string `json:"rosterSource"`
	Status       string `json:"status"`
	StartAt      int64  `json:"startAt"`
	EndAt        int64  `json:"endAt"`
	PlayUrl      string `json:"playUrl"`
	TenantName   string `json:"tenantName"`
}

type ActivityDetail struct {
	ActivityBrief
	Prizes       []PrizeView `json:"prizes"`
	ParticipantN int64       `json:"participantN"`
	WinN         int64       `json:"winN"`
	UiConfig     *UiConfig   `json:"uiConfig,omitempty"`
}

// UiConfig 大屏/活动页装修配置（对应 log-lottery 界面配置的可落库子集）。
// 注意：go-zero mapping 只认 `,optional`（omitempty 不生效），指针类型也有坑，全用值类型。
type UiConfig struct {
	TopTitle       string `json:"topTitle,optional"`
	RowCount       int    `json:"rowCount,optional"`
	CardWidth      int    `json:"cardWidth,optional"`
	CardHeight     int    `json:"cardHeight,optional"`
	CardColor      string `json:"cardColor,optional"`
	LuckyCardColor string `json:"luckyCardColor,optional"`
	TextColor      string `json:"textColor,optional"`
	PatternColor   string `json:"patternColor,optional"`
	Background     string `json:"background,optional"`
	ShowAvatar     bool   `json:"showAvatar,optional"`
	ShowPrizeList  bool   `json:"showPrizeList,optional"`
	PatternList    []int  `json:"patternList,optional"`
}

type BlacklistReq struct {
	Account string `json:"account"`
	Reason  string `json:"reason,optional"`
}

type PrizeInput struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Stock    int    `json:"stock"`
	PerRound int    `json:"perRound,optional"`
	IsAll    bool   `json:"isAll,optional"`
	ImageUrl string `json:"imageUrl,optional"`
}

type CreateActivityReq struct {
	Title          string       `json:"title"`
	Mode           string       `json:"mode"`
	RosterSource   string       `json:"rosterSource,optional"`
	Timezone       string       `json:"timezone,optional"`
	StartAt        int64        `json:"startAt"`
	EndAt          int64        `json:"endAt"`
	MaxEnrollments int          `json:"maxEnrollments,optional"`
	Prizes         []PrizeInput `json:"prizes"`
}

type UpdateActivityReq struct {
	Id             uint64       `path:"id"`
	Title          string       `json:"title"`
	Mode           string       `json:"mode"`
	RosterSource   string       `json:"rosterSource,optional"`
	Timezone       string       `json:"timezone,optional"`
	StartAt        int64        `json:"startAt"`
	EndAt          int64        `json:"endAt"`
	MaxEnrollments int          `json:"maxEnrollments,optional"`
	Prizes         []PrizeInput `json:"prizes"`
}

type UpdateUiConfigReq struct {
	Id     uint64   `path:"id"`
	Config UiConfig `json:"config"`
}

type EnrollReq struct {
	PublicId string `json:"publicId"`
}

type FeedResp struct {
	List []WinnerItem `json:"list"`
}

type FillAddressReq struct {
	PrizeToken   string `json:"prizeToken"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	Address      string `json:"address"`
}

type IdPathReq struct {
	Id uint64 `path:"id"`
}

type IdResp struct {
	Id uint64 `json:"id,string"`
}

type ListActivityReq struct {
	Status string `form:"status,optional"`
}

type ListActivityResp struct {
	List []ActivityBrief `json:"list"`
}

type LoginReq struct {
	TenantName string `json:"tenantName"`
	Account    string `json:"account"`
	Password   string `json:"password"`
}

type LoginResp struct {
	Token    string `json:"token"`
	Role     string `json:"role"`
	Nickname string `json:"nickname"`
	Tenant   string `json:"tenant"`
}

type MyPrizeItem struct {
	PrizeName  string `json:"prizeName"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	RedeemCode string `json:"redeemCode,optional"`
	WonAt      int64  `json:"wonAt"`
	Activity   string `json:"activity"`
}

type MyPrizesResp struct {
	List []MyPrizeItem `json:"list"`
}

type OkResp struct {
	Ok bool `json:"ok"`
}

// ---------- 参与者名单 ----------

type ParticipantInput struct {
	Uid        string `json:"uid"`
	Name       string `json:"name"`
	Department string `json:"department,optional"`
	Identity   string `json:"identity,optional"`
	AvatarUrl  string `json:"avatarUrl,optional"`
}

type ImportParticipantsReq struct {
	Id   uint64             `path:"id"`
	Rows []ParticipantInput `json:"rows"`
}

type ImportResp struct {
	Total  int `json:"total"`
	Failed int `json:"failed"`
}

type ParticipantItem struct {
	Id         uint64 `json:"id,string"`
	Uid        string `json:"uid"`
	Name       string `json:"name"`
	Department string `json:"department"`
	Identity   string `json:"identity"`
	AvatarUrl  string `json:"avatarUrl"`
	Source     string `json:"source"`
	IsWin      bool   `json:"isWin"`
	CreatedAt  int64  `json:"createdAt"`
}

type ParticipantsResp struct {
	List []ParticipantItem `json:"list"`
}

type ParticipantPathReq struct {
	Id  uint64 `path:"id"`
	Pid uint64 `path:"pid"`
}

// ---------- 现场大屏抽取 ----------

type LiveDrawReq struct {
	Id             uint64 `path:"id"`
	PrizeId        uint64 `json:"prizeId,string"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type LiveWinner struct {
	ParticipantId uint64 `json:"participantId,string"`
	Uid           string `json:"uid"`
	Name          string `json:"name"`
	Department    string `json:"department"`
	Identity      string `json:"identity"`
	AvatarUrl     string `json:"avatarUrl"`
}

type LiveDrawResp struct {
	DrawId    string       `json:"drawId"`
	PrizeId   uint64       `json:"prizeId,string"`
	PrizeName string       `json:"prizeName"`
	Kind      string       `json:"kind"`
	Winners   []LiveWinner `json:"winners"`
	Remain    int          `json:"remain"` // 该奖项剩余名额
}

type LiveUndoReq struct {
	Id     uint64 `path:"id"`
	DrawId string `json:"drawId"`
}

// ---------- 中奖名单 ----------

type WinnerItem struct {
	Nickname  string `json:"nickname"`
	PrizeName string `json:"prizeName"`
	Kind      string `json:"kind"`
	WonAt     int64  `json:"wonAt"`
}

type WinnersResp struct {
	List []WinnerItem `json:"list"`
}

type AdminWinnerItem struct {
	ParticipantId uint64 `json:"participantId,string"`
	Uid           string `json:"uid"`
	Name          string `json:"name"`
	Department    string `json:"department"`
	PrizeName     string `json:"prizeName"`
	Kind          string `json:"kind"`
	PrizeToken    string `json:"prizeToken"`
	Source        string `json:"source"`
	RedeemStatus  string `json:"redeemStatus"`
	WonAt         int64  `json:"wonAt"`
}

type AdminWinnersResp struct {
	List []AdminWinnerItem `json:"list"`
}

type OfflineRedeemReq struct {
	Id         uint64 `path:"id"`
	PrizeToken string `json:"prizeToken"`
}

type UploadResp struct {
	Url string `json:"url"`
}

type PrizeView struct {
	Id       uint64 `json:"id,string"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Stock    int    `json:"stock"`
	PerRound int    `json:"perRound"`
	IsAll    bool   `json:"isAll"`
	ImageUrl string `json:"imageUrl"`
	Remain   int    `json:"remain"`
}

type PublicIdReq struct {
	PublicId string `path:"publicId"`
}

type RedeemReq struct {
	Code string `json:"code"`
}

type RegisterTenantReq struct {
	TenantName string `json:"tenantName"`
	Account    string `json:"account"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname,optional"`
}

type UserRegisterReq struct {
	PublicId string `json:"publicId"`
	Account  string `json:"account"`
	Password string `json:"password"`
	Nickname string `json:"nickname,optional"`
}
