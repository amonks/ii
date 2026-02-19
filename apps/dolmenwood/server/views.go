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
	AuditLog     []db.AuditLogEntry

	// Computed fields
	AC               int
	ArmorName        string
	ShieldName       string
	AttackBonus      int
	Weapons          []engine.EquippedWeapon
	MagicResistance  int
	Saves            engine.SaveTargets
	Traits           engine.Traits
	AdvancementTable *engine.AdvancementTable
	Speed                   int
	SpeedEncounter          int
	SpeedExplorationUnknown int
	SpeedExplorationMapped  int
	SpeedRunning            int
	SpeedOverland           int
	EquippedSlots           int
	StowedSlots             int
	TotalStowedSlots        int
	StowedCapacity          int
	StowedContainers        []engine.ContainerInfo
	XPModPercent            int
	KindredTraits           []engine.Trait
	ClassTraits             []engine.Trait
	BirthdayMonths          []engine.Month
	BirthdayDays            []int
	MoonSign                *engine.MoonSign
	XPToNext                int
	NewLevel                int
	CanLevelUp              bool
	PurseCoins              map[string]int // computed: inventory coins minus found treasure
	PurseGPValue            int            // computed: inventory GP value minus found GP value
	FoundGPValue            int
	InventoryCoins          map[string]int // coin counts from inventory items, keyed by CoinType
	InventoryGPValue        int            // total GP value of all inventory coin items
	BreedNames              []string

	// Inventory tree
	EquippedItems   []InventoryItem
	CompanionGroups []CompanionInventory
	MoveTargets     []MoveTarget

	// Store
	StoreGroups []StoreGroup

	// Bank
	GameDay      int
	CalendarDate engine.CalendarDisplay
	BankDeposits []BankDepositView
	BankTotalCP  int
}

// BankDepositView wraps a bank deposit with computed maturity info.
type BankDepositView struct {
	db.BankDeposit
	IsMature        bool
	DaysUntilMature int
	GPValue         int
}

type StoreGroup struct {
	Title string
	Items []StoreItem
}

