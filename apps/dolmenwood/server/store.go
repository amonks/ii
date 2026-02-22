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
	purchasedQty := 1
	if bundle := engine.ItemBundleSize(name); bundle > 0 {
		purchasedQty = bundle
	}
	if costCP == 0 {
		if err := s.buyFreeStoreItem(ch, name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.addAuditLog(ch, "store_buy", fmt.Sprintf("bought %s, qty %d for %s", name, purchasedQty, itemCostLabel(costCP, "")))
		s.renderInventory(w, r, ch)
		return
	}
	if costCP < 0 {
		http.Error(w, "Invalid store price", http.StatusBadRequest)
		return
	}

	// Companion breeds: deduct coins, create companion instead of item
	if engine.IsCompanionBreed(name) {
		if err := s.buyStoreCompanion(ch, name, costCP); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.addAuditLog(ch, "store_buy", fmt.Sprintf("bought %s for %s", name, itemCostLabel(costCP, "")))
		s.renderInventoryAndCompanions(w, r, ch)
		return
	}

	if err := s.buyStoreItem(ch, name, costCP); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.addAuditLog(ch, "store_buy", fmt.Sprintf("bought %s, qty %d for %s", name, purchasedQty, itemCostLabel(costCP, "")))
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

func (s *Server) buyStoreCompanion(ch *db.Character, breed string, costCP int) error {
	if err := s.deductCoins(ch, costCP); err != nil {
		return err
	}
	stats, ok := engine.BreedStats(breed)
	if !ok {
		return fmt.Errorf("unknown breed: %s", breed)
	}
	comp := &db.Companion{
		CharacterID: ch.ID,
		Name:        breed,
		Breed:       breed,
		HPMax:       stats.HPMax,
		HPCurrent:   stats.HPMax,
	}
	if err := s.db.CreateCompanion(comp); err != nil {
		return err
	}
	s.addAuditLog(ch, "companion_add", fmt.Sprintf("created %s the %s, HP %d/%d", comp.Name, comp.Breed, comp.HPCurrent, comp.HPMax))
	return nil
}

// deductCoins deducts costCP from the character's spendable coins.
func (s *Server) deductCoins(ch *db.Character, costCP int) error {
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

// storeSellPriceCP returns the sell price (half of buy price) for a named item.
func storeSellPriceCP(name string) (int, bool) {
	item, ok := storeCatalogItem(name)
	if !ok || item.CostCP == 0 {
		return 0, false
	}
	return item.CostCP / 2, true
}

func (s *Server) handleSellItem(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	itemID := atoui(r.PathValue("itemID"))
	item, err := s.db.GetItem(itemID)
	if err != nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	sellCP, ok := storeSellPriceCP(item.Name)
	if !ok {
		http.Error(w, "Item cannot be sold", http.StatusBadRequest)
		return
	}

	// Delete the item
	if err := s.db.DeleteItem(item.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add coins
	coins := minCoinCounts(sellCP)
	coinMap := make(map[engine.CoinType]int)
	totalCoins := 0
	if coins.PP > 0 {
		coinMap[engine.PP] = coins.PP
		totalCoins += coins.PP
	}
	if coins.GP > 0 {
		coinMap[engine.GP] = coins.GP
		totalCoins += coins.GP
	}
	if coins.EP > 0 {
		coinMap[engine.EP] = coins.EP
		totalCoins += coins.EP
	}
	if coins.SP > 0 {
		coinMap[engine.SP] = coins.SP
		totalCoins += coins.SP
	}
	if coins.CP > 0 {
		coinMap[engine.CP] = coins.CP
		totalCoins += coins.CP
	}
	if totalCoins > 0 {
		coinItem := &db.Item{
			CharacterID: ch.ID,
			Name:        engine.CoinItemNameStr,
			Quantity:    totalCoins,
			Notes:       engine.FormatCoinNotes(coinMap),
		}
		if err := s.combineStackableItems(ch.ID, coinItem, ch.CurrentDay); err != nil {
			if !errors.Is(err, errNotCombined) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.db.CreateItem(coinItem); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	s.addAuditLog(ch, "store_sell", fmt.Sprintf("sold %s for %s", item.Name, itemCostLabel(sellCP, "")))
	s.renderInventory(w, r, ch)
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
