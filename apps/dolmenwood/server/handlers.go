package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

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
		ArmorName:  r.FormValue("armor_name"),
		ArmorAC:    atoi(r.FormValue("armor_ac")),
		Alignment:  r.FormValue("alignment"),
		Background: r.FormValue("background"),
		Liege:      r.FormValue("liege"),
	}
	if ch.ArmorAC == 0 {
		ch.ArmorAC = 10
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
	slotCost := atoi(r.FormValue("slot_cost"))
	if slotCost == 0 {
		slotCost = 1
	}
	item := &db.Item{
		CharacterID: ch.ID,
		Name:        r.FormValue("name"),
		SlotCost:    slotCost,
		Quantity:    1,
		Location:    r.FormValue("location"),
	}
	if item.Location == "" {
		item.Location = "stowed"
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

func (s *Server) handleAddCompanion(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	hpMax := atoi(r.FormValue("hp_max"))
	comp := &db.Companion{
		CharacterID:  ch.ID,
		Name:         r.FormValue("name"),
		Breed:        r.FormValue("breed"),
		HPCurrent:    hpMax,
		HPMax:        hpMax,
		AC:           atoi(r.FormValue("ac")),
		Speed:        atoi(r.FormValue("speed")),
		LoadCapacity: atoi(r.FormValue("load_capacity")),
	}
	if err := s.db.CreateCompanion(comp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.db.AddAuditLog(ch.ID, "companion_add", comp.Name)
	s.renderCompanions(w, r, ch)
}

func (s *Server) handleUpdateCompanionHP(w http.ResponseWriter, r *http.Request) {
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
			comp.HPCurrent = atoi(r.FormValue("hp_current"))
			s.db.UpdateCompanion(&comp)
			break
		}
	}
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

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoui(s string) uint {
	n, _ := strconv.ParseUint(s, 10, 64)
	return uint(n)
}
