package server

import (
	"fmt"
	"strings"

	"monks.co/apps/dolmenwood/db"
	"monks.co/apps/dolmenwood/engine"
)

// CharacterView combines DB data with engine computations for rendering.
type CharacterView struct {
	Character    *db.Character
	Items        []db.Item
	Companions   []CompanionView
	Transactions []db.Transaction
	XPLog        []db.XPLogEntry
	Notes        []db.Note

	// Computed fields
	AC               int
	ArmorName        string
	AttackBonus      int
	Weapons          []engine.EquippedWeapon
	MagicResistance int
	Saves            engine.SaveTargets
	Traits           engine.Traits
	Speed            int
	EquippedSlots    int
	StowedSlots      int
	CoinSlotsCount   int
	TotalStowedSlots int
	StowedCapacity   int
	StowedContainers []engine.ContainerInfo
	XPModPercent     int
	KindredTraits    []engine.Trait
	ClassTraits      []engine.Trait
	XPToNext         int
	NewLevel         int
	CanLevelUp       bool
	PurseGPValue     int
	FoundGPValue     int
	TotalPurseCoins  int
	TotalFoundCoins  int
	BreedNames       []string

	// Inventory tree
	EquippedItems   []InventoryItem
	CompanionGroups []CompanionInventory
	MoveTargets     []MoveTarget
}

// InventoryItem is a db.Item with computed slots and nested children.
type InventoryItem struct {
	db.Item
	Slots      int
	BundleSize int
	Children   []InventoryItem
	Capacity   int // container capacity (0 if not a container)
	UsedSlots  int // sum of children's slots
}

// CompanionInventory groups items under a companion.
type CompanionInventory struct {
	Companion CompanionView
	Items     []InventoryItem
	UsedSlots int
}

// MoveTarget represents a destination for moving an item.
type MoveTarget struct {
	Label string // "Equipped", "Backpack", "Bessie (Mule)"
	Value string // "equipped", "container:42", "companion:7"
}

// SpeedChartCell represents one slot in the speed bracket chart.
type SpeedChartCell struct {
	Speed  int  // 40, 30, 20, or 10
	Filled bool // whether this slot is occupied
}

// EquippedSpeedChart returns a 10-cell chart for equipped slot speed brackets.
func EquippedSpeedChart(slots int) []SpeedChartCell {
	// Equipped: 0-3 → 40, 4-5 → 30, 6-7 → 20, 8-10 → 10
	brackets := []struct{ speed, count int }{
		{40, 3}, {30, 2}, {20, 2}, {10, 3},
	}
	cells := make([]SpeedChartCell, 0, 10)
	for _, b := range brackets {
		for j := 0; j < b.count; j++ {
			cells = append(cells, SpeedChartCell{Speed: b.speed, Filled: len(cells) < slots})
		}
	}
	return cells
}

// StowedSpeedChart returns a 16-cell chart for stowed slot speed brackets.
func StowedSpeedChart(slots int) []SpeedChartCell {
	// Stowed: 0-10 → 40, 11-12 → 30, 13-14 → 20, 15-16 → 10
	brackets := []struct{ speed, count int }{
		{40, 10}, {30, 2}, {20, 2}, {10, 2},
	}
	cells := make([]SpeedChartCell, 0, 16)
	for _, b := range brackets {
		for j := 0; j < b.count; j++ {
			cells = append(cells, SpeedChartCell{Speed: b.speed, Filled: len(cells) < slots})
		}
	}
	return cells
}