type StoreItem struct {
	ID            string
	Name          string
	CostCP        int
	CostCoinLabel string
	Weight        int
	Bulk          int
	Damage        string
	AC            int
	Qualities     string
	Load          int
	Cargo         int
	Capacity      int
}

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
	auditLog, err := d.ListAuditLog(ch.ID)
	if err != nil {
		return nil, err
	}
	bankDeposits, err := d.ListBankDeposits(ch.ID)
	if err != nil {
		return nil, err
	}

	calendarDate, err := engine.CalendarDisplayForGameDay(ch.CalendarStartDay, ch.CurrentDay)
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

	foundPurse := engine.CoinPurse{CP: ch.FoundCP, SP: ch.FoundSP, EP: ch.FoundEP, GP: ch.FoundGP, PP: ch.FoundPP}

	totalStowed := stowed

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

	// Aggregate coin items from inventory by parsing notes
	inventoryCoins := make(map[string]int)
	for _, item := range items {
		if strings.EqualFold(item.Name, engine.CoinItemNameStr) {
			parsed := engine.ParseCoinNotes(item.Notes)
			for ct, qty := range parsed {
				inventoryCoins[ct] += qty
			}
		}
	}
	inventoryCoinPurse := engine.CoinPurse{
		CP: inventoryCoins[engine.CP],
		SP: inventoryCoins[engine.SP],
		EP: inventoryCoins[engine.EP],
		GP: inventoryCoins[engine.GP],
		PP: inventoryCoins[engine.PP],
	}
	inventoryGPValue := engine.CoinPurseGPValue(inventoryCoinPurse)
	foundGPValue := engine.CoinPurseGPValue(foundPurse)

	// Purse = inventory coins minus found treasure
	purseCoins := map[string]int{
		engine.CP: inventoryCoins[engine.CP] - ch.FoundCP,
		engine.SP: inventoryCoins[engine.SP] - ch.FoundSP,
		engine.EP: inventoryCoins[engine.EP] - ch.FoundEP,
		engine.GP: inventoryCoins[engine.GP] - ch.FoundGP,
		engine.PP: inventoryCoins[engine.PP] - ch.FoundPP,
	}

	birthdayDays := make([]int, 0, 31)
	for day := 1; day <= 31; day++ {
		birthdayDays = append(birthdayDays, day)
	}
	if maxDay, ok := engine.DaysInMonth(ch.BirthdayMonth); ok {
		birthdayDays = birthdayDays[:0]
		for day := 1; day <= maxDay; day++ {
			birthdayDays = append(birthdayDays, day)
		}
	}
	var moonSign *engine.MoonSign
	if sign, ok := engine.MoonSignFromBirthday(ch.BirthdayMonth, ch.BirthdayDay); ok {
		moonSign = &sign
	}

	ac, armorName := engine.CharacterAC(ch.Kindred, engineItems, ch.DEX)
	shieldName := ""
	if _, hasShield := engine.ArmorContributors(engineItems); hasShield {
		shieldName = "Shield"
	}
	xpMod := engine.TotalXPModifier(ch.Kindred, scores, primes)
	newLevel, canLevelUp := engine.DetectLevelUp(ch.Level, ch.TotalXP)
	var advancementTable *engine.AdvancementTable
	if table, ok := engine.AdvancementTableForClass(ch.Class); ok {
		advancementTable = &table
	}

	// Build bank deposit views
	var bankDepositViews []BankDepositView
	bankTotalCP := 0
	for _, dep := range bankDeposits {
		daysUntil := max(30-(ch.CurrentDay-dep.DepositDay), 0)
		bankDepositViews = append(bankDepositViews, BankDepositView{
			BankDeposit:     dep,
			IsMature:        engine.IsMature(dep.DepositDay, ch.CurrentDay),
			DaysUntilMature: daysUntil,
			GPValue:         dep.CPValue / 100,
		})
		bankTotalCP += dep.CPValue
	}

	// Build inventory tree
	equippedTree, compGroups := buildInventoryTree(items, compViews, companionSlots)
	moveTargets := buildMoveTargets(items, compViews)
	storeGroups := buildStoreGroups()

	// Compute speed breakdowns
	speed := engine.SpeedFromSlots(equipped, totalStowed)
	speedEncounter := speed
	speedExplorationUnknown := speed * 3
	speedExplorationMapped := speed * 10
	speedRunning := speed * 3
	speedOverland := speed / 5

	return &CharacterView{
		Character:        ch,
		Items:            items,
		Companions:       compViews,
		Transactions:     transactions,
		XPLog:            xpLog,
		Notes:            notes,
		AuditLog:         auditLog,
		AC:               ac,
		ArmorName:        armorName,
		ShieldName:       shieldName,
		AttackBonus:      engine.KnightAttackBonus(ch.Level),
		Weapons:          engine.EquippedWeapons(engineItems),
		MagicResistance:  engine.MagicResistance(ch.Kindred, ch.WIS),
		Saves:            engine.KnightSaveTargets(ch.Level),
		Traits:           engine.KnightTraits(ch.Level),
		AdvancementTable: advancementTable,
		Speed:                   speed,
		SpeedEncounter:          speedEncounter,
		SpeedExplorationUnknown: speedExplorationUnknown,
		SpeedExplorationMapped:  speedExplorationMapped,
		SpeedRunning:            speedRunning,
		SpeedOverland:           speedOverland,
		EquippedSlots:           equipped,
		StowedSlots:             stowed,
		TotalStowedSlots:        totalStowed,
		StowedCapacity:          stowedCap,
		StowedContainers:        stowedContainers,
		XPModPercent:            xpMod,
		KindredTraits:           engine.KindredTraits(ch.Kindred, ch.Level),
		ClassTraits:             engine.ClassTraits(ch.Class, ch.Level),
		BirthdayMonths:          engine.Months(),
		BirthdayDays:            birthdayDays,
		MoonSign:                moonSign,
		XPToNext:                engine.XPToNextLevel(ch.Level, ch.TotalXP),
		NewLevel:                newLevel,
		CanLevelUp:              canLevelUp,
		PurseCoins:              purseCoins,
		PurseGPValue:            inventoryGPValue - foundGPValue,
		FoundGPValue:            foundGPValue,
		InventoryCoins:          inventoryCoins,
		InventoryGPValue:        inventoryGPValue,
		BreedNames:              engine.BreedNames(),
		EquippedItems:           equippedTree,
		CompanionGroups:         compGroups,
		MoveTargets:             moveTargets,
		StoreGroups:             storeGroups,
		GameDay:                 ch.CurrentDay,
		CalendarDate:            calendarDate,
		BankDeposits:            bankDepositViews,
		BankTotalCP:             bankTotalCP,
	}, nil
}

