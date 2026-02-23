package server

import (
	"bytes"
	"errors"
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

func (s *Server) handleDeleteCharacter(w http.ResponseWriter, r *http.Request) {
	id := atoui(r.PathValue("id"))
	if err := s.db.DeleteCharacter(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
	s.addAuditLog(ch, "character_create", fmt.Sprintf("created %s, Level %d %s %s", ch.Name, ch.Level, ch.Kindred, ch.Class))
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
	oldHPCurrent := ch.HPCurrent
	oldHPMax := ch.HPMax
	ch.HPCurrent = atoi(r.FormValue("hp_current"))
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.addAuditLog(ch, "hp_change", fmt.Sprintf("HP %d/%d → %d/%d", oldHPCurrent, oldHPMax, ch.HPCurrent, ch.HPMax))
	s.renderStats(w, r, ch)
}

func (s *Server) handleUpdateBirthday(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	oldMonth, oldDay := ch.BirthdayMonth, ch.BirthdayDay
	ch.BirthdayMonth = strings.TrimSpace(r.FormValue("birthday_month"))
	ch.BirthdayDay = atoi(r.FormValue("birthday_day"))
	if maxDay, ok := engine.DaysInMonth(ch.BirthdayMonth); ok {
		if ch.BirthdayDay > maxDay {
			ch.BirthdayDay = maxDay
		}
	} else {
		ch.BirthdayMonth = ""
		ch.BirthdayDay = 0
	}
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if oldMonth != "" || ch.BirthdayMonth != "" {
		oldLabel := "unset"
		if oldMonth != "" {
			oldLabel = fmt.Sprintf("%s %d", oldMonth, oldDay)
		}
		newLabel := "unset"
		if ch.BirthdayMonth != "" {
			newLabel = fmt.Sprintf("%s %d", ch.BirthdayMonth, ch.BirthdayDay)
		}
		s.addAuditLog(ch, "birthday_update", fmt.Sprintf("birthday %s → %s", oldLabel, newLabel))
	}
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	rawName := strings.TrimSpace(r.FormValue("name"))

	// Try to recognize coin expressions like "5 cp", "50gp 10sp"
	if amounts, err := engine.ParseCoinExpression(rawName); err == nil {
		coinMap := make(map[engine.CoinType]int)
		totalCoins := 0
		for _, a := range amounts {
			coinMap[a.CoinType] += a.Amount
			totalCoins += a.Amount
		}
		item := &db.Item{
			CharacterID: ch.ID,
			Name:        engine.CoinItemNameStr,
			Quantity:    totalCoins,
			Notes:       engine.FormatCoinNotes(coinMap),
			Location:    r.FormValue("location"),
		}
		if item.Location == "" {
			item.Location = "stowed"
		}
		if moveTo := r.FormValue("move_to"); moveTo != "" {
			containerID, companionID := parseMoveTarget(moveTo)
			item.ContainerID = containerID
			item.CompanionID = companionID
			item.Location = ""
		}
		if err := s.combineStackableItems(ch.ID, item, ch.CurrentDay); err != nil {
			if !errors.Is(err, errNotCombined) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.db.CreateItem(item); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.addAuditLog(ch, "item_add", fmt.Sprintf("add Coins (%s) in %s", item.Notes, s.itemLocationLabel(item)))
		}
		s.renderInventory(w, r, ch)
		return
	}

	name, qty := parseItemInput(rawName)

	// Negative quantity: deduct from existing inventory items
	if qty < 0 {
		deductQty := -qty
		if err := s.deductItemQuantity(ch, name, deductQty); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.addAuditLog(ch, "item_deduct", fmt.Sprintf("deduct %s, qty %d", name, deductQty))
		s.renderInventory(w, r, ch)
		return
	}

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
		if err := s.combineStackableItems(ch.ID, item, ch.CurrentDay); err != nil {
			if !errors.Is(err, errNotCombined) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			s.renderInventory(w, r, ch)
			return
		}
		if err := s.db.CreateItem(item); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.addAuditLog(ch, "item_add", fmt.Sprintf("add %s, qty %d in %s", item.Name, item.Quantity, s.itemLocationLabel(item)))
		s.renderInventory(w, r, ch)
		return
	}
	if err := s.combineStackableItems(ch.ID, item, ch.CurrentDay); err != nil {
		if !errors.Is(err, errNotCombined) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		s.renderInventory(w, r, ch)
		return
	}
	if err := s.db.CreateItem(item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.addAuditLog(ch, "item_add", fmt.Sprintf("add %s, qty %d in %s", item.Name, item.Quantity, s.itemLocationLabel(item)))
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
			oldQty := item.Quantity
			oldNotes := item.Notes
			oldLocation := s.itemLocationLabel(&item)

			if loc := r.FormValue("location"); loc != "" {
				item.Location = loc
			}
			if qty := r.FormValue("quantity"); qty != "" {
				item.Quantity = atoi(qty)
			}
			if notes := r.FormValue("notes"); notes != "" {
				item.Notes = notes
			} else if r.FormValue("has_notes") != "" {
				item.Notes = ""
			}
			// Support move_to for container hierarchy
			if moveTo := r.FormValue("move_to"); moveTo != "" {
				// Consume: delete item
				if moveTo == "consume" && !strings.EqualFold(item.Name, engine.CoinItemNameStr) {
					if err := s.db.DeleteItem(item.ID); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					s.addAuditLog(ch, "item_consume", fmt.Sprintf("consume %s, qty %d", item.Name, item.Quantity))
					s.renderInventory(w, r, ch)
					return
				}

				// Bank deposit: convert coin item to a bank deposit
				if moveTo == "bank" && strings.EqualFold(item.Name, engine.CoinItemNameStr) {
					// Parse coin notes and reject PP/EP
					parsed := engine.ParseCoinNotes(item.Notes)
					for ct := range parsed {
						if ct == engine.PP {
							http.Error(w, "Banks don't deal in fairy silver (pp)", http.StatusBadRequest)
							return
						}
						if ct == engine.EP {
							http.Error(w, "Electrum pieces don't exist in Dolmenwood", http.StatusBadRequest)
							return
						}
					}

					cpValue := engine.CoinNotesCPValue(item.Notes)

					// Delete the coin item
					if err := s.db.DeleteItem(item.ID); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					// Create bank deposit
					dep := &db.BankDeposit{
						CharacterID: ch.ID,
						CoinNotes:   item.Notes,
						CPValue:     cpValue,
						DepositDay:  ch.CurrentDay,
					}
					if err := s.db.CreateBankDeposit(dep); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}

					s.addAuditLog(ch, "bank_deposit", fmt.Sprintf("deposit %s (%d cp) from %s to bank", item.Notes, cpValue, oldLocation))
					s.renderSheetBody(w, r, ch)
					return
				}

				containerID, companionID := parseMoveTarget(moveTo)
				item.ContainerID = containerID
				item.CompanionID = companionID
				item.Location = "" // clear legacy location
			}
			s.db.UpdateItem(&item)

			// Build audit log based on what actually changed
			newLocation := s.itemLocationLabel(&item)
			var changes []string
			if newLocation != oldLocation {
				changes = append(changes, fmt.Sprintf("moved from %s to %s", oldLocation, newLocation))
			}
			if item.Quantity != oldQty {
				changes = append(changes, fmt.Sprintf("qty %d → %d", oldQty, item.Quantity))
			}
			if item.Notes != oldNotes {
				if item.Notes == "" {
					changes = append(changes, "notes cleared")
				} else {
					changes = append(changes, fmt.Sprintf("notes: %s", item.Notes))
				}
			}
			if len(changes) > 0 {
				s.addAuditLog(ch, "item_update", fmt.Sprintf("%s: %s", item.Name, strings.Join(changes, ", ")))
			}

			if item.ContainerID != nil || item.CompanionID != nil {
				if err := s.combineStackableItems(ch.ID, &item, ch.CurrentDay); err != nil {
					if !errors.Is(err, errNotCombined) {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
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
	item, _ := s.db.GetItem(itemID)
	if err := s.db.DeleteItem(itemID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	detail := "deleted item"
	if item != nil {
		detail = fmt.Sprintf("delete %s, qty %d from %s", item.Name, item.Quantity, s.itemLocationLabel(item))
	}
	s.addAuditLog(ch, "item_delete", detail)
	s.renderInventory(w, r, ch)
}

func (s *Server) handleSplitItem(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	itemID := atoui(r.PathValue("itemID"))
	source, err := s.db.GetItem(itemID)
	if err != nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	sourceLabel := s.itemLocationLabel(source)

	moveTo := r.FormValue("move_to")
	containerID, companionID := parseMoveTarget(moveTo)
	qtyStr := strings.TrimSpace(r.FormValue("quantity"))

	// Empty quantity means "move all"
	if qtyStr == "" {
		if strings.EqualFold(source.Name, engine.CoinItemNameStr) {
			qtyStr = source.Notes
		} else {
			qtyStr = strconv.Itoa(source.Quantity)
		}
	}

	if strings.EqualFold(source.Name, engine.CoinItemNameStr) {
		// Consolidated coin split: parse as coin expression, subtract from source notes
		amounts, err := engine.ParseCoinExpression(qtyStr)
		if err != nil {
			http.Error(w, "Invalid coin expression", http.StatusBadRequest)
			return
		}

		newNotes, newTotal, err := engine.SubtractCoinNotes(source.Notes, amounts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Build the split notes for the destination
		splitMap := make(map[engine.CoinType]int)
		splitTotal := 0
		for _, a := range amounts {
			splitMap[a.CoinType] += a.Amount
			splitTotal += a.Amount
		}
		splitNotes := engine.FormatCoinNotes(splitMap)

		// Reject consume for coin items
		if moveTo == "consume" {
			http.Error(w, "Cannot consume coins", http.StatusBadRequest)
			return
		}

		// Bank deposit: convert coins to a bank deposit instead of moving
		if moveTo == "bank" {
			// Reject PP and EP
			for _, a := range amounts {
				if a.CoinType == engine.PP {
					http.Error(w, "Banks don't deal in fairy silver (pp)", http.StatusBadRequest)
					return
				}
				if a.CoinType == engine.EP {
					http.Error(w, "Electrum pieces don't exist in Dolmenwood", http.StatusBadRequest)
					return
				}
			}

			cpValue := engine.CoinNotesCPValue(splitNotes)

			// Update or delete source item
			if newTotal == 0 {
				if err := s.db.DeleteItem(source.ID); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				source.Notes = newNotes
				source.Quantity = newTotal
				if err := s.db.UpdateItem(source); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			// Create bank deposit
			dep := &db.BankDeposit{
				CharacterID: ch.ID,
				CoinNotes:   splitNotes,
				CPValue:     cpValue,
				DepositDay:  ch.CurrentDay,
			}
			if err := s.db.CreateBankDeposit(dep); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			s.addAuditLog(ch, "bank_deposit", fmt.Sprintf("deposit %s (%d cp) from %s to bank", splitNotes, cpValue, sourceLabel))
			s.renderSheetBody(w, r, ch)
			return
		}

		if newTotal == 0 {
			// Moving all coins: just move the source item
			source.Notes = splitNotes
			source.Quantity = splitTotal
			source.ContainerID = containerID
			source.CompanionID = companionID
			source.Location = ""
			if err := s.db.UpdateItem(source); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Try to merge at destination
			s.combineStackableItems(ch.ID, source, ch.CurrentDay)
		} else {
			// Update source with remaining
			source.Notes = newNotes
			source.Quantity = newTotal
			if err := s.db.UpdateItem(source); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Create new coin item at destination
			newItem := &db.Item{
				CharacterID: ch.ID,
				Name:        engine.CoinItemNameStr,
				Quantity:    splitTotal,
				Notes:       splitNotes,
				ContainerID: containerID,
				CompanionID: companionID,
			}
			if err := s.combineStackableItems(ch.ID, newItem, ch.CurrentDay); err != nil {
				if !errors.Is(err, errNotCombined) {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if err := s.db.CreateItem(newItem); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		s.addAuditLog(ch, "item_split", fmt.Sprintf("move Coins %s from %s to %s", qtyStr, sourceLabel, s.resolveMoveTargetLabel(moveTo)))
	} else {
		// Non-coin split: parse quantity as integer
		qty := atoi(qtyStr)
		if qty <= 0 {
			http.Error(w, "Invalid quantity", http.StatusBadRequest)
			return
		}
		if qty > source.Quantity {
			http.Error(w, fmt.Sprintf("Not enough %s (have %d, want %d)", source.Name, source.Quantity, qty), http.StatusBadRequest)
			return
		}
		if moveTo == "consume" {
			source.Quantity -= qty
			if source.Quantity <= 0 {
				if err := s.db.DeleteItem(source.ID); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				if err := s.db.UpdateItem(source); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			s.addAuditLog(ch, "item_consume", fmt.Sprintf("consume %s, qty %d", source.Name, qty))
		} else if moveTo == "sell" {
			totalSellCP, oldWealth, newWealth, err := s.sellItemQuantity(ch, source, qty)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.addAuditLog(ch, "store_sell", fmt.Sprintf("sold %d %s for %s, wealth %s -> %s", qty, source.Name, itemCostLabel(totalSellCP, ""), oldWealth, newWealth))
		} else if qty == source.Quantity {
			// Move the whole item
			source.ContainerID = containerID
			source.CompanionID = companionID
			source.Location = ""
			if err := s.db.UpdateItem(source); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.combineStackableItems(ch.ID, source, ch.CurrentDay); err != nil {
				if !errors.Is(err, errNotCombined) {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			s.addAuditLog(ch, "item_split", fmt.Sprintf("move %s qty %d from %s to %s", source.Name, qty, sourceLabel, s.resolveMoveTargetLabel(moveTo)))
		} else {
			// Reduce source and create new item
			source.Quantity -= qty
			if err := s.db.UpdateItem(source); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			newItem := &db.Item{
				CharacterID: ch.ID,
				Name:        source.Name,
				Quantity:    qty,
				ContainerID: containerID,
				CompanionID: companionID,
			}
			if err := s.db.CreateItem(newItem); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.combineStackableItems(ch.ID, newItem, ch.CurrentDay); err != nil {
				if !errors.Is(err, errNotCombined) {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			s.addAuditLog(ch, "item_split", fmt.Sprintf("move %s qty %d from %s to %s", source.Name, qty, sourceLabel, s.resolveMoveTargetLabel(moveTo)))
		}
	}

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
	oldQty := item.Quantity
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
	s.addAuditLog(ch, "item_decrement", fmt.Sprintf("%s -%d (%d → %d)", item.Name, bundle, oldQty, item.Quantity))
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
	if engine.IsRetainer(breed) {
		comp.Loyalty = engine.RetainerLoyalty(engine.Modifier(ch.CHA))
	}
	if err := s.db.CreateCompanion(comp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.addAuditLog(ch, "companion_add", fmt.Sprintf("created %s the %s, HP %d/%d", comp.Name, comp.Breed, comp.HPCurrent, comp.HPMax))
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
			oldName := comp.Name
			oldHP := comp.HPCurrent
			oldHPMax := comp.HPMax
			oldLoyalty := comp.Loyalty

			if name := r.FormValue("name"); name != "" {
				comp.Name = name
			}
			comp.HPCurrent = atoi(r.FormValue("hp_current"))
			if hpMax := r.FormValue("hp_max"); hpMax != "" {
				comp.HPMax = atoi(hpMax)
			}
			if loyalty := r.FormValue("loyalty"); loyalty != "" && engine.IsRetainer(comp.Breed) {
				comp.Loyalty = atoi(loyalty)
			}
			s.db.UpdateCompanion(&comp)

			var changes []string
			if comp.Name != oldName {
				changes = append(changes, fmt.Sprintf("name %s → %s", oldName, comp.Name))
			}
			if comp.HPCurrent != oldHP || comp.HPMax != oldHPMax {
				changes = append(changes, fmt.Sprintf("HP %d/%d → %d/%d", oldHP, oldHPMax, comp.HPCurrent, comp.HPMax))
			}
			if engine.IsRetainer(comp.Breed) && comp.Loyalty != oldLoyalty {
				changes = append(changes, fmt.Sprintf("loyalty %d → %d", oldLoyalty, comp.Loyalty))
			}
			if len(changes) > 0 {
				s.addAuditLog(ch, "companion_update", fmt.Sprintf("%s: %s", comp.Name, strings.Join(changes, ", ")))
			}
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
	comp, _ := s.db.GetCompanion(compID)
	if err := s.db.DeleteCompanion(compID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	detail := "deleted companion"
	if comp != nil {
		detail = fmt.Sprintf("deleted %s the %s", comp.Name, comp.Breed)
	}
	s.addAuditLog(ch, "companion_delete", detail)
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

	// Update found treasure accounting (purse is computed from inventory)
	if isFound {
		addToFound(ch, amount, coinType)
		if err := s.db.UpdateCharacter(ch); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create or merge consolidated coin inventory item
	coinNotes := engine.FormatCoinNotes(map[engine.CoinType]int{coinType: amount})
	coinItem := &db.Item{
		CharacterID: ch.ID,
		Name:        engine.CoinItemNameStr,
		Quantity:    amount,
		Notes:       coinNotes,
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

	s.addAuditLog(ch, "treasure_add", fmt.Sprintf("%d %s %s (%s)", amount, coinType, desc, txType))
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

	// Reverse found treasure accounting (purse is computed from inventory)
	if orig.IsFoundTreasure {
		addToFound(ch, -orig.Amount, orig.CoinType)
		if err := s.db.UpdateCharacter(ch); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Remove coins from consolidated inventory items
	subAmounts := []engine.CoinAmount{{Amount: orig.Amount, CoinType: orig.CoinType}}
	items, err := s.db.ListItems(ch.ID)
	if err == nil {
		// Prefer items directly on character (no container, no companion)
		subtracted := false
		for i := range items {
			if !strings.EqualFold(items[i].Name, engine.CoinItemNameStr) {
				continue
			}
			if items[i].ContainerID != nil || items[i].CompanionID != nil {
				continue
			}
			newNotes, newTotal, err := engine.SubtractCoinNotes(items[i].Notes, subAmounts)
			if err != nil {
				continue // insufficient in this item, try next
			}
			if newTotal == 0 {
				s.db.DeleteItem(items[i].ID)
			} else {
				items[i].Notes = newNotes
				items[i].Quantity = newTotal
				s.db.UpdateItem(&items[i])
			}
			subtracted = true
			break
		}
		// Then from any location
		if !subtracted {
			for i := range items {
				if !strings.EqualFold(items[i].Name, engine.CoinItemNameStr) {
					continue
				}
				newNotes, newTotal, err := engine.SubtractCoinNotes(items[i].Notes, subAmounts)
				if err != nil {
					continue
				}
				if newTotal == 0 {
					s.db.DeleteItem(items[i].ID)
				} else {
					items[i].Notes = newNotes
					items[i].Quantity = newTotal
					s.db.UpdateItem(&items[i])
				}
				break
			}
		}
	}

	txTypeLabel := "earned"
	if orig.IsFoundTreasure {
		txTypeLabel = "found"
	}
	s.addAuditLog(ch, "treasure_undo", fmt.Sprintf("undo %d %s %s (%s)", orig.Amount, orig.CoinType, orig.Description, txTypeLabel))
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
	xpMod := engine.TotalXPModifier(ch.Kindred, scores, []string{"str"})

	if err := s.db.ReturnToSafety(ch.ID, xpMod, ch.CurrentDay); err != nil {
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
	s.addAuditLog(ch, "level_up", fmt.Sprintf("Level %d → %d", oldLevel, newLevel))
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
	s.addAuditLog(ch, "xp_add", fmt.Sprintf("+%d XP (%s)", xpAmount, description))
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
	s.addAuditLog(ch, "note_add", note.Content)
	s.renderNotes(w, r, ch)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	noteID := atoui(r.PathValue("noteID"))
	note, _ := s.db.GetNote(noteID)
	if err := s.db.DeleteNote(noteID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	detail := "deleted note"
	if note != nil {
		detail = note.Content
	}
	s.addAuditLog(ch, "note_delete", detail)
	s.renderNotes(w, r, ch)
}

// --- Bank / Day handlers ---

func (s *Server) handleAdvanceDay(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	delta := 1
	if deltaText := r.FormValue("day_delta"); deltaText != "" {
		parsed, err := strconv.Atoi(deltaText)
		if err != nil {
			http.Error(w, "Invalid day change", http.StatusBadRequest)
			return
		}
		delta = parsed
	}
	oldDay := ch.CurrentDay
	ch.CurrentDay += delta
	if ch.CurrentDay < 1 {
		ch.CurrentDay = 1
	}
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.addAuditLog(ch, "day_advance", fmt.Sprintf("Day %d → %d", oldDay, ch.CurrentDay))
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleUpdateCalendar(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	day := atoi(r.FormValue("calendar_day"))
	month := atoi(r.FormValue("calendar_month"))
	startDay, err := engine.StartDayOfYearForGameDay(ch.CurrentDay, month, day)
	if err != nil {
		http.Error(w, "Invalid calendar date", http.StatusBadRequest)
		return
	}
	ch.CalendarStartDay = startDay
	if err := s.db.UpdateCharacter(ch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	date, err := engine.CalendarDisplayForGameDay(ch.CalendarStartDay, ch.CurrentDay)
	if err != nil {
		http.Error(w, "Invalid calendar date", http.StatusBadRequest)
		return
	}
	if date.IsWysenday {
		s.addAuditLog(ch, "calendar_update", fmt.Sprintf("Calendar set to %s", date.Wysenday))
	} else {
		s.addAuditLog(ch, "calendar_update", fmt.Sprintf("Calendar set to %s %d", date.MonthName, day))
	}
	s.renderSheetBody(w, r, ch)
}

func (s *Server) handleBankWithdraw(w http.ResponseWriter, r *http.Request) {
	ch, err := s.getCharacter(r)
	if err != nil {
		http.Error(w, "Character not found", http.StatusNotFound)
		return
	}
	r.ParseForm()
	coinExpr := r.FormValue("coins")
	amounts, err := engine.ParseCoinExpression(coinExpr)
	if err != nil {
		http.Error(w, "Invalid coin expression", http.StatusBadRequest)
		return
	}

	// Convert requested amount to CP
	requestedCP := 0
	for _, a := range amounts {
		switch a.CoinType {
		case engine.CP:
			requestedCP += a.Amount
		case engine.SP:
			requestedCP += a.Amount * 10
		case engine.GP:
			requestedCP += a.Amount * 100
		}
	}

	// Load bank deposits and plan withdrawal
	deposits, err := s.db.ListBankDeposits(ch.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lots := make([]engine.BankLot, len(deposits))
	for i, dep := range deposits {
		lots[i] = engine.BankLot{ID: dep.ID, CPValue: dep.CPValue, DepositDay: dep.DepositDay}
	}
	result, err := engine.PlanWithdrawal(lots, requestedCP, ch.CurrentDay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Execute: delete consumed lots, update partial lots
	for _, id := range result.ConsumedLots {
		s.db.DeleteBankDeposit(id)
	}
	for id, newValue := range result.UpdatedLots {
		for i := range deposits {
			if deposits[i].ID == id {
				deposits[i].CPValue = newValue
				s.db.UpdateBankDeposit(&deposits[i])
				break
			}
		}
	}

	// Add withdrawn coins to inventory (purse is computed from inventory)
	coins := engine.MinCoins(result.NetCP)
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
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.db.CreateItem(coinItem); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	feeDetail := ""
	if result.FeeCP > 0 {
		feeDetail = fmt.Sprintf(" (fee: %d cp)", result.FeeCP)
	}
	s.addAuditLog(ch, "bank_withdraw", fmt.Sprintf("Withdrew %s%s", coinExpr, feeDetail))
	s.renderSheetBody(w, r, ch)
}

// --- Helpers ---

// resolveMoveTargetLabel resolves a move_to form value like "container:42"
// or "companion:7" into a human-readable name like "Backpack" or "Bessie".
func (s *Server) resolveMoveTargetLabel(moveTo string) string {
	if moveTo == "equipped" || moveTo == "" {
		return "Equipped"
	}
	if moveTo == "bank" {
		return "Bank"
	}
	if moveTo == "consume" {
		return "Consumed"
	}
	if after, ok := strings.CutPrefix(moveTo, "container:"); ok {
		id := atoui(after)
		if item, err := s.db.GetItem(id); err == nil {
			return item.Name
		}
	}
	if after, ok := strings.CutPrefix(moveTo, "companion:"); ok {
		id := atoui(after)
		if comp, err := s.db.GetCompanion(id); err == nil {
			return comp.Name
		}
	}
	return moveTo
}

func (s *Server) addAuditLog(ch *db.Character, action, detail string) {
	s.db.AddAuditLog(ch.ID, action, detail, ch.CurrentDay)
}

// itemLocationLabel resolves an item's current location to a human-readable label.
func (s *Server) itemLocationLabel(item *db.Item) string {
	if item.CompanionID != nil {
		if comp, err := s.db.GetCompanion(*item.CompanionID); err == nil {
			return comp.Name
		}
		return "companion"
	}
	if item.ContainerID != nil {
		if container, err := s.db.GetItem(*item.ContainerID); err == nil {
			return container.Name
		}
		return "container"
	}
	if item.Location != "" {
		return item.Location
	}
	return "inventory"
}

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
	// OOB swap: also update companions (moving saddle/barding changes companion stats)
	buf.Reset()
	CompanionsSection(view).Render(r.Context(), &buf)
	oob = strings.Replace(buf.String(), `id="companions"`, `id="companions" hx-swap-oob="outerHTML"`, 1)
	fmt.Fprint(w, oob)
}

func (s *Server) renderInventoryAndCompanions(w http.ResponseWriter, r *http.Request, ch *db.Character) {
	view, err := buildCharacterView(s.db, ch)
	if err != nil {
		slog.Error("renderInventoryAndCompanions", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	InventorySection(view).Render(r.Context(), w)
	// OOB swap encumbrance
	var buf bytes.Buffer
	EncumbranceSection(view).Render(r.Context(), &buf)
	oob := strings.Replace(buf.String(), `id="encumbrance"`, `id="encumbrance" hx-swap-oob="outerHTML"`, 1)
	fmt.Fprint(w, oob)
	// OOB swap companions
	buf.Reset()
	CompanionsSection(view).Render(r.Context(), &buf)
	oob = strings.Replace(buf.String(), `id="companions"`, `id="companions" hx-swap-oob="outerHTML"`, 1)
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
	// OOB swap: also update inventory (companion changes affect move targets and inventory sections)
	var buf bytes.Buffer
	InventorySection(view).Render(r.Context(), &buf)
	oob := strings.Replace(buf.String(), `id="inventory"`, `id="inventory" hx-swap-oob="outerHTML"`, 1)
	fmt.Fprint(w, oob)
	// OOB swap: also update encumbrance
	buf.Reset()
	EncumbranceSection(view).Render(r.Context(), &buf)
	oob = strings.Replace(buf.String(), `id="encumbrance"`, `id="encumbrance" hx-swap-oob="outerHTML"`, 1)
	fmt.Fprint(w, oob)
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

var errNotCombined = errors.New("not combined")

func (s *Server) combineStackableItems(characterID uint, item *db.Item, gameDay ...int) error {
	day := 0
	if len(gameDay) > 0 {
		day = gameDay[0]
	}
	if item.ContainerID != nil || item.CompanionID != nil {
		item.Location = ""
	}
	if engine.ItemBundleSize(item.Name) == 0 && !engine.IsCoinItem(item.Name) {
		return errNotCombined
	}
	items, err := s.db.ListItems(characterID)
	if err != nil {
		return err
	}
	for _, existing := range items {
		if existing.ID == item.ID {
			continue
		}
		if !strings.EqualFold(existing.Name, item.Name) {
			continue
		}
		if !sameID(existing.ContainerID, item.ContainerID) || !sameID(existing.CompanionID, item.CompanionID) {
			continue
		}
		if !bundleLocationsMatch(existing, item) {
			continue
		}
		// For consolidated "Coins" items, merge denomination notes
		if strings.EqualFold(existing.Name, engine.CoinItemNameStr) {
			addAmounts := engine.ParseCoinNotes(item.Notes)
			var coinAmounts []engine.CoinAmount
			for ct, qty := range addAmounts {
				if qty > 0 {
					coinAmounts = append(coinAmounts, engine.CoinAmount{Amount: qty, CoinType: ct})
				}
			}
			newNotes, newTotal := engine.MergeCoinNotes(existing.Notes, coinAmounts)
			existing.Notes = newNotes
			existing.Quantity = newTotal
		} else {
			existing.Quantity += item.Quantity
		}
		if err := s.db.UpdateItem(&existing); err != nil {
			return err
		}
		if item.ID != 0 {
			if err := s.db.DeleteItem(item.ID); err != nil {
				return err
			}
		}
		s.db.AddAuditLog(characterID, "item_add", fmt.Sprintf("add %s, qty %d (combined) in %s", item.Name, item.Quantity, s.itemLocationLabel(item)), day)
		return nil
	}
	return errNotCombined
}

func bundleLocationsMatch(existing db.Item, item *db.Item) bool {
	if existing.ContainerID == nil && existing.CompanionID == nil {
		return existing.Location == item.Location
	}
	return existing.Location == "" && item.Location == ""
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

// deductItemQuantity finds items matching name and deducts the given quantity.
// Deducts from the first matching item found. Deletes the item if quantity reaches 0.
// Returns an error if there aren't enough items without modifying anything.
func (s *Server) deductItemQuantity(ch *db.Character, name string, qty int) error {
	items, err := s.db.ListItems(ch.ID)
	if err != nil {
		return err
	}
	// First pass: check if we have enough
	available := 0
	for _, item := range items {
		if strings.EqualFold(item.Name, name) {
			available += item.Quantity
		}
	}
	if available < qty {
		return fmt.Errorf("not enough %s (have %d, need %d)", name, available, qty)
	}
	// Second pass: deduct
	remaining := qty
	for _, item := range items {
		if remaining <= 0 {
			break
		}
		if !strings.EqualFold(item.Name, name) {
			continue
		}
		if item.Quantity <= remaining {
			remaining -= item.Quantity
			if err := s.db.DeleteItem(item.ID); err != nil {
				return err
			}
		} else {
			item.Quantity -= remaining
			remaining = 0
			if err := s.db.UpdateItem(&item); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseItemInput parses "5x preserved rations" into ("preserved rations", 5)
// or "-2x feed" into ("feed", -2), or just "rope" into ("rope", 1).
func parseItemInput(input string) (string, int) {
	input = strings.TrimSpace(input)
	if idx := strings.Index(strings.ToLower(input), "x "); idx > 0 {
		if qty := atoi(input[:idx]); qty != 0 {
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