// SpeedCellClass returns the CSS class for a speed chart cell.
func SpeedCellClass(cell SpeedChartCell) string {
	if cell.Filled {
		switch cell.Speed {
		case 40:
			return "bg-green-500"
		case 30:
			return "bg-amber-400"
		case 20:
			return "bg-orange-400"
		default:
			return "bg-red-500"
		}
	}
	switch cell.Speed {
	case 40:
		return "bg-green-100"
	case 30:
		return "bg-amber-100"
	case 20:
		return "bg-orange-100"
	default:
		return "bg-red-100"
	}
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
		engineItems[i] = dbItemToEngine(item)
	}

	scores := map[string]int{
		"str": ch.STR, "dex": ch.DEX, "con": ch.CON,
		"int": ch.INT, "wis": ch.WIS, "cha": ch.CHA,
	}
	primes := []string{"str"} // Knight prime is STR

	// Use hierarchy-based encumbrance
	equipped, stowed, companionSlots := engine.CalculateEncumbrance(engineItems)

	pursePurse := engine.CoinPurse{CP: ch.PurseCP, SP: ch.PurseSP, EP: ch.PurseEP, GP: ch.PurseGP, PP: ch.PursePP}
	foundPurse := engine.CoinPurse{CP: ch.FoundCP, SP: ch.FoundSP, EP: ch.FoundEP, GP: ch.FoundGP, PP: ch.FoundPP}

	totalCoins := engine.TotalCoins(pursePurse) + engine.TotalCoins(foundPurse)
	coinSlots := engine.CoinSlots(totalCoins)
	totalStowed := stowed + coinSlots

	stowedCap, stowedContainers := engine.StowedCapacity(engineItems)

	// Build companion views with breed-derived stats
	compViews := make([]CompanionView, len(companions))
	for i, comp := range companions {
		cv := CompanionView{Companion: comp}
		if stats, ok := engine.BreedStats(comp.Breed); ok {
			cv.AC = engine.CompanionAC(stats.AC, comp.HasBarding)
			cv.Speed = stats.Speed
			cv.LoadCapacity = engine.CompanionLoadCapacity(stats.LoadCapacity, comp.SaddleType)
			cv.Level = stats.Level
			cv.Saves = stats.Saves
			cv.Attack = stats.Attack
			cv.Morale = stats.Morale
		}
		compViews[i] = cv
	}

	ac, armorName := engine.CharacterAC(ch.Kindred, engineItems, ch.DEX)
	xpMod := engine.TotalXPModifier(ch.Kindred, scores, primes)
	newLevel, canLevelUp := engine.DetectLevelUp(ch.Level, ch.TotalXP)

	// Build inventory tree
	equippedTree, compGroups := buildInventoryTree(items, compViews, companionSlots)
	moveTargets := buildMoveTargets(items, compViews)

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
		MagicResistance: engine.MagicResistance(ch.Kindred, ch.WIS),
		Saves:            engine.KnightSaveTargets(ch.Level),
		Traits:           engine.KnightTraits(ch.Level),
		Speed:            engine.SpeedFromSlots(equipped, totalStowed),
		EquippedSlots:    equipped,
		StowedSlots:      stowed,
		CoinSlotsCount:   coinSlots,
		TotalStowedSlots: totalStowed,
		StowedCapacity:   stowedCap,
		StowedContainers: stowedContainers,
		XPModPercent:     xpMod,
		KindredTraits:    engine.KindredTraits(ch.Kindred, ch.Level),
		ClassTraits:      engine.ClassTraits(ch.Class, ch.Level),
		XPToNext:         engine.XPToNextLevel(ch.Level, ch.TotalXP),
		NewLevel:         newLevel,
		CanLevelUp:       canLevelUp,
		PurseGPValue:     engine.CoinPurseGPValue(pursePurse),
		FoundGPValue:     engine.CoinPurseGPValue(foundPurse),
		TotalPurseCoins:  engine.TotalCoins(pursePurse),
		TotalFoundCoins:  engine.TotalCoins(foundPurse),
		BreedNames:       engine.BreedNames(),
		EquippedItems:    equippedTree,
		CompanionGroups:  compGroups,
		MoveTargets:      moveTargets,
	}, nil
}

func dbItemToEngine(item db.Item) engine.Item {
	return engine.Item{
		ID:             item.ID,
		Name:           item.Name,
		Quantity:       item.Quantity,
		Location:       item.Location,
		WeightOverride: item.WeightOverride,
		ContainerID:    item.ContainerID,
		CompanionID:    item.CompanionID,
		IsTiny:         item.IsTiny,
	}
}

// itemSlots returns the number of gear slots occupied by an item.
func itemSlots(item db.Item) int {
	return engine.ItemSlots(dbItemToEngine(item))
}

