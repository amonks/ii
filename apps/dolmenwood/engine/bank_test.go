package engine

import (
	"testing"
)

func TestCoinNotesCPValue(t *testing.T) {
	tests := []struct {
		notes string
		want  int
	}{
		{"", 0},
		{"100cp", 100},
		{"10sp", 100},
		{"1gp", 100},
		{"1gp 5sp 10cp", 160},
		{"2gp 3sp", 230},
	}
	for _, tt := range tests {
		t.Run(tt.notes, func(t *testing.T) {
			got := CoinNotesCPValue(tt.notes)
			if got != tt.want {
				t.Errorf("CoinNotesCPValue(%q) = %d, want %d", tt.notes, got, tt.want)
			}
		})
	}
}

func TestMinCoins(t *testing.T) {
	tests := []struct {
		cp     int
		wantGP int
		wantSP int
		wantCP int
	}{
		{0, 0, 0, 0},
		{1, 0, 0, 1},
		{10, 0, 1, 0},
		{15, 0, 1, 5},
		{100, 1, 0, 0},
		{123, 1, 2, 3},
		{90, 0, 9, 0},
		{180, 1, 8, 0},
		{250, 2, 5, 0},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := MinCoins(tt.cp)
			if got.GP != tt.wantGP || got.SP != tt.wantSP || got.CP != tt.wantCP {
				t.Errorf("MinCoins(%d) = {GP:%d SP:%d CP:%d}, want {GP:%d SP:%d CP:%d}",
					tt.cp, got.GP, got.SP, got.CP, tt.wantGP, tt.wantSP, tt.wantCP)
			}
			if got.PP != 0 || got.EP != 0 {
				t.Errorf("MinCoins(%d) should have PP=0 EP=0, got PP=%d EP=%d", tt.cp, got.PP, got.EP)
			}
		})
	}
}

func TestIsMature(t *testing.T) {
	tests := []struct {
		depositDay int
		currentDay int
		want       bool
	}{
		{1, 1, false},
		{1, 30, false},
		{1, 31, true},
		{1, 100, true},
		{10, 39, false},
		{10, 40, true},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := IsMature(tt.depositDay, tt.currentDay)
			if got != tt.want {
				t.Errorf("IsMature(%d, %d) = %v, want %v", tt.depositDay, tt.currentDay, got, tt.want)
			}
		})
	}
}

func TestPlanWithdrawal_AllMature(t *testing.T) {
	lots := []BankLot{
		{ID: 1, CPValue: 200, DepositDay: 1},
	}
	result, err := PlanWithdrawal(lots, 100, 50)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	if result.GrossCP != 100 {
		t.Errorf("GrossCP = %d, want 100", result.GrossCP)
	}
	if result.FeeCP != 0 {
		t.Errorf("FeeCP = %d, want 0", result.FeeCP)
	}
	if result.NetCP != 100 {
		t.Errorf("NetCP = %d, want 100", result.NetCP)
	}
	// Lot 1 partially consumed: 200 -> 100
	if result.UpdatedLots[1] != 100 {
		t.Errorf("UpdatedLot[1] = %d, want 100", result.UpdatedLots[1])
	}
	if len(result.ConsumedLots) != 0 {
		t.Errorf("ConsumedLots = %v, want empty", result.ConsumedLots)
	}
}

func TestPlanWithdrawal_AllMatureFullConsume(t *testing.T) {
	lots := []BankLot{
		{ID: 1, CPValue: 100, DepositDay: 1},
	}
	result, err := PlanWithdrawal(lots, 100, 50)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	if result.GrossCP != 100 {
		t.Errorf("GrossCP = %d, want 100", result.GrossCP)
	}
	if result.FeeCP != 0 {
		t.Errorf("FeeCP = %d, want 0", result.FeeCP)
	}
	if len(result.ConsumedLots) != 1 || result.ConsumedLots[0] != 1 {
		t.Errorf("ConsumedLots = %v, want [1]", result.ConsumedLots)
	}
}

func TestPlanWithdrawal_AllImmature(t *testing.T) {
	// Deposit 100cp, withdraw 90cp next day.
	// Need 90 net. Gross = ceil(90 * 10 / 9) = 100. Fee = 100 - 90 = 10.
	lots := []BankLot{
		{ID: 1, CPValue: 100, DepositDay: 1},
	}
	result, err := PlanWithdrawal(lots, 90, 2)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	if result.GrossCP != 100 {
		t.Errorf("GrossCP = %d, want 100", result.GrossCP)
	}
	if result.FeeCP != 10 {
		t.Errorf("FeeCP = %d, want 10", result.FeeCP)
	}
	if result.NetCP != 90 {
		t.Errorf("NetCP = %d, want 90", result.NetCP)
	}
	if len(result.ConsumedLots) != 1 || result.ConsumedLots[0] != 1 {
		t.Errorf("ConsumedLots = %v, want [1]", result.ConsumedLots)
	}
}

