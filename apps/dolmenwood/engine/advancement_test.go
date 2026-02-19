package engine

import "testing"

func TestAdvancementTableForClass(t *testing.T) {
	table, ok := AdvancementTableForClass("Knight")
	if !ok {
		t.Fatal("expected knight advancement table to be present")
	}
	if table.Title != "Knight Advancement" {
		t.Errorf("Title = %q, want %q", table.Title, "Knight Advancement")
	}
	if len(table.Headers) == 0 {
		t.Fatalf("expected headers to be present")
	}
	if table.Headers[0] != "Level" {
		t.Errorf("Headers[0] = %q, want %q", table.Headers[0], "Level")
	}
	if len(table.Rows) == 0 {
		t.Fatalf("expected rows to be present")
	}
	if table.Rows[0][0] != "1" || table.Rows[0][1] != "0" {
		t.Errorf("first row = %v, want level 1 starting XP", table.Rows[0])
	}
}

func TestAdvancementTableForClassUnknown(t *testing.T) {
	_, ok := AdvancementTableForClass("Unknown")
	if ok {
		t.Fatal("expected unknown class to return false")
	}
}
