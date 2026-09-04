package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"luckygo/internal/xerr"
)

const WeightSum = 10000

type PrizeSpec struct {
	ID     uint64
	Name   string
	Kind   string
	Stock  int
	Weight int
}

type BucketItem struct {
	PrizeID uint64
	Kind    string
	Token   string
}

func (b BucketItem) Encode() string {
	return fmt.Sprintf("%d:%s:%s", b.PrizeID, b.Kind, b.Token)
}

func DecodeItem(s string) (BucketItem, error) {
	var zero BucketItem
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return zero, xerr.Internal()
	}
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return zero, xerr.Internal()
	}
	return BucketItem{PrizeID: id, Kind: parts[1], Token: parts[2]}, nil
}

func ValidatePrizes(prizes []PrizeSpec, mode string) error {
	if len(prizes) < 2 {
		return xerr.Bad("至少需要一个真实奖和一个谢谢参与")
	}
	sum := 0
	hasThank := false
	realStock := 0
	for _, p := range prizes {
		if p.Stock < 0 || p.Weight < 0 || p.Name == "" {
			return xerr.ErrInvalidParam
		}
		if p.Kind != "thank_you" && p.Kind != "virtual" && p.Kind != "physical" {
			return xerr.Bad("奖品类型不合法")
		}
		if p.Kind == "thank_you" {
			hasThank = true
		} else {
			realStock += p.Stock
		}
		if p.Stock == 0 {
			return xerr.Bad("奖品库存必须大于 0")
		}
		sum += p.Weight
	}
	if !hasThank {
		return xerr.Bad("必须配置「谢谢参与」")
	}
	if sum != WeightSum {
		return xerr.Bad("所有奖品权重之和必须为 10000（万分比）")
	}
	if mode == "scheduled" && realStock == 0 {
		return xerr.Bad("定时开奖至少需要一个有库存的真实奖品")
	}
	return nil
}

func BuildBucket(prizes []PrizeSpec) ([]string, error) {
	if err := ValidatePrizes(prizes, "instant"); err != nil {
		return nil, err
	}
	var items []string
	for _, p := range prizes {
		for i := 0; i < p.Stock; i++ {
			tok, err := randomToken()
			if err != nil {
				return nil, err
			}
			items = append(items, BucketItem{PrizeID: p.ID, Kind: p.Kind, Token: tok}.Encode())
		}
	}
	if err := cryptoShuffle(items); err != nil {
		return nil, err
	}
	return items, nil
}

func cryptoShuffle(items []string) error {
	n := len(items)
	for i := n - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return err
		}
		items[i], items[j] = items[j], items[i]
	}
	return nil
}

func randInt(n int) (int, error) {
	if n <= 0 {
		return 0, xerr.Internal()
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return 0, err
	}
	var v uint64
	for _, b := range buf {
		v = v<<8 | uint64(b)
	}
	return int(v % uint64(n)), nil
}

func randomToken() (string, error) {
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
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

func RandomSeed() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
