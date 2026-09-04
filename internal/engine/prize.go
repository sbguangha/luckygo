package engine

import (
	"crypto/rand"
	"encoding/hex"

	"luckygo/internal/xerr"
)

// PrizeSpec 奖项配置。live/scheduled 模式下奖项即名额：stock=总中奖人数，
// perRound=单次抽取个数，isAll=是否全员参与（true=未中过本奖即可再抽）。
type PrizeSpec struct {
	ID       uint64
	Name     string
	Kind     string
	Stock    int
	PerRound int
	IsAll    bool
}

// ValidatePrizes 校验奖项配置。live 与 scheduled 都是从名单里抽真实奖项，
// 不再有「谢谢参与」与权重概念。
func ValidatePrizes(prizes []PrizeSpec, mode string) error {
	if len(prizes) < 1 {
		return xerr.Bad("至少配置一个奖项")
	}
	for _, p := range prizes {
		if p.Name == "" || len(p.Name) > 64 {
			return xerr.Bad("奖项名称不合法")
		}
		if p.Kind != "virtual" && p.Kind != "physical" {
			return xerr.Bad("奖项类型仅支持 virtual（虚拟奖）/ physical（实物奖）")
		}
		if p.Stock < 1 || p.Stock > 100000 {
			return xerr.Bad("奖项人数须为 1-100000")
		}
		if mode == "live" {
			if p.PerRound < 1 || p.PerRound > 50 {
				return xerr.Bad("单次抽取个数须为 1-50")
			}
			if p.PerRound > p.Stock {
				return xerr.Bad("单次抽取个数不能超过奖项总人数")
			}
		}
	}
	return nil
}

func RandomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func RandomPublicID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func RandomRedeemCode() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func RandomSeed() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
