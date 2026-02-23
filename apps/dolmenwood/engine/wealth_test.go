package engine

import "testing"

func TestParseTransaction(t *testing.T) {
	cases := []struct {
		input       string
		wantAmount  int
		wantCoin    CoinType
		wantDesc    string
		wantErr     bool
	}{
		{"50g dragon hoard", 50, GP, "dragon hoard", false},
		{"100sp tavern", 100, SP, "tavern", false},
		{"50gp stuff", 50, GP, "stuff", false},
		{"3p gems", 3, PP, "gems", false},
		{"25c rations", 25, CP, "rations", false},
		{"25cp rations", 25, CP, "rations", false},
		{"10ep ring", 10, EP, "ring", false},
		{"10e ring", 10, EP, "ring", false},
		{"-50g bought rations", -50, GP, "bought rations", false},
		{"-10sp ale", -10, SP, "ale", false},
		{"200sp", 200, SP, "", false},
		{"50cp", 50, CP, "", false},
		{"100gp", 100, GP, "", false},
		{"bad input", 0, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			amount, coin, desc, err := ParseTransaction(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if amount != tc.wantAmount {
				t.Errorf("amount = %d, want %d", amount, tc.wantAmount)
			}
			if coin != tc.wantCoin {
				t.Errorf("coin = %q, want %q", coin, tc.wantCoin)
			}
			if desc != tc.wantDesc {
				t.Errorf("desc = %q, want %q", desc, tc.wantDesc)
			}
		})
	}
}

func TestCoinPurseGPValue(t *testing.T) {
	purse := CoinPurse{CP: 100, SP: 50, GP: 10}
	got := CoinPurseGPValue(purse)
	// 10 GP + 50 SP (=5 GP) + 100 CP (=1 GP) = 16 GP
	if got != 16 {
		t.Errorf("CoinPurseGPValue = %d, want 16", got)
	}
}

func TestTotalCoins(t *testing.T) {
	purse := CoinPurse{CP: 50, SP: 30, GP: 10}
	got := TotalCoins(purse)
	if got != 90 {
		t.Errorf("TotalCoins = %d, want 90", got)
	}
}

func TestAddToPurse(t *testing.T) {
	purse := CoinPurse{GP: 10}
	purse = AddToPurse(purse, 5, GP)
	if purse.GP != 15 {
		t.Errorf("GP = %d, want 15", purse.GP)
	}
	purse = AddToPurse(purse, 100, SP)
	if purse.SP != 100 {
		t.Errorf("SP = %d, want 100", purse.SP)
	}
}

func TestIsCoinItem(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		// Consolidated name
		{"Coins", true},
		{"coins", true},
		{"COINS", true},
		// Legacy per-denomination names (backward compat)
		{"Copper Pieces", true},
		{"Silver Pieces", true},
		{"Electrum Pieces", true},
		{"Gold Pieces", true},
		{"Platinum Pieces", true},
		{"copper pieces", true},
		{"GOLD PIECES", true},
		// Non-coin items
		{"Rope", false},
		{"Gold", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsCoinItem(tc.name); got != tc.want {
				t.Errorf("IsCoinItem(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestCoinItemName(t *testing.T) {
	cases := []struct {
		coinType CoinType
		want     string
	}{
		{CP, "Copper Pieces"},
		{SP, "Silver Pieces"},
		{EP, "Electrum Pieces"},
		{GP, "Gold Pieces"},
		{PP, "Platinum Pieces"},
	}
	for _, tc := range cases {
		t.Run(tc.coinType, func(t *testing.T) {
			if got := CoinItemName(tc.coinType); got != tc.want {
				t.Errorf("CoinItemName(%q) = %q, want %q", tc.coinType, got, tc.want)
			}
		})
	}
}

func TestParseCoinExpression(t *testing.T) {
	cases := []struct {
		input   string
		want    []CoinAmount
		wantErr bool
	}{
		{"100gp", []CoinAmount{{100, GP}}, false},
		{"100gp 2sp", []CoinAmount{{100, GP}, {2, SP}}, false},
		{"50g", []CoinAmount{{50, GP}}, false},
		{"50g 10s 5c", []CoinAmount{{50, GP}, {10, SP}, {5, CP}}, false},
		{"3pp", []CoinAmount{{3, PP}}, false},
		{"10ep", []CoinAmount{{10, EP}}, false},
		{"5", nil, true},       // ambiguous
		{"", nil, true},        // empty
		{"abc", nil, true},     // invalid
		{"100gp abc", nil, true}, // trailing junk
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseCoinExpression(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d amounts, want %d", len(got), len(tc.want))
			}
			for i, w := range tc.want {
				if got[i].Amount != w.Amount || got[i].CoinType != w.CoinType {
					t.Errorf("got[%d] = {%d, %q}, want {%d, %q}", i, got[i].Amount, got[i].CoinType, w.Amount, w.CoinType)
				}
			}
		})
	}
}

func TestCoinTypeFromItemName(t *testing.T) {
	cases := []struct {
		name     string
		wantType CoinType
		wantOK   bool
	}{
		{"Copper Pieces", CP, true},
		{"Silver Pieces", SP, true},
		{"Electrum Pieces", EP, true},
		{"Gold Pieces", GP, true},
		{"Platinum Pieces", PP, true},
		{"gold pieces", GP, true},
		{"PLATINUM PIECES", PP, true},
		{"Rope", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			coinType, ok := CoinTypeFromItemName(tc.name)
			if ok != tc.wantOK {
				t.Errorf("CoinTypeFromItemName(%q) ok = %v, want %v", tc.name, ok, tc.wantOK)
			}
			if ok && coinType != tc.wantType {
				t.Errorf("CoinTypeFromItemName(%q) = %q, want %q", tc.name, coinType, tc.wantType)
			}
		})
	}
}

