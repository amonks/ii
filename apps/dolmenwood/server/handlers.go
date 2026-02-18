package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/engine"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	chars, err := s.db.ListCharacters()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	CharacterList(chars).Render(r.Context(), w)
}

func (s *Server) handleCreateCharacter(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	hpMax := atoi(r.FormValue("hp_max"))
	ch := &db.Character{
		Name:       r.FormValue("name"),
		Class:      "Knight",
		Kindred:    "Human",
		Level:      1,
		STR:        atoi(r.FormValue("str")),
		DEX:        atoi(r.FormValue("dex")),
		CON:        atoi(r.FormValue("con")),
		INT:        atoi(r.FormValue("int")),
		WIS:        atoi(r.FormValue("wis")),
		CHA:        atoi(r.FormValue("cha")),
		HPCurrent:  hpMax,
		HPMax:      hpMax,
		Alignment:  r.FormValue("alignment"),
		Background: r.FormValue("background"),
		Liege:      r.FormValue("liege"),
	}
	if err := s.db.CreateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("%d/", ch.ID))
	w.WriteHeader(http.StatusSeeOther)
}

func (s *Server) handleCharacterSheet(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	CharacterSheet(view).Render(r.Context(), w)
}

func (s *Server) handleUpdateHP(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	oldHP := ch.HPCurrent
	ch.HPCurrent = atoi(r.FormValue("hp_current"))
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "hp_change", fmt.Sprintf("HP %d → %d", oldHP, ch.HPCurrent))
	s.renderStats(w, r, ch)
}

