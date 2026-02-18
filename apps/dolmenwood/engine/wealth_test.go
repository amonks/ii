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
