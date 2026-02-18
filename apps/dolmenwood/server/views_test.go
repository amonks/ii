package server

import (
	"testing"

	"monks.co/apps/dolmenwood/db"
)

func TestItemIsTiny(t *testing.T) {
	tests := []struct {
		name string
		item InventoryItem
		want bool
	}{
		{
			name: "explicit tiny flag",
			item: InventoryItem{Item: db.Item{Name: "Clothes", IsTiny: true}},
			want: true,
		},
		{
			name: "built-in tiny item",
			item: InventoryItem{Item: db.Item{Name: "Bell"}},
			want: true,
		},
		{
			name: "clothing is not tiny",
			item: InventoryItem{Item: db.Item{Name: "Clothes"}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := itemIsTiny(tt.item); got != tt.want {
				t.Errorf("itemIsTiny(%q) = %t, want %t", tt.item.Name, got, tt.want)
			}
		})
	}
}