func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	name, qty := parseItemInput(r.FormValue("name"))
	isTiny, name := extractTinyFlag(name)
	item := &db.Item{
		CharacterID: ch.ID,
		Name:        name,
		Quantity:    qty,
		Location:    r.FormValue("location"),
		IsTiny:      isTiny,
	}
	if item.Location == "" {
		item.Location = "stowed"
	}
	// Support adding directly into a container or onto a companion
	if moveTo := r.FormValue("move_to"); moveTo != "" {
		containerID, companionID := parseMoveTarget(moveTo)
		item.ContainerID = containerID
		item.CompanionID = companionID
		item.Location = "" // clear legacy location when using hierarchy
	}
	if item.Location != "" && item.ContainerID == nil && item.CompanionID == nil {
		if err := s.db.CreateItem(item); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.db.AddAuditLog(ch.ID, "item_add", item.Name)
		s.renderInventory(w, r, ch)
		return
	}
	if bundle := engine.ItemBundleSize(item.Name); bundle > 0 {
		items, err := s.db.ListItems(ch.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, existing := range items {
			if strings.EqualFold(existing.Name, item.Name) && sameID(existing.ContainerID, item.ContainerID) && sameID(existing.CompanionID, item.CompanionID) {
				if existing.ContainerID == nil && existing.CompanionID == nil {
					if existing.Location != item.Location {
						continue
					}
				} else if existing.Location != "" {
					continue
				}
				existing.Quantity += item.Quantity
				if err := s.db.UpdateItem(&existing); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				s.db.AddAuditLog(ch.ID, "item_add", fmt.Sprintf("%s (bundled)", item.Name))
				s.renderInventory(w, r, ch)
				return
			}
		}
	}
	if err := s.db.CreateItem(item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "item_add", item.Name)
	s.renderInventory(w, r, ch)
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	itemID := atoui(r.PathValue("itemID"))
	items, err := s.db.ListItems(ch.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, item := range items {
		if item.ID == itemID {
			if loc := r.FormValue("location"); loc != "" {
				item.Location = loc
			}
			if qty := r.FormValue("quantity"); qty != "" {
				item.Quantity = atoi(qty)
			}
			// Support move_to for container hierarchy
			if moveTo := r.FormValue("move_to"); moveTo != "" {
				containerID, companionID := parseMoveTarget(moveTo)
				item.ContainerID = containerID
				item.CompanionID = companionID
				item.Location = "" // clear legacy location
			}
			s.db.UpdateItem(&item)
			break
		}
	}
	s.renderInventory(w, r, ch)
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	itemID := atoui(r.PathValue("itemID"))
	if err := s.db.DeleteItem(itemID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "item_delete", fmt.Sprintf("item %d", itemID))
	s.renderInventory(w, r, ch)
}

func (s *Server) handleDecrementItem(w http.ResponseWriter, r *http.Request) {
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
	bundle := engine.ItemBundleSize(item.Name)
	if bundle == 0 {
		http.Error(w, "Item is not bundled", http.StatusBadRequest)
		return
	}
	item.Quantity -= bundle
	if item.Quantity <= 0 {
		if err := s.db.DeleteItem(item.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err := s.db.UpdateItem(item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderInventory(w, r, ch)
}

func (s *Server) handleAddCompanion(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	breed := r.FormValue("breed")
	comp := &db.Companion{
		CharacterID: ch.ID,
		Name:        r.FormValue("name"),
		Breed:       breed,
	}
	if stats, ok := engine.BreedStats(breed); ok {
		comp.HPMax = stats.HPMax
		comp.HPCurrent = stats.HPMax
	}
	if err := s.db.CreateCompanion(comp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "companion_add", comp.Name)
	s.renderCompanions(w, r, ch)
}

func (s *Server) handleUpdateCompanion(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	compID := atoui(r.PathValue("compID"))
	comps, err := s.db.ListCompanions(ch.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, comp := range comps {
		if comp.ID == compID {
			if name := r.FormValue("name"); name != "" {
				comp.Name = name
			}
			comp.HPCurrent = atoi(r.FormValue("hp_current"))
			if hpMax := r.FormValue("hp_max"); hpMax != "" {
				comp.HPMax = atoi(hpMax)
			}
			comp.SaddleType = r.FormValue("saddle_type")
			comp.HasBarding = r.FormValue("has_barding") == "on"
			s.db.UpdateCompanion(&comp)
			break
		}
	}
	s.renderCompanions(w, r, ch)
}

func (s *Server) handleDeleteCompanion(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	compID := atoui(r.PathValue("compID"))
	if err := s.db.DeleteCompanion(compID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "companion_delete", fmt.Sprintf("companion %d", compID))
	s.renderCompanions(w, r, ch)
}

func (s *Server) handleAddTreasure(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	entry := r.FormValue("entry")
	txType := r.FormValue("type")

	amount, coinType, desc, err := engine.ParseTransaction(entry)
	if err != nil {
		http.Error(w, "Invalid format. Use: 50g dragon hoard", http.StatusBadRequest)
		return
	}

	isFound := txType == "found"
	tx := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          amount,
		CoinType:        coinType,
		Description:     desc,
		IsFoundTreasure: isFound,
	}
	if err := s.db.CreateTransaction(tx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update character coins
	if isFound {
		addToFound(ch, amount, coinType)
	} else {
		addToPurse(ch, amount, coinType)
	}
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.db.AddAuditLog(ch.ID, "treasure_add", fmt.Sprintf("%d %s %s (%s)", amount, coinType, desc, txType))
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleUndoTransaction(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	txID := atoui(r.PathValue("txID"))
	orig, err := s.db.GetTransaction(txID)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Create inverse transaction
	desc := "undo"
	if orig.Description != "" {
		desc = "undo " + orig.Description
	}
	undo := &db.Transaction{
		CharacterID:     ch.ID,
		Amount:          -orig.Amount,
		CoinType:        orig.CoinType,
		Description:     desc,
		IsFoundTreasure: orig.IsFoundTreasure,
	}
	if err := s.db.CreateTransaction(undo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Reverse the coin effect
	if orig.IsFoundTreasure {
		addToFound(ch, -orig.Amount, orig.CoinType)
	} else {
		addToPurse(ch, -orig.Amount, orig.CoinType)
	}
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.db.AddAuditLog(ch.ID, "treasure_undo", desc)
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleReturnToSafety(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}

	scores := map[string]int{
		"str": ch.STR, "dex": ch.DEX, "con": ch.CON,
		"int": ch.INT, "wis": ch.WIS, "cha": ch.CHA,
	}
	xpMod := engine.HumanTotalXPModifier(scores, []string{"str"})

	if err := s.db.ReturnToSafety(ch.ID, xpMod); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Reload
	ch, _ = s.db.GetCharacter(ch.ID)
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleLevelUp(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}

	newLevel, canLevel := engine.DetectLevelUp(ch.Level, ch.TotalXP)
	if !canLevel {
		http.Error(w, "Not enough XP to level up", http.StatusBadRequest)
		return
	}

	oldLevel := ch.Level
	ch.Level = newLevel
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "level_up", fmt.Sprintf("Level %d → %d", oldLevel, newLevel))
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleAddXP(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	xpAmount := atoi(r.FormValue("xp_amount"))
	description := r.FormValue("description")
	if xpAmount == 0 {
		http.Error(w, "XP amount must be non-zero", http.StatusBadRequest)
		return
	}
	if err := s.db.CreateXPLogEntry(&db.XPLogEntry{
		CharacterID: ch.ID,
		Amount:      xpAmount,
		Description: description,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ch.TotalXP += xpAmount
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "xp_add", fmt.Sprintf("+%d XP (%s)", xpAmount, description))
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	note := &db.Note{
		CharacterID: ch.ID,
		Content:     r.FormValue("content"),
	}
	if err := s.db.CreateNote(note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderNotes(w, r, ch)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	noteID := atoui(r.PathValue("noteID"))
	if err := s.db.DeleteNote(noteID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderNotes(w, r, ch)
}

// --- Helpers ---

func (s *Server) getCharacter(r *http.Request) (*db.Character, error) {
	id := atoui(r.PathValue("id"))
	return s.db.GetCharacter(id)
}

func (s *Server) renderStats(w http.ResponseWriter, r *http.Request, ch *db.Character) {
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		slog.Error("renderStats", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	StatsSection(view).Render(r.Context(), w)
}

func (s *Server) renderInventory(w http.ResponseWriter, r *http.Request, ch *db.Character) {
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		slog.Error("renderInventory", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	InventorySection(view).Render(r.Context(), w)
	// OOB swap: also update encumbrance section (speed, slots) when inventory changes
	var buf bytes.Buffer
	EncumbranceSection(view).Render(r.Context(), &buf)
	oob := strings.Replace(buf.String(), `id="encumbrance"`, `id="encumbrance" hx-swap-oob="outerHTML"`, 1)
	fmt.Fprint(w, oob)
}

func (s *Server) renderCompanions(w http.ResponseWriter, r *http.Request, ch *db.Character) {
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		slog.Error("renderCompanions", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	CompanionsSection(view).Render(r.Context(), w)
}

func (s *Server) renderNotes(w http.ResponseWriter, r *http.Request, ch *db.Character) {
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		slog.Error("renderNotes", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	NotesSection(view).Render(r.Context(), w)
}

func (s *Server) renderSheetBody(w http.ResponseWriter, r *http.Request, ch *db.Character) {
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		slog.Error("renderSheetBody", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SheetBody(view).Render(r.Context(), w)
}

func addToFound(ch *db.Character, amount int, coinType string) {
	switch coinType {
	case engine.CP:
		ch.FoundCP += amount
	case engine.SP:
		ch.FoundSP += amount
	case engine.EP:
		ch.FoundEP += amount
	case engine.GP:
		ch.FoundGP += amount
	case engine.PP:
		ch.FoundPP += amount
	}
}

func addToPurse(ch *db.Character, amount int, coinType string) {
	switch coinType {
	case engine.CP:
		ch.PurseCP += amount
	case engine.SP:
		ch.PurseSP += amount
	case engine.EP:
		ch.PurseEP += amount
	case engine.GP:
		ch.PurseGP += amount
	case engine.PP:
		ch.PursePP += amount
	}
}

// parseItemInput parses "5x preserved rations" into ("preserved rations", 5)
// or just "rope" into ("rope", 1).
func parseItemInput(input string) (string, int) {
	input = strings.TrimSpace(input)
	if idx := strings.Index(strings.ToLower(input), "x "); idx > 0 {
		if qty := atoi(input[:idx]); qty > 0 {
			return strings.TrimSpace(input[idx+2:]), qty
		}
	}
	return input, 1
}

func extractTinyFlag(name string) (bool, string) {
	if name == "" {
		return false, name
	}
	if isKnownItemName(name) {
		return false, name
	}
	words := strings.Fields(name)
	var cleaned []string
	found := false
	for _, word := range words {
		if strings.EqualFold(word, "tiny") {
			found = true
			continue
		}
		cleaned = append(cleaned, word)
	}
	if !found {
		return false, name
	}
	cleanedName := strings.TrimSpace(strings.Join(cleaned, " "))
	if cleanedName == "" {
		return false, name
	}
	return true, cleanedName
}

func isKnownItemName(name string) bool {
	if _, explicit := engine.ItemSlotCostExplicit(name); explicit {
		return true
	}
	if _, ok := engine.ItemWeight(name); ok {
		return true
	}
	return false
}

func sameID(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoui(s string) uint {
	n, _ := strconv.ParseUint(s, 10, 64)
	return uint(n)
}
