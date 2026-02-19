package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/engine"
)

const storeItemSeparator = "|"

func (s *Server) handleStoreBuy(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	itemID := r.FormValue("item_id")
	name, costCP, err := parseStoreItemID(itemID)
	if err != nil {
		http.Error(w, "Invalid store item", http.StatusBadRequest)
		return
	}
	if costCP == 0 {
		if err := s.buyFreeStoreItem(ch, name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.addAuditLog(ch, "store_buy", fmt.Sprintf("Bought %s for %s", name, itemCostLabel(costCP, "")))
		s.renderInventory(w, r, ch)
		return
	}
	if costCP < 0 {
		http.Error(w, "Invalid store price", http.StatusBadRequest)
		return
	}
	if err := s.buyStoreItem(ch, name, costCP); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.addAuditLog(ch, "store_buy", fmt.Sprintf("Bought %s for %s", name, itemCostLabel(costCP, "")))
	s.renderInventory(w, r, ch)
}

func parseStoreItemID(itemID string) (string, int, error) {
	parts := strings.Split(itemID, storeItemSeparator)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid store item id")
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", 0, fmt.Errorf("invalid store item name")
	}
	costCP, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid store item cost")
	}
	if costCP < 0 {
		return "", 0, fmt.Errorf("invalid store item cost")
	}

	item, ok := storeCatalogItem(name)
	if !ok {
		return "", 0, fmt.Errorf("unknown store item")
	}
	if item.CostCP != costCP {
		return "", 0, fmt.Errorf("invalid store item cost")
	}

	return item.Name, item.CostCP, nil
}

func storeCatalogItem(name string) (StoreItem, bool) {
	for _, group := range buildStoreGroups() {
		for _, item := range group.Items {
			if strings.EqualFold(item.Name, name) {
				return item, true
			}
		}
	}
	return StoreItem{}, false
}

func (s *Server) buyFreeStoreItem(ch *db.Character, name string) error {
	if name == "" {
		return fmt.Errorf("invalid store item")
	}
	purchasedQty := 1
	if bundle := engine.ItemBundleSize(name); bundle > 0 {
		purchasedQty = bundle
	}
	purchased := &db.Item{CharacterID: ch.ID, Name: name, Quantity: purchasedQty, Location: "stowed"}
	if err := s.combineStackableItems(ch.ID, purchased, ch.CurrentDay); err != nil {
		if !errors.Is(err, errNotCombined) {
			return err
		}
		if err := s.db.CreateItem(purchased); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) buyStoreItem(ch *db.Character, name string, costCP int) error {
	items, err := s.db.ListItems(ch.ID)
	if err != nil {
		return err
	}
	coinItem, ok := findSpendableCoins(items)
	if !ok {
		return fmt.Errorf("no coins available")
	}
	coinNotes := engine.ParseCoinNotes(coinItem.Notes)
	inventoryCP := coinPurseCPValue(coinNotes)
	foundCP := foundTreasureCP(ch)
	purseCP := inventoryCP - foundCP
	if purseCP < costCP {
		return fmt.Errorf("insufficient coins")
	}
	remainingInventoryCP := inventoryCP - costCP
	remainingCoins := minCoinCounts(remainingInventoryCP)
	newNotes := engine.FormatCoinNotes(map[engine.CoinType]int{
		engine.PP: remainingCoins.PP,
		engine.GP: remainingCoins.GP,
		engine.EP: remainingCoins.EP,
		engine.SP: remainingCoins.SP,
		engine.CP: remainingCoins.CP,
	})
	newTotal := remainingCoins.PP + remainingCoins.GP + remainingCoins.EP + remainingCoins.SP + remainingCoins.CP
	coinItem.Notes = newNotes
	coinItem.Quantity = newTotal
	if newTotal == 0 {
		if err := s.db.DeleteItem(coinItem.ID); err != nil {
			return err
		}
	} else if err := s.db.UpdateItem(&coinItem); err != nil {
		return err
	}
	purchasedQty := 1
	if bundle := engine.ItemBundleSize(name); bundle > 0 {
		purchasedQty = bundle
	}
	purchased := &db.Item{CharacterID: ch.ID, Name: name, Quantity: purchasedQty, Location: "stowed"}
	if err := s.combineStackableItems(ch.ID, purchased, ch.CurrentDay); err != nil {
		if !errors.Is(err, errNotCombined) {
			return err
		}
		if err := s.db.CreateItem(purchased); err != nil {
			return err
		}
	}
	return nil
}

func foundTreasureCP(ch *db.Character) int {
	return ch.FoundCP + ch.FoundSP*10 + ch.FoundEP*50 + ch.FoundGP*100 + ch.FoundPP*500
}

func coinPurseCPValue(coins map[engine.CoinType]int) int {
	cp := coins[engine.CP]
	sp := coins[engine.SP]
	ep := coins[engine.EP]
	gp := coins[engine.GP]
	pp := coins[engine.PP]
	return cp + sp*10 + ep*50 + gp*100 + pp*500
}

type coinCounts struct {
	PP int
	GP int
	EP int
	SP int
	CP int
}

func minCoinCounts(cpValue int) coinCounts {
	pp := cpValue / 500
	rem := cpValue % 500
	gp := rem / 100
	rem = rem % 100
	ep := rem / 50
	rem = rem % 50
	sp := rem / 10
	cp := rem % 10
	return coinCounts{PP: pp, GP: gp, EP: ep, SP: sp, CP: cp}
}

func findSpendableCoins(items []db.Item) (db.Item, bool) {
	for _, item := range items {
		if !strings.EqualFold(item.Name, engine.CoinItemNameStr) {
			continue
		}
		if item.ContainerID != nil || item.CompanionID != nil {
			continue
		}
		return item, true
	}
	for _, item := range items {
		if !strings.EqualFold(item.Name, engine.CoinItemNameStr) {
			continue
		}
		return item, true
	}
	return db.Item{}, false
}
