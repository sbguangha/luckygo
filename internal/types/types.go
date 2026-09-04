package types

type ActivityBrief struct {
	Id              uint64 `json:"id,string"`
	PublicId        string `json:"publicId"`
	Title           string `json:"title"`
	Mode            string `json:"mode"`
	Status          string `json:"status"`
	StartAt         int64  `json:"startAt"`
	EndAt           int64  `json:"endAt"`
	MaxDrawsPerUser int    `json:"maxDrawsPerUser"`
	PlayUrl         string `json:"playUrl"`
	TenantName      string `json:"tenantName"`
}

type ActivityDetail struct {
	ActivityBrief
	Prizes       []PrizeView `json:"prizes"`
	ParticipantN int64       `json:"participantN"`
	WinN         int64       `json:"winN"`
}

type BlacklistReq struct {
	Account string `json:"account"`
	Reason  string `json:"reason,optional"`
}

type CreateActivityReq struct {
	Title           string       `json:"title"`
	Mode            string       `json:"mode"`
	Timezone        string       `json:"timezone,optional"`
	StartAt         int64        `json:"startAt"`
	EndAt           int64        `json:"endAt"`
	MaxDrawsPerUser int          `json:"maxDrawsPerUser"`
	MaxEnrollments  int          `json:"maxEnrollments,optional"`
	Prizes          []PrizeInput `json:"prizes"`
}

type DrawReq struct {
	PublicId       string `json:"publicId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type DrawResp struct {
	Result      string `json:"result"`
	PrizeId     string `json:"prizeId"`
	PrizeName   string `json:"prizeName"`
	Kind        string `json:"kind"`
	PrizeToken  string `json:"prizeToken"`
	RedeemCode  string `json:"redeemCode,optional"`
	RemainDraws int    `json:"remainDraws"`
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

type PrizeInput struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Stock    int    `json:"stock"`
	Weight   int    `json:"weight"`
	ImageUrl string `json:"imageUrl,optional"`
}

type PrizeView struct {
	Id       uint64 `json:"id,string"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Stock    int    `json:"stock"`
	Weight   int    `json:"weight"`
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

type WinnerItem struct {
	Nickname  string `json:"nickname"`
	PrizeName string `json:"prizeName"`
	Kind      string `json:"kind"`
	WonAt     int64  `json:"wonAt"`
}

type WinnersResp struct {
	List []WinnerItem `json:"list"`
}