// buildInventoryTree builds the hierarchical inventory from flat items.
func buildInventoryTree(items []db.Item, compViews []CompanionView, companionSlots map[uint]int) ([]InventoryItem, []CompanionInventory) {
	// Index items by ID
	byID := make(map[uint]db.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}

	// Group children by parent
	childrenOf := make(map[uint][]db.Item) // container_id -> children
	compItems := make(map[uint][]db.Item)  // companion_id -> items
	var equippedRoots []db.Item

	for _, it := range items {
		if it.ContainerID != nil {
			childrenOf[*it.ContainerID] = append(childrenOf[*it.ContainerID], it)
		} else if it.CompanionID != nil {
			compItems[*it.CompanionID] = append(compItems[*it.CompanionID], it)
		} else {
			equippedRoots = append(equippedRoots, it)
		}
	}

	// Recursive tree builder
	var buildTree func(item db.Item) InventoryItem
	buildTree = func(item db.Item) InventoryItem {
		inv := InventoryItem{
			Item:       item,
			Slots:      itemSlots(item),
			BundleSize: engine.ItemBundleSize(item.Name),
		}
		if cap, ok := engine.ContainerCapacity(item.Name); ok {
			inv.Capacity = cap
		}
		for _, child := range childrenOf[item.ID] {
			childItem := buildTree(child)
			inv.UsedSlots += childItem.Slots
			inv.Children = append(inv.Children, childItem)
		}
		return inv
	}

	// Build equipped tree
	equipped := make([]InventoryItem, 0, len(equippedRoots))
	for _, it := range equippedRoots {
		equipped = append(equipped, buildTree(it))
	}

	// Build companion groups
	compGroups := make([]CompanionInventory, 0, len(compViews))
	for _, cv := range compViews {
		group := CompanionInventory{
			Companion: cv,
			UsedSlots: companionSlots[cv.ID],
		}
		for _, it := range compItems[cv.ID] {
			group.Items = append(group.Items, buildTree(it))
		}
		compGroups = append(compGroups, group)
	}

	return equipped, compGroups
}

func itemIsTiny(item InventoryItem) bool {
	if item.IsTiny {
		return true
	}
	return engine.IsTinyItem(item.Name)
}

// buildMoveTargets builds the list of destinations an item can be moved to.
func buildMoveTargets(items []db.Item, compViews []CompanionView) []MoveTarget {
	targets := []MoveTarget{
		{Label: "Equipped", Value: "equipped"},
	}

	// Index items by ID for parent lookups
	byID := make(map[uint]db.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}

	// Build companion name lookup
	compName := make(map[uint]string, len(compViews))
	for _, cv := range compViews {
		compName[cv.ID] = cv.Name
	}

	// Add all containers at any nesting depth
	for _, it := range items {
		if !engine.IsContainer(it.Name) {
			continue
		}
		label := it.Name
		if it.Quantity > 1 {
			label = fmt.Sprintf("%s (%d)", it.Name, it.Quantity)
		}
		// If directly on a companion, note which one
		if it.CompanionID != nil {
			if name, ok := compName[*it.CompanionID]; ok {
				label = fmt.Sprintf("%s (%s)", it.Name, name)
			}
		}
		targets = append(targets, MoveTarget{
			Label: label,
			Value: fmt.Sprintf("container:%d", it.ID),
		})
	}

	// Add companions
	for _, cv := range compViews {
		label := cv.Name
		if cv.Breed != "" {
			label = fmt.Sprintf("%s (%s)", cv.Name, cv.Breed)
		}
		targets = append(targets, MoveTarget{
			Label: label,
			Value: fmt.Sprintf("companion:%d", cv.ID),
		})
	}

	return targets
}

// parseMoveTarget parses a move_to form value into container_id and companion_id.
func parseMoveTarget(value string) (containerID *uint, companionID *uint) {
	if value == "equipped" || value == "" {
		return nil, nil
	}
	if after, ok := strings.CutPrefix(value, "container:"); ok {
		id := atoui(after)
		return &id, nil
	}
	if after, ok := strings.CutPrefix(value, "companion:"); ok {
		id := atoui(after)
		return nil, &id
	}
	return nil, nil
}
