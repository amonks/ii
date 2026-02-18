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

// CoinItemNameStr is the consolidated coin item name.
const CoinItemNameStr = "Coins"

// coinDenomOrder is the canonical display order for coin denominations.
var coinDenomOrder = []CoinType{PP, GP, EP, SP, CP}

type CoinPurse struct {
	CP int
	SP int
	EP int
	GP int
	PP int
}

var txPattern = regexp.MustCompile(`^(-?\d+)\s*(cp|c|sp|s|ep|e|gp|g|pp|p)\s+(.+)$`)

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

// IsCoinItem returns true if the item name represents coins.
// Matches both consolidated "Coins" and legacy per-denomination names.
func IsCoinItem(name string) bool {
	lower := strings.ToLower(name)
	if lower == "coins" {
		return true
	}
	_, ok := coinItemNames[lower]
	return ok
}

// CoinItemName returns the inventory item name for a coin type.
func CoinItemName(coinType CoinType) string {
	switch coinType {
	case CP:
		return "Copper Pieces"
	case SP:
		return "Silver Pieces"
	case EP:
		return "Electrum Pieces"
	case GP:
		return "Gold Pieces"
	case PP:
		return "Platinum Pieces"
	}
	return "Gold Pieces"
}

// CoinTypeFromItemName returns the coin type for a coin item name.
func CoinTypeFromItemName(name string) (CoinType, bool) {
	ct, ok := coinItemNames[strings.ToLower(name)]
	return ct, ok
}

var coinItemNames = map[string]CoinType{
	"copper pieces":   CP,
	"silver pieces":   SP,
	"electrum pieces": EP,
	"gold pieces":     GP,
	"platinum pieces": PP,
}

// CoinAmount represents a quantity of a specific coin denomination.
type CoinAmount struct {
	Amount   int
	CoinType CoinType
}

var coinExprPattern = regexp.MustCompile(`(\d+)\s*(cp|c|sp|s|ep|e|gp|g|pp|p)`)

// ParseCoinExpression parses a free-text coin expression like "100gp 2sp"
// into a list of (amount, coinType) pairs.
func ParseCoinExpression(input string) ([]CoinAmount, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty coin expression")
	}

	matches := coinExprPattern.FindAllStringSubmatch(strings.ToLower(input), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid coin expression: %q", input)
	}

	// Verify the entire input is consumed by the matches (no trailing junk)
	rebuilt := coinExprPattern.ReplaceAllString(strings.ToLower(input), "")
	rebuilt = strings.TrimSpace(rebuilt)
	if rebuilt != "" {
		return nil, fmt.Errorf("invalid coin expression: unexpected %q", rebuilt)
	}

	var result []CoinAmount
	for _, m := range matches {
		amount, _ := strconv.Atoi(m[1])
		coin := normalizeCoin(m[2])
		result = append(result, CoinAmount{Amount: amount, CoinType: coin})
	}
	return result, nil
}

// FormatCoinNotes formats a denomination map into a notes string like "50gp 20sp 10cp".
// Ordered pp/gp/ep/sp/cp, zeros omitted.
func FormatCoinNotes(coins map[CoinType]int) string {
	var parts []string
	for _, ct := range coinDenomOrder {
		if qty, ok := coins[ct]; ok && qty > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", qty, ct))
		}
	}
	return strings.Join(parts, " ")
}

// ParseCoinNotes parses a notes string like "50gp 20sp" into a denomination map.
func ParseCoinNotes(notes string) map[CoinType]int {
	result := make(map[CoinType]int)
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return result
	}
	amounts, err := ParseCoinExpression(notes)
	if err != nil {
		return result
	}
	for _, a := range amounts {
		result[a.CoinType] += a.Amount
	}
	return result
}

// MergeCoinNotes merges additional coin amounts into an existing notes string.
// Returns the new notes string and total coin count.
func MergeCoinNotes(existingNotes string, add []CoinAmount) (string, int) {
	coins := ParseCoinNotes(existingNotes)
	for _, a := range add {
		coins[a.CoinType] += a.Amount
	}
	notes := FormatCoinNotes(coins)
	total := 0
	for _, qty := range coins {
		total += qty
	}
	return notes, total
}

// SubtractCoinNotes subtracts coin amounts from an existing notes string.
// Returns the new notes string, total coin count, and error if insufficient.
func SubtractCoinNotes(existingNotes string, sub []CoinAmount) (string, int, error) {
	coins := ParseCoinNotes(existingNotes)
	for _, a := range sub {
		if coins[a.CoinType] < a.Amount {
			return "", 0, fmt.Errorf("insufficient %s: have %d, want %d", a.CoinType, coins[a.CoinType], a.Amount)
		}
		coins[a.CoinType] -= a.Amount
	}
	notes := FormatCoinNotes(coins)
	total := 0
	for _, qty := range coins {
		total += qty
	}
	return notes, total, nil
}

// CoinNotesGPValue computes the GP value from a notes string.
func CoinNotesGPValue(notes string) int {
	coins := ParseCoinNotes(notes)
	purse := CoinPurse{
		CP: coins[CP],
		SP: coins[SP],
		EP: coins[EP],
		GP: coins[GP],
		PP: coins[PP],
	}
	return CoinPurseGPValue(purse)
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