func TestPlanWithdrawal_MixedMatureImmature(t *testing.T) {
	// Mature lot: 50cp (free). Immature lot: 100cp (with fee).
	// Request 90cp. First pull 50 from mature (free), then need 40 more net.
	// From immature: gross = ceil(40 * 10 / 9) = 45. Fee = 45 - 40 = 5.
	lots := []BankLot{
		{ID: 1, CPValue: 50, DepositDay: 1},
		{ID: 2, CPValue: 100, DepositDay: 25},
	}
	result, err := PlanWithdrawal(lots, 90, 31)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	if result.GrossCP != 94 {
		t.Errorf("GrossCP = %d, want 94 (50 mature + 44 immature)", result.GrossCP)
	}
	if result.FeeCP != 4 {
		t.Errorf("FeeCP = %d, want 4", result.FeeCP)
	}
	if result.NetCP != 90 {
		t.Errorf("NetCP = %d, want 90", result.NetCP)
	}
}

func TestPlanWithdrawal_InsufficientFunds(t *testing.T) {
	lots := []BankLot{
		{ID: 1, CPValue: 50, DepositDay: 1},
	}
	_, err := PlanWithdrawal(lots, 100, 2)
	if err == nil {
		t.Error("expected error for insufficient funds")
	}
}

func TestPlanWithdrawal_EmptyLots(t *testing.T) {
	_, err := PlanWithdrawal(nil, 100, 1)
	if err == nil {
		t.Error("expected error for empty lots")
	}
}

func TestPlanWithdrawal_MatureConsumedBeforeImmature(t *testing.T) {
	// Two mature lots and one immature.
	// Should consume mature lots (oldest first) before touching immature.
	lots := []BankLot{
		{ID: 1, CPValue: 30, DepositDay: 1},  // mature (oldest)
		{ID: 2, CPValue: 30, DepositDay: 5},  // mature
		{ID: 3, CPValue: 100, DepositDay: 40}, // immature
	}
	result, err := PlanWithdrawal(lots, 60, 50)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	// Should take 30 from lot 1 + 30 from lot 2 = 60 mature, no fee
	if result.FeeCP != 0 {
		t.Errorf("FeeCP = %d, want 0 (all from mature lots)", result.FeeCP)
	}
	if result.GrossCP != 60 {
		t.Errorf("GrossCP = %d, want 60", result.GrossCP)
	}
	if len(result.ConsumedLots) != 2 {
		t.Errorf("ConsumedLots = %v, want [1, 2]", result.ConsumedLots)
	}
}

func TestPlanWithdrawal_ImmatureNewestFirst(t *testing.T) {
	// Two immature lots. Should pull from newest first.
	lots := []BankLot{
		{ID: 1, CPValue: 100, DepositDay: 25},
		{ID: 2, CPValue: 100, DepositDay: 28},
	}
	// Request 9cp. Gross from immature = ceil(9*10/9) = 10. Fee = 1.
	result, err := PlanWithdrawal(lots, 9, 30)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	if result.GrossCP != 10 {
		t.Errorf("GrossCP = %d, want 10", result.GrossCP)
	}
	if result.FeeCP != 1 {
		t.Errorf("FeeCP = %d, want 1", result.FeeCP)
	}
	// Should pull from lot 2 (newest) first
	if result.UpdatedLots[2] != 90 {
		t.Errorf("UpdatedLot[2] = %d, want 90", result.UpdatedLots[2])
	}
	if _, ok := result.UpdatedLots[1]; ok {
		t.Errorf("Lot 1 should not be touched")
	}
}

func TestPlanWithdrawal_TwoImmatureDeposits(t *testing.T) {
	// Deposit 100cp twice, withdraw 180cp.
	// All immature. Need gross = ceil(180 * 10 / 9) = 200. Fee = 20.
	lots := []BankLot{
		{ID: 1, CPValue: 100, DepositDay: 1},
		{ID: 2, CPValue: 100, DepositDay: 1},
	}
	result, err := PlanWithdrawal(lots, 180, 2)
	if err != nil {
		t.Fatalf("PlanWithdrawal: %v", err)
	}
	if result.GrossCP != 200 {
		t.Errorf("GrossCP = %d, want 200", result.GrossCP)
	}
	if result.FeeCP != 20 {
		t.Errorf("FeeCP = %d, want 20", result.FeeCP)
	}
	if result.NetCP != 180 {
		t.Errorf("NetCP = %d, want 180", result.NetCP)
	}
}
