package server

import (
	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/engine"
)

// CharacterView combines DB data with engine computations for rendering.
type CharacterView struct {
	Character *db.Character
	Items     []db.Item
	Companions []db.Companion
	Transactions []db.Transaction
	XPLog     []db.XPLogEntry
	Notes     []db.Note

	// Computed fields
	AC           int
	AttackBonus  int
	Saves        engine.SaveTargets
	Traits       engine.Traits
	Speed        int
	EquippedSlots int
	StowedSlots  int
	CoinSlotsCount int
	XPModPercent int
	XPToNext     int
	NewLevel     int
	CanLevelUp   bool
	PurseGPValue int
	FoundGPValue int
	TotalPurseCoins int
	TotalFoundCoins int
}

func buildCharacterView(d *db.DB, ch *db.Character) (*CharacterView, error) {
	items, err := d.ListItems(ch.ID)
	if err != nil {
		return nil, err
	}
	companions, err := d.ListCompanions(ch.ID)
	if err != nil {
		return nil, err
	}
	transactions, err := d.ListTransactions(ch.ID)
	if err != nil {
		return nil, err
	}
	xpLog, err := d.ListXPLog(ch.ID)
	if err != nil {
		return nil, err
	}
	notes, err := d.ListNotes(ch.ID)
	if err != nil {
		return nil, err
	}

	// Convert items for engine calculations
	engineItems := make([]engine.Item, len(items))
	for i, item := range items {
		engineItems[i] = engine.Item{
			SlotCost: item.SlotCost,
			Quantity: item.Quantity,
			Location: item.Location,
		}
	}

	scores := map[string]int{
		"str": ch.STR, "dex": ch.DEX, "con": ch.CON,
		"int": ch.INT, "wis": ch.WIS, "cha": ch.CHA,
	}
	primes := []string{"str"} // Knight prime is STR

	equippedSlots := engine.TotalEquippedSlots(engineItems)
	stowedSlots := engine.TotalStowedSlots(engineItems)

	pursePurse := engine.CoinPurse{CP: ch.PurseCP, SP: ch.PurseSP, EP: ch.PurseEP, GP: ch.PurseGP, PP: ch.PursePP}
	foundPurse := engine.CoinPurse{CP: ch.FoundCP, SP: ch.FoundSP, EP: ch.FoundEP, GP: ch.FoundGP, PP: ch.FoundPP}

	totalCoins := engine.TotalCoins(pursePurse) + engine.TotalCoins(foundPurse)
	coinSlots := engine.CoinSlots(totalCoins)
	totalStowed := stowedSlots + coinSlots

	xpMod := engine.HumanTotalXPModifier(scores, primes)
	newLevel, canLevelUp := engine.DetectLevelUp(ch.Level, ch.TotalXP)

	return &CharacterView{
		Character:       ch,
		Items:           items,
		Companions:      companions,
		Transactions:    transactions,
		XPLog:           xpLog,
		Notes:           notes,
		AC:              engine.ACFromArmor(ch.ArmorAC, ch.DEX, ch.HasShield),
		AttackBonus:     engine.KnightAttackBonus(ch.Level),
		Saves:           engine.KnightSaveTargets(ch.Level),
		Traits:          engine.KnightTraits(ch.Level),
		Speed:           engine.SpeedFromSlots(equippedSlots, totalStowed),
		EquippedSlots:   equippedSlots,
		StowedSlots:     stowedSlots,
		CoinSlotsCount:  coinSlots,
		XPModPercent:    xpMod,
		XPToNext:        engine.XPToNextLevel(ch.Level, ch.TotalXP),
		NewLevel:        newLevel,
		CanLevelUp:      canLevelUp,
		PurseGPValue:    engine.CoinPurseGPValue(pursePurse),
		FoundGPValue:    engine.CoinPurseGPValue(foundPurse),
		TotalPurseCoins: engine.TotalCoins(pursePurse),
		TotalFoundCoins: engine.TotalCoins(foundPurse),
	}, nil
}
