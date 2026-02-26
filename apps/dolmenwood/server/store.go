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
		oldWealth, newWealth, err := s.buyStoreCompanion(ch, name, costCP)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.addAuditLog(ch, "store_buy", fmt.Sprintf("bought %s for %s, wealth %s -> %s", name, itemCostLabel(costCP, ""), oldWealth, newWealth))
		s.renderInventory(w, r, ch)
		return
	}

	oldWealth, newWealth, err := s.buyStoreItem(ch, name, costCP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.addAuditLog(ch, "store_buy", fmt.Sprintf("bought %s, qty %d for %s, wealth %s -> %s", name, purchasedQty, itemCostLabel(costCP, ""), oldWealth, newWealth))
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

func (s *Server) buyStoreCompanion(ch *db.Character, breed string, costCP int) (string, string, error) {
	oldWealth, newWealth, err := s.deductCoins(ch, costCP)
	if err != nil {
		return "", "", err
	}
	stats, ok := engine.BreedStats(breed)
	if !ok {
		return "", "", fmt.Errorf("unknown breed: %s", breed)
	}
	comp := &db.Companion{
		CharacterID: ch.ID,
		Name:        breed,
		Breed:       breed,
		HPMax:       stats.HPMax,
		HPCurrent:   stats.HPMax,
	}
	if engine.IsRetainer(breed) {
		comp.Loyalty = engine.RetainerLoyalty(engine.Modifier(ch.CHA))
	}
	if err := s.db.CreateCompanion(comp); err != nil {
		return "", "", err
	}
	s.addAuditLog(ch, "companion_add", fmt.Sprintf("created %s the %s, HP %d/%d", comp.Name, comp.Breed, comp.HPCurrent, comp.HPMax))
	return oldWealth, newWealth, nil
}

// deductCoins deducts costCP from the character's spendable coins.
// Returns the old and new wealth labels (e.g. "50gp", "25gp").
func (s *Server) deductCoins(ch *db.Character, costCP int) (string, string, error) {
	items, err := s.db.ListItems(ch.ID)
	if err != nil {
		return "", "", err
	}
	coinItem, ok := findSpendableCoins(items)
	if !ok {
		return "", "", fmt.Errorf("no coins available")
	}
	coinNotes := engine.ParseCoinNotes(coinItem.Notes)
	existingPP := coinNotes[engine.PP]
	inventoryCP := coinPurseCPValue(coinNotes)
	oldWealth := cpAsCoinLabel(inventoryCP)
	foundCP := foundTreasureCP(ch)
	purseCP := inventoryCP - foundCP
	if purseCP < costCP {
		return "", "", fmt.Errorf("insufficient coins")
	}
	remainingInventoryCP := inventoryCP - costCP

	// Preserve existing PP: do changemaking only on non-PP coins.
	// If the cost exceeds non-PP coins, break PP as needed.
	ppCP := existingPP * 500
	nonPPCP := remainingInventoryCP - ppCP
	if nonPPCP < 0 {
		ppToBreak := (-nonPPCP + 499) / 500
		existingPP -= ppToBreak
		nonPPCP += ppToBreak * 500
	}

	remainingCoins := minCoinCounts(nonPPCP)
	newNotes := engine.FormatCoinNotes(map[engine.CoinType]int{
		engine.PP: existingPP,
		engine.GP: remainingCoins.GP,
		engine.SP: remainingCoins.SP,
		engine.CP: remainingCoins.CP,
	})
	newTotal := existingPP + remainingCoins.GP + remainingCoins.SP + remainingCoins.CP
	newWealth := cpAsCoinLabel(remainingInventoryCP)
	coinItem.Notes = newNotes
	coinItem.Quantity = newTotal
	if newTotal == 0 {
		if err := s.db.DeleteItem(coinItem.ID); err != nil {
			return "", "", err
		}
	} else if err := s.db.UpdateItem(&coinItem); err != nil {
		return "", "", err
	}
	return oldWealth, newWealth, nil
}

func (s *Server) buyStoreItem(ch *db.Character, name string, costCP int) (string, string, error) {
	oldWealth, newWealth, err := s.deductCoins(ch, costCP)
	if err != nil {
		return "", "", err
	}
	purchasedQty := 1
	if bundle := engine.ItemBundleSize(name); bundle > 0 {
		purchasedQty = bundle
	}
	purchased := &db.Item{CharacterID: ch.ID, Name: name, Quantity: purchasedQty, Location: "stowed"}
	if err := s.combineStackableItems(ch.ID, purchased, ch.CurrentDay); err != nil {
		if !errors.Is(err, errNotCombined) {
			return "", "", err
		}
		if err := s.db.CreateItem(purchased); err != nil {
			return "", "", err
		}
	}
	return oldWealth, newWealth, nil
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
	GP int
	SP int
	CP int
}

// minCoinCounts converts a CP value into the fewest coins using GP/SP/CP.
// PP is excluded because it is not a financial instrument in Dolmenwood.
// EP is excluded because electrum pieces don't exist in Dolmenwood.
func minCoinCounts(cpValue int) coinCounts {
	gp := cpValue / 100
	rem := cpValue % 100
	sp := rem / 10
	cp := rem % 10
	return coinCounts{GP: gp, SP: sp, CP: cp}
}

// storeSellPriceCP returns the sell price (half of buy price) for a named item.
// For bundled items, this is the price per bundle.
func storeSellPriceCP(name string) (int, bool) {
	item, ok := storeCatalogItem(name)
	if !ok || item.CostCP == 0 {
		return 0, false
	}
	return item.CostCP / 2, true
}

// cpAsCoinLabel converts a CP value to a human-readable coin label (e.g. "5gp").
func cpAsCoinLabel(cpValue int) string {
	coins := minCoinCounts(cpValue)
	return engine.FormatCoinNotes(map[engine.CoinType]int{
		engine.GP: coins.GP,
		engine.SP: coins.SP,
		engine.CP: coins.CP,
	})
}

// sellItemQuantity sells qty units of an item, returning the total CP received
// and old/new wealth labels.
func (s *Server) sellItemQuantity(ch *db.Character, item *db.Item, qty int) (int, string, string, error) {
	sellCP, ok := storeSellPriceCP(item.Name)
	if !ok {
		return 0, "", "", fmt.Errorf("item cannot be sold")
	}
	bundleSize := max(1, engine.ItemBundleSize(item.Name))
	totalSellCP := sellCP * qty / bundleSize

	// Get old wealth before any changes
	items, err := s.db.ListItems(ch.ID)
	if err != nil {
		return 0, "", "", err
	}
	oldWealthCP := 0
	if coinItem, ok := findSpendableCoins(items); ok {
		coinNotes := engine.ParseCoinNotes(coinItem.Notes)
		oldWealthCP = coinPurseCPValue(coinNotes)
	}
	oldWealth := cpAsCoinLabel(oldWealthCP)

	// Remove items
	if qty >= item.Quantity {
		if err := s.db.DeleteItem(item.ID); err != nil {
			return 0, "", "", err
		}
	} else {
		item.Quantity -= qty
		if err := s.db.UpdateItem(item); err != nil {
			return 0, "", "", err
		}
	}

	if totalSellCP == 0 {
		return 0, oldWealth, oldWealth, nil
	}

	// Add coins
	coins := minCoinCounts(totalSellCP)
	coinMap := make(map[engine.CoinType]int)
	totalCoins := 0
	if coins.GP > 0 {
		coinMap[engine.GP] = coins.GP
		totalCoins += coins.GP
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
				return 0, "", "", err
			}
			if err := s.db.CreateItem(coinItem); err != nil {
				return 0, "", "", err
			}
		}
	}

	newWealthCP := oldWealthCP + totalSellCP
	newWealth := cpAsCoinLabel(newWealthCP)

	return totalSellCP, oldWealth, newWealth, nil
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

	totalSellCP, oldWealth, newWealth, err := s.sellItemQuantity(ch, item, item.Quantity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.addAuditLog(ch, "store_sell", fmt.Sprintf("sold %s for %s, wealth %s -> %s", item.Name, itemCostLabel(totalSellCP, ""), oldWealth, newWealth))
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