func buildStoreGroups() []StoreGroup {
	groups := []StoreGroup{
		{
			Title: "Adventuring Gear",
			Items: []StoreItem{
				storeItem("Backpack", 400, "4gp"),
				storeItem("Barrel", 100, "1gp"),
				storeItem("Belt pouch", 100, "1gp"),
				storeItem("Bucket", 100, "1gp"),
				storeItem("Casket (iron, large)", 3000, "30gp"),
				storeItem("Casket (iron, small)", 1000, "10gp"),
				storeItem("Chest (wooden, large)", 500, "5gp"),
				storeItem("Chest (wooden, small)", 100, "1gp"),
				storeItem("Sack", 100, "1gp"),
				storeItem("Scroll case", 100, "1gp"),
				storeItem("Vial", 100, "1gp"),
				storeItem("Waterskin", 100, "1gp"),
				storeItem("Candles", 100, "1gp"),
				storeItem("Lantern (hooded)", 500, "5gp"),
				storeItem("Lantern (bullseye)", 1000, "10gp"),
				storeItem("Oil", 100, "1gp"),
				storeItem("Tinder box", 300, "3gp"),
				storeItem("Bedroll", 200, "2gp"),
				storeItem("Cooking pots", 300, "3gp"),
				storeItem("Firewood", 100, "1gp"),
				storeItem("Fishing rod and tackle", 400, "4gp"),
				storeItem("Rations (preserved)", 200, "2gp"),
				storeItem("Rations (fresh)", 100, "1gp"),
				storeItem("Tent", 2000, "20gp"),
				storeItem("Holy symbol (gold)", 10000, "100gp"),
				storeItem("Holy symbol (silver)", 2500, "25gp"),
				storeItem("Holy symbol (wooden)", 500, "5gp"),
				storeItem("Holy water", 2500, "25gp"),
				storeItem("Bell", 100, "1gp"),
				storeItem("Block and tackle", 500, "5gp"),
				storeItem("Caltrops", 100, "1gp"),
				storeItem("Chain", 3000, "30gp"),
				storeItem("Chalk", 100, "1gp"),
				storeItem("Chisel", 200, "2gp"),
				storeItem("Crowbar", 1000, "10gp"),
				storeItem("Grappling hook", 2000, "20gp"),
				storeItem("Hammer (small)", 200, "2gp"),
				storeItem("Hammer (sledgehammer)", 500, "5gp"),
				storeItem("Ink", 100, "1gp"),
				storeItem("Iron spikes", 100, "1gp"),
				storeItem("Lock", 2000, "20gp"),
				storeItem("Magnifying glass", 300, "3gp"),
				storeItem("Manacles", 1500, "15gp"),
				storeItem("Marbles", 100, "1gp"),
				storeItem("Mining pick", 300, "3gp"),
				storeItem("Mirror (small)", 500, "5gp"),
				storeItem("Musical instrument (stringed)", 2000, "20gp"),
				storeItem("Musical instrument (wind)", 500, "5gp"),
				storeItem("Paper", 50, "50cp"),
				storeItem("Parchment", 50, "50cp"),
				storeItem("Pole", 100, "1gp"),
				storeItem("Quill", 100, "1gp"),
				storeItem("Rope", 100, "1gp"),
				storeItem("Rope ladder", 500, "5gp"),
				storeItem("Saw", 100, "1gp"),
				storeItem("Shovel", 200, "2gp"),
				storeItem("Spell book", 10000, "100gp"),
				storeItem("Thieves' tools", 2500, "25gp"),
				storeItem("Twine", 100, "1gp"),
				storeItem("Whistle", 100, "1gp"),
				storeItem("Clothes, common", 100, "1gp"),
				storeItem("Clothes, extravagant", 10000, "100gp"),
				storeItem("Clothes, fine", 2000, "20gp"),
				storeItem("Habit, friar's", 200, "2gp"),
				storeItem("Robes, ritual", 1000, "10gp"),
				storeItem("Winter cloak", 200, "2gp"),
			},
		},
		{
			Title: "Weapons",
			Items: []StoreItem{
				storeItem("Battle axe", 700, "7gp"),
				storeItem("Club", 300, "3gp"),
				storeItem("Crossbow", 3000, "30gp"),
				storeItem("Dagger", 300, "3gp"),
				storeItem("Hand axe", 400, "4gp"),
				storeItem("Lance", 500, "5gp"),
				storeItem("Longbow", 4000, "40gp"),
				storeItem("Longsword", 1000, "10gp"),
				storeItem("Mace", 500, "5gp"),
				storeItem("Polearm", 700, "7gp"),
				storeItem("Shortbow", 2500, "25gp"),
				storeItem("Shortsword", 700, "7gp"),
				storeItem("Sling", 200, "2gp"),
				storeItem("Spear", 300, "3gp"),
				storeItem("Staff", 200, "2gp"),
				storeItem("Torches", 100, "1gp"),
				storeItem("Two-handed sword", 1500, "15gp"),
				storeItem("War hammer", 500, "5gp"),
			},
		},
		{
			Title: "Ammunition",
			Items: []StoreItem{
				storeItem("Arrows", 500, "5gp"),
				storeItem("Quarrels", 1000, "10gp"),
				storeItem("Sling stones", 0, ""),
			},
		},
		{
			Title: "Armour",
			Items: []StoreItem{
				storeItem("Leather", 2000, "20gp"),
				storeItem("Bark", 3000, "30gp"),
				storeItem("Chainmail", 4000, "40gp"),
				storeItem("Pinecone", 5000, "50gp"),
				storeItem("Plate mail", 6000, "60gp"),
				storeItem("Full plate", 100000, "1000gp"),
				storeItem("Shield", 1000, "10gp"),
			},
		},
		{
			Title: "Horses and Vehicles",
			Items: []StoreItem{
				storeItem("Charger", 25000, "250gp"),
				storeItem("Dapple-doff", 4000, "40gp"),
				storeItem("Hop-clopper", 8000, "80gp"),
				storeItem("Mule", 3000, "30gp"),
				storeItem("Prigwort prancer", 7500, "75gp"),
				storeItem("Yellow-flank", 25000, "250gp"),
				storeItem("Feed", 5, "5cp"),
				storeItem("Horse barding", 15000, "150gp"),
				storeItem("Pack saddle and bridle", 1000, "10gp"),
				storeItem("Riding saddle and bridle", 2500, "25gp"),
				storeItem("Riding saddle bags", 500, "5gp"),
				storeItem("Cart", 10000, "100gp"),
				storeItem("Wagon", 20000, "200gp"),
				storeItem("Barge", 50000, "500gp"),
				storeItem("Canoe", 3000, "30gp"),
				storeItem("Fishing boat", 35000, "350gp"),
				storeItem("Raft", 100, "1gp"),
				storeItem("Rowing boat", 2500, "25gp"),
			},
		},
	}

	for gi := range groups {
		for ii := range groups[gi].Items {
			item := &groups[gi].Items[ii]
			item.ID = storeItemID(item.Name, item.CostCP)
			item.CostCoinLabel = itemCostLabel(item.CostCP, item.CostCoinLabel)
			item.Weight = storeItemWeight(item.Name, item.CostCP)
			item.Bulk = storeItemSlots(item.Name, item.CostCP)
			item.Damage = itemDamage(item.Name)
			item.AC = itemArmorClass(item.Name)
			item.Qualities = itemQualities(item.Name)
			item.Load = storeItemLoad(item.Name)
			item.Cargo = storeItemCargo(item.Name)
			item.Capacity = storeItemCapacity(item.Name)
		}
	}

	return groups
}

