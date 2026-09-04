package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

type ScheduledWin struct {
	UserID  uint64
	PrizeID uint64
	Kind    string
	Token   string
	Rank    int
}

// AssignScheduled deterministically assigns limited real prizes to shuffled participants.
// Thank-you prizes are ignored. If players < stock, leftover prizes are unissued (not recycled).
func AssignScheduled(userIDs []uint64, prizes []PrizeSpec, seedHex string) ([]ScheduledWin, error) {
	seed, err := hex.DecodeString(seedHex)
	if err != nil || len(seed) < 16 {
		s, e := RandomSeed()
		if e != nil {
			return nil, e
		}
		seed, _ = hex.DecodeString(s)
		seedHex = s
	}
	ids := append([]uint64(nil), userIDs...)
	if err := shuffleUint64(ids, seed); err != nil {
		return nil, err
	}
	var wins []ScheduledWin
	rank := 1
	cursor := 0
	for _, p := range prizes {
		if p.Kind == "thank_you" {
			continue
		}
		for i := 0; i < p.Stock; i++ {
			if cursor >= len(ids) {
				return wins, nil
			}
			tok, err := tokenFromSeed(seed, rank)
			if err != nil {
				return nil, err
			}
			wins = append(wins, ScheduledWin{
				UserID:  ids[cursor],
				PrizeID: p.ID,
				Kind:    p.Kind,
				Token:   tok,
				Rank:    rank,
			})
			cursor++
			rank++
		}
	}
	return wins, nil
}

func shuffleUint64(ids []uint64, seed []byte) error {
	h := sha256.Sum256(seed)
	n := len(ids)
	for i := n - 1; i > 0; i-- {
		rnd := binary.BigEndian.Uint64(h[:8]) + uint64(i)*0x9e3779b97f4a7c15
		j := int(rnd % uint64(i+1))
		ids[i], ids[j] = ids[j], ids[i]
		h = sha256.Sum256(append(h[:], byte(i), byte(j)))
	}
	return nil
}

func tokenFromSeed(seed []byte, rank int) (string, error) {
	sum := sha256.Sum256(append(seed, byte(rank>>8), byte(rank)))
	return hex.EncodeToString(sum[:16]), nil
}
