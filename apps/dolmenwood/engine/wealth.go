package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type CoinType = string

const (
	CP CoinType = "cp"
	SP CoinType = "sp"
	EP CoinType = "ep"
	GP CoinType = "gp"
	PP CoinType = "pp"
)

type CoinPurse struct {
	CP int
	SP int
	EP int
	GP int
	PP int
}

var txPattern = regexp.MustCompile(`^(\d+)\s*(cp|c|sp|s|ep|e|gp|g|pp|p)\s+(.+)$`)

// ParseTransaction parses a shorthand transaction string like "50g dragon hoard"
// into amount, coin type, and description.
func ParseTransaction(input string) (int, CoinType, string, error) {
	input = strings.TrimSpace(input)
	m := txPattern.FindStringSubmatch(strings.ToLower(input))
	if m == nil {
		return 0, "", "", fmt.Errorf("invalid transaction format: %q", input)
	}
	amount, _ := strconv.Atoi(m[1])
	coin := normalizeCoin(m[2])
	desc := strings.TrimSpace(m[3])
	return amount, coin, desc, nil
}

func normalizeCoin(s string) CoinType {
	switch s {
	case "c", "cp":
		return CP
	case "s", "sp":
		return SP
	case "e", "ep":
		return EP
	case "g", "gp":
		return GP
	case "p", "pp":
		return PP
	}
	return GP
}

// CoinPurseGPValue returns the total value of a purse in gold pieces.
// 100CP=1GP, 10SP=1GP, 2EP=1GP, 1GP=1GP, 1PP=5GP
func CoinPurseGPValue(purse CoinPurse) int {
	return purse.GP + purse.SP/10 + purse.CP/100 + purse.EP/2 + purse.PP*5
}

// TotalCoins returns the total number of individual coins in a purse.
func TotalCoins(purse CoinPurse) int {
	return purse.CP + purse.SP + purse.EP + purse.GP + purse.PP
}

// AddToPurse returns a new purse with the given amount of coins added.
func AddToPurse(purse CoinPurse, amount int, coin CoinType) CoinPurse {
	switch coin {
	case CP:
		purse.CP += amount
	case SP:
		purse.SP += amount
	case EP:
		purse.EP += amount
	case GP:
		purse.GP += amount
	case PP:
		purse.PP += amount
	}
	return purse
}
