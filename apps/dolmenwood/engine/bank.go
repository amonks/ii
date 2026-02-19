package engine

import (
	"fmt"
	"sort"
)

// CoinNotesCPValue returns the total value of a coin notes string in copper pieces.
// Exchange rates: 1cp=1, 1sp=10, 1gp=100.
func CoinNotesCPValue(notes string) int {
	coins := ParseCoinNotes(notes)
	return coins[CP] + coins[SP]*10 + coins[GP]*100
}

// CoinPurseCPValue returns the total CP value of a purse (gp/sp/cp only).
func CoinPurseCPValue(purse CoinPurse) int {
	return purse.CP + purse.SP*10 + purse.GP*100
}

// MinCoins converts a CP value into the fewest coins using gp/sp/cp only.
// Banks don't deal in PP or EP.
func MinCoins(cpValue int) CoinPurse {
	gp := cpValue / 100
	rem := cpValue % 100
	sp := rem / 10
	cp := rem % 10
	return CoinPurse{GP: gp, SP: sp, CP: cp}
}

// IsMature returns true if a deposit has been held for 30+ days.
func IsMature(depositDay, currentDay int) bool {
	return currentDay-depositDay >= 30
}

// BankLot represents a single deposit lot in the bank.
type BankLot struct {
	ID         uint
	CPValue    int
	DepositDay int
}

// WithdrawResult describes how to execute a withdrawal.
type WithdrawResult struct {
	ConsumedLots []uint       // lot IDs fully consumed (to delete)
	UpdatedLots  map[uint]int // lot ID -> new CP value (partially consumed)
	GrossCP      int          // total CP consumed from lots
	FeeCP        int          // CP lost to fees
	NetCP        int          // CP the player receives
}

// PlanWithdrawal plans how to withdraw requestedCP from the given lots.
// Mature lots (oldest first) are consumed free. Immature lots (newest first)
// incur a 10% fee: to net N cp, ceil(N*10/9) is consumed.
func PlanWithdrawal(lots []BankLot, requestedCP int, currentDay int) (*WithdrawResult, error) {
	if len(lots) == 0 {
		return nil, fmt.Errorf("no bank deposits")
	}

	// Separate into mature and immature
	var mature, immature []BankLot
	for _, lot := range lots {
		if IsMature(lot.DepositDay, currentDay) {
			mature = append(mature, lot)
		} else {
			immature = append(immature, lot)
		}
	}

	// Sort mature by deposit day ascending (oldest first)
	sort.Slice(mature, func(i, j int) bool {
		return mature[i].DepositDay < mature[j].DepositDay
	})
	// Sort immature by deposit day descending (newest first)
	sort.Slice(immature, func(i, j int) bool {
		return immature[i].DepositDay > immature[j].DepositDay
	})

	result := &WithdrawResult{
		UpdatedLots: make(map[uint]int),
	}
	remaining := requestedCP

	// Pull from mature lots first (free, 1:1)
	for _, lot := range mature {
		if remaining <= 0 {
			break
		}
		take := lot.CPValue
		if take > remaining {
			take = remaining
		}
		result.GrossCP += take
		remaining -= take
		if take == lot.CPValue {
			result.ConsumedLots = append(result.ConsumedLots, lot.ID)
		} else {
			result.UpdatedLots[lot.ID] = lot.CPValue - take
		}
	}

	// Pull from immature lots (newest first, with 10% fee)
	for _, lot := range immature {
		if remaining <= 0 {
			break
		}
		// To net N cp from immature, consume N + floor(N/9) gross.
		// This is the minimum G where G - floor(G/10) >= N.
		grossNeeded := remaining + remaining/9
		// Verify and bump if rounding leaves us short
		if grossNeeded-grossNeeded/10 < remaining {
			grossNeeded++
		}
		take := lot.CPValue
		if take > grossNeeded {
			take = grossNeeded
		}
		result.GrossCP += take
		// fee = floor(10% of gross taken); net = gross - fee
		fee := take / 10
		net := take - fee
		remaining -= net
		result.FeeCP += fee
		if take == lot.CPValue {
			result.ConsumedLots = append(result.ConsumedLots, lot.ID)
		} else {
			result.UpdatedLots[lot.ID] = lot.CPValue - take
		}
	}

	if remaining > 0 {
		return nil, fmt.Errorf("insufficient bank funds: need %d more cp", remaining)
	}

	result.NetCP = requestedCP
	return result, nil
}
