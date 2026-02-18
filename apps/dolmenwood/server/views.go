package server

import (
	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/engine"
)

// CharacterView combines DB data with engine computations for rendering.
type CharacterView struct {
	Character *db.Character
	Items     []db.Item
	Companions []CompanionView
	Transactions []db.Transaction
	XPLog     []db.XPLogEntry
	Notes     []db.Note

	// Computed fields
	AC           int
	ArmorName    string
	AttackBonus  int
	Weapons      []engine.EquippedWeapon
	Saves        engine.SaveTargets
	Traits       engine.Traits
	Speed        int
	EquippedSlots   int
	StowedSlots     int
	CoinSlotsCount  int
	TotalStowedSlots int
	HorseSlots      int
	StowedCapacity  int
	StowedContainers []engine.ContainerInfo
	HorseCapacity   int
	XPModPercent int
	XPToNext     int
	NewLevel     int
	CanLevelUp   bool
	PurseGPValue int
	FoundGPValue int
	TotalPurseCoins int
	TotalFoundCoins int
	BreedNames      []string
}

// CompanionView combines DB companion data with breed-derived stats.
type CompanionView struct {
	db.Companion
	AC           int
	Speed        int
	LoadCapacity int
	Level        int
	Saves        engine.SaveTargets
	Attack       string
	Morale       int
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
			Name:           item.Name,
			Quantity:       item.Quantity,
			Location:       item.Location,
			WeightOverride: item.WeightOverride,
		}
	}

	scores := map[string]int{
		"str": ch.STR, "dex": ch.DEX, "con": ch.CON,
		"int": ch.INT, "wis": ch.WIS, "cha": ch.CHA,
	}
	primes := []string{"str"} // Knight prime is STR

	equippedSlots := engine.TotalEquippedSlots(engineItems)
	stowedSlots := engine.TotalStowedSlots(engineItems)
	horseSlots := engine.TotalHorseSlots(engineItems)

	pursePurse := engine.CoinPurse{CP: ch.PurseCP, SP: ch.PurseSP, EP: ch.PurseEP, GP: ch.PurseGP, PP: ch.PursePP}
	foundPurse := engine.CoinPurse{CP: ch.FoundCP, SP: ch.FoundSP, EP: ch.FoundEP, GP: ch.FoundGP, PP: ch.FoundPP}

	totalCoins := engine.TotalCoins(pursePurse) + engine.TotalCoins(foundPurse)
	coinSlots := engine.CoinSlots(totalCoins)
	totalStowed := stowedSlots + coinSlots

	stowedCap, stowedContainers := engine.StowedCapacity(engineItems)

	// Build companion views with breed-derived stats
	compViews := make([]CompanionView, len(companions))
	horseCap := 0
	for i, comp := range companions {
		cv := CompanionView{Companion: comp}
		if stats, ok := engine.BreedStats(comp.Breed); ok {
			cv.AC = engine.CompanionAC(stats.AC, comp.HasBarding)
			cv.Speed = stats.Speed
			cv.LoadCapacity = stats.LoadCapacity
			cv.Level = stats.Level
			cv.Saves = stats.Saves
			cv.Attack = stats.Attack
			cv.Morale = stats.Morale
		}
		horseCap += cv.LoadCapacity
		compViews[i] = cv
	}

	ac, armorName := engine.ACFromEquippedItems(engineItems, ch.DEX)
	xpMod := engine.HumanTotalXPModifier(scores, primes)
	newLevel, canLevelUp := engine.DetectLevelUp(ch.Level, ch.TotalXP)

	return &CharacterView{
		Character:        ch,
		Items:            items,
		Companions:       compViews,
		Transactions:     transactions,
		XPLog:            xpLog,
		Notes:            notes,
		AC:               ac,
		ArmorName:        armorName,
		AttackBonus:      engine.KnightAttackBonus(ch.Level),
		Weapons:          engine.EquippedWeapons(engineItems),
		Saves:            engine.KnightSaveTargets(ch.Level),
		Traits:           engine.KnightTraits(ch.Level),
		Speed:            engine.SpeedFromSlots(equippedSlots, totalStowed),
		EquippedSlots:    equippedSlots,
		StowedSlots:      stowedSlots,
		CoinSlotsCount:   coinSlots,
		TotalStowedSlots: totalStowed,
		HorseSlots:       horseSlots,
		StowedCapacity:   stowedCap,
		StowedContainers: stowedContainers,
		HorseCapacity:    horseCap,
		XPModPercent:     xpMod,
		XPToNext:         engine.XPToNextLevel(ch.Level, ch.TotalXP),
		NewLevel:         newLevel,
		CanLevelUp:       canLevelUp,
		PurseGPValue:     engine.CoinPurseGPValue(pursePurse),
		FoundGPValue:     engine.CoinPurseGPValue(foundPurse),
		TotalPurseCoins:  engine.TotalCoins(pursePurse),
		TotalFoundCoins:  engine.TotalCoins(foundPurse),
		BreedNames:       engine.BreedNames(),
	}, nil
}

// itemSlots returns the number of gear slots occupied by an item.
func itemSlots(item db.Item) int {
	return engine.ItemSlots(engine.Item{
		Name:           item.Name,
		Quantity:       item.Quantity,
		Location:       item.Location,
		WeightOverride: item.WeightOverride,
	})
}