var storeHorseLoads = map[string]int{
	"charger":          4000,
	"dapple-doff":      5000,
	"hop-clopper":      5000,
	"mule":             2500,
	"prigwort prancer": 3000,
	"yellow-flank":     3500,
}

var storeVehicleCargo = map[string]int{
	"cart":         10000,
	"wagon":        20000,
	"barge":        160000,
	"canoe":        5000,
	"fishing boat": 25000,
	"raft":         500,
	"rowing boat":  5000,
}

func storeItem(name string, costCP int, label string) StoreItem {
	return StoreItem{Name: name, CostCP: costCP, CostCoinLabel: label}
}

func storeItemID(name string, costCP int) string {
	return fmt.Sprintf("%s|%d", name, costCP)
}

func itemCostLabel(costCP int, label string) string {
	if label != "" {
		return label
	}
	if costCP == 0 {
		return "Free"
	}
	if costCP%100 == 0 {
		return fmt.Sprintf("%dgp", costCP/100)
	}
	return fmt.Sprintf("%dcp", costCP)
}

func storeItemWeight(name string, costCP int) int {
	if w, ok := engine.ItemWeight(name); ok {
		if bundle := storeBundleSize(costCP, name); bundle > 0 {
			return w * bundle
		}
		return w
	}
	return 0
}