func TestFormatCoinNotes(t *testing.T) {
	cases := []struct {
		name  string
		coins map[CoinType]int
		want  string
	}{
		{"all denominations", map[CoinType]int{PP: 5, GP: 50, EP: 10, SP: 20, CP: 100}, "5pp 50gp 10ep 20sp 100cp"},
		{"gp only", map[CoinType]int{GP: 50}, "50gp"},
		{"zeros omitted", map[CoinType]int{GP: 50, SP: 0, CP: 10}, "50gp 10cp"},
		{"empty map", map[CoinType]int{}, ""},
		{"nil map", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatCoinNotes(tc.coins)
			if got != tc.want {
				t.Errorf("FormatCoinNotes(%v) = %q, want %q", tc.coins, got, tc.want)
			}
		})
	}
}

func TestParseCoinNotes(t *testing.T) {
	cases := []struct {
		name    string
		notes   string
		want    map[CoinType]int
	}{
		{"full notes", "5pp 50gp 10ep 20sp 100cp", map[CoinType]int{PP: 5, GP: 50, EP: 10, SP: 20, CP: 100}},
		{"gp only", "50gp", map[CoinType]int{GP: 50}},
		{"empty string", "", map[CoinType]int{}},
		{"shorthand", "50g 10s", map[CoinType]int{GP: 50, SP: 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCoinNotes(tc.notes)
			for ct, want := range tc.want {
				if got[ct] != want {
					t.Errorf("ParseCoinNotes(%q)[%s] = %d, want %d", tc.notes, ct, got[ct], want)
				}
			}
			for ct, v := range got {
				if tc.want[ct] != v {
					t.Errorf("ParseCoinNotes(%q) has unexpected %s=%d", tc.notes, ct, v)
				}
			}
		})
	}
}

func TestFormatParseCoinNotesRoundTrip(t *testing.T) {
	coins := map[CoinType]int{PP: 3, GP: 100, SP: 50, CP: 25}
	notes := FormatCoinNotes(coins)
	parsed := ParseCoinNotes(notes)
	for ct, want := range coins {
		if parsed[ct] != want {
			t.Errorf("round-trip %s: got %d, want %d", ct, parsed[ct], want)
		}
	}
}

func TestMergeCoinNotes(t *testing.T) {
	cases := []struct {
		name          string
		existing      string
		add           []CoinAmount
		wantNotes     string
		wantTotal     int
	}{
		{"add to empty", "", []CoinAmount{{50, GP}}, "50gp", 50},
		{"add same denom", "50gp", []CoinAmount{{25, GP}}, "75gp", 75},
		{"add different denom", "50gp", []CoinAmount{{20, SP}}, "50gp 20sp", 70},
		{"add multiple", "10gp 5sp", []CoinAmount{{5, GP}, {10, SP}, {100, CP}}, "15gp 15sp 100cp", 130},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notes, total := MergeCoinNotes(tc.existing, tc.add)
			if notes != tc.wantNotes {
				t.Errorf("MergeCoinNotes notes = %q, want %q", notes, tc.wantNotes)
			}
			if total != tc.wantTotal {
				t.Errorf("MergeCoinNotes total = %d, want %d", total, tc.wantTotal)
			}
		})
	}
}

func TestSubtractCoinNotes(t *testing.T) {
	cases := []struct {
		name      string
		existing  string
		sub       []CoinAmount
		wantNotes string
		wantTotal int
		wantErr   bool
	}{
		{"subtract partial", "100gp 50sp", []CoinAmount{{30, GP}}, "70gp 50sp", 120, false},
		{"subtract exact", "50gp", []CoinAmount{{50, GP}}, "", 0, false},
		{"subtract multiple", "100gp 50sp", []CoinAmount{{30, GP}, {20, SP}}, "70gp 30sp", 100, false},
		{"insufficient", "10gp", []CoinAmount{{20, GP}}, "", 0, true},
		{"insufficient one denom", "100gp 5sp", []CoinAmount{{10, SP}}, "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notes, total, err := SubtractCoinNotes(tc.existing, tc.sub)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if notes != tc.wantNotes {
				t.Errorf("SubtractCoinNotes notes = %q, want %q", notes, tc.wantNotes)
			}
			if total != tc.wantTotal {
				t.Errorf("SubtractCoinNotes total = %d, want %d", total, tc.wantTotal)
			}
		})
	}
}

func TestCoinNotesGPValue(t *testing.T) {
	cases := []struct {
		name  string
		notes string
		want  int
	}{
		{"gp only", "100gp", 100},
		{"mixed", "10gp 100sp 200cp", 22}, // 10 + 10 + 2
		{"empty", "", 0},
		{"pp", "2pp 5gp", 15}, // 10 + 5
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CoinNotesGPValue(tc.notes)
			if got != tc.want {
				t.Errorf("CoinNotesGPValue(%q) = %d, want %d", tc.notes, got, tc.want)
			}
		})
	}
}