func storeItemSlots(name string, costCP int) int {
	if bundle := storeBundleSize(costCP, name); bundle > 0 {
		return engine.ItemSlots(engine.Item{Name: name, Quantity: bundle, Location: "stowed"})
	}
	if bulk, ok := engine.ItemSlotCostExplicit(name); ok {
		return bulk
	}
	return 0
}

func storeBundleSize(costCP int, name string) int {
	bundle := engine.ItemBundleSize(name)
	if bundle == 0 {
		return 0
	}
	if expected := storeBundleCostCP(name); expected != 0 && expected != costCP {
		return 0
	}
	return bundle
}

func storeBundleCostCP(name string) int {
	switch strings.ToLower(name) {
	case "torches":
		return 100
	case "arrows":
		return 500
	default:
		return 0
	}
}

func itemDamage(name string) string {
	if weapon, ok := engine.WeaponStats(storeWeaponName(name)); ok {
		return weapon.Damage
	}
	return ""
}

func itemQualities(name string) string {
	if weapon, ok := engine.WeaponStats(storeWeaponName(name)); ok {
		return weapon.Qualities
	}
	return ""
}

func storeWeaponName(name string) string {
	switch strings.ToLower(name) {
	case "holy water":
		return "holy water vial"
	case "oil":
		return "oil flask (burning)"
	case "torches":
		return "torch (flaming)"
	case "crossbow":
		return "crossbow (ranged)"
	case "longbow":
		return "longbow (ranged)"
	case "shortbow":
		return "shortbow (ranged)"
	case "sling":
		return "sling (ranged)"
	case "spear":
		return "spear (ranged)"
	default:
		return name
	}
}

func itemArmorClass(name string) int {
	itemArmorClassOverrides := map[string]int{
		"horse barding": 2,
		"shield":        1,
	}
	if ac, ok := itemArmorClassOverrides[strings.ToLower(name)]; ok {
		return ac
	}
	if armor, ok := engine.ArmorStats(name); ok {
		return armor.AC
	}
	return 0
}

func storeItemLoad(name string) int {
	if load, ok := storeHorseLoads[strings.ToLower(name)]; ok {
		return load
	}
	return 0
}

func storeItemCargo(name string) int {
	if cargo, ok := storeVehicleCargo[strings.ToLower(name)]; ok {
		return cargo
	}
	return 0
}

func storeItemCapacity(name string) int {
	if capacity, ok := engine.ContainerCapacity(name); ok {
		return capacity
	}
	return 0
}

func filterAuditLog(entries []db.AuditLogEntry, actions ...string) []db.AuditLogEntry {
	set := make(map[string]bool, len(actions))
	for _, a := range actions {
		set[a] = true
	}
	var out []db.AuditLogEntry
	for _, e := range entries {
		if set[e.Action] {
			out = append(out, e)
		}
	}
	return out
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
// Returns equipped items and companion groups.
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
			inv.UsedSlots += childItem.Slots + childItem.UsedSlots
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

	// Add bank as a move target for coin deposits
	targets = append(targets, MoveTarget{Label: "Bank", Value: "bank"})

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
