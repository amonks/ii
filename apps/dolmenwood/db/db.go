package db

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"

	"monks.co/apps/dolmenwood/engine"
	"monks.co/pkg/database"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*database.DB
}

// Character represents a player character.
type Character struct {
	ID         uint   `gorm:"primarykey"`
	Name       string `gorm:"column:name"`
	Class      string `gorm:"column:class"`
	Kindred    string `gorm:"column:kindred"`
	Level      int    `gorm:"column:level"`
	STR        int    `gorm:"column:str"`
	DEX        int    `gorm:"column:dex"`
	CON        int    `gorm:"column:con"`
	INT        int    `gorm:"column:int_"`
	WIS        int    `gorm:"column:wis"`
	CHA        int    `gorm:"column:cha"`
	HPCurrent  int    `gorm:"column:hp_current"`
	HPMax      int    `gorm:"column:hp_max"`
	Alignment  string `gorm:"column:alignment"`
	Background string `gorm:"column:background"`
	Liege      string `gorm:"column:liege"`

	// Found treasure staging (per denomination)
	FoundCP int `gorm:"column:found_cp"`
	FoundSP int `gorm:"column:found_sp"`
	FoundEP int `gorm:"column:found_ep"`
	FoundGP int `gorm:"column:found_gp"`
	FoundPP int `gorm:"column:found_pp"`

	// Spendable purse
	PurseCP int `gorm:"column:purse_cp"`
	PurseSP int `gorm:"column:purse_sp"`
	PurseEP int `gorm:"column:purse_ep"`
	PurseGP int `gorm:"column:purse_gp"`
	PursePP int `gorm:"column:purse_pp"`

	// Coin location (where the virtual "Coins" item lives)
	CoinCompanionID *uint `gorm:"column:coin_companion_id"`
	CoinContainerID *uint `gorm:"column:coin_container_id"`

	// XP
	TotalXP int `gorm:"column:total_xp"`

	// Game day counter (per character)
	CurrentDay int `gorm:"column:current_day"`

	// Calendar start day-of-year (day-of-year that corresponds to game day 1)
	CalendarStartDay int `gorm:"column:calendar_start_day"`

	// Birthday
	BirthdayMonth string `gorm:"column:birthday_month"`
	BirthdayDay   int    `gorm:"column:birthday_day"`

	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

type Item struct {
	ID             uint   `gorm:"primarykey"`
	CharacterID    uint   `gorm:"column:character_id"`
	Name           string `gorm:"column:name"`
	WeightOverride *int   `gorm:"column:weight_override"`
	Quantity       int    `gorm:"column:quantity"`
	Location       string `gorm:"column:location"`
	Notes          string `gorm:"column:notes"`
	SortOrder      int    `gorm:"column:sort_order"`
	ContainerID    *uint  `gorm:"column:container_id"`
	CompanionID    *uint  `gorm:"column:companion_id"`
	IsTiny         bool   `gorm:"column:is_tiny"`
}

type Companion struct {
	ID           uint   `gorm:"primarykey"`
	CharacterID  uint   `gorm:"column:character_id"`
	Name         string `gorm:"column:name"`
	Breed        string `gorm:"column:breed"`
	HPCurrent    int    `gorm:"column:hp_current"`
	HPMax        int    `gorm:"column:hp_max"`
	AC           int    `gorm:"column:ac"`
	Speed        int    `gorm:"column:speed"`
	LoadCapacity int    `gorm:"column:load_capacity"`
	Level        int    `gorm:"column:level"`
	Attack       string `gorm:"column:attack"`
	Morale       int    `gorm:"column:morale"`
	HasBarding   bool   `gorm:"column:has_barding"`
	SaddleType   string `gorm:"column:saddle_type"` // "", "riding", "pack"
	Loyalty      int    `gorm:"column:loyalty"`     // retainer loyalty score (7 + CHA mod)
}

type RetainerContract struct {
	ID           uint      `gorm:"primarykey"`
	EmployerID   uint      `gorm:"column:employer_id;index"`
	RetainerID   uint      `gorm:"column:retainer_id;index"`
	LootSharePct float64   `gorm:"column:loot_share_pct;default:15.0"`
	XPSharePct   float64   `gorm:"column:xp_share_pct;default:50.0"`
	DailyWageCP  int       `gorm:"column:daily_wage_cp;default:0"`
	HiredOnDay   int       `gorm:"column:hired_on_day;default:1"`
	Active       bool      `gorm:"column:active;default:true"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

type Transaction struct {
	ID              uint      `gorm:"primarykey"`
	CharacterID     uint      `gorm:"column:character_id"`
	Amount          int       `gorm:"column:amount"`
	CoinType        string    `gorm:"column:coin_type"`
	Description     string    `gorm:"column:description"`
	IsFoundTreasure bool      `gorm:"column:is_found_treasure"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (XPLogEntry) TableName() string { return "xp_log" }

type XPLogEntry struct {
	ID          uint      `gorm:"primarykey"`
	CharacterID uint      `gorm:"column:character_id"`
	Amount      int       `gorm:"column:amount"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type Note struct {
	ID          uint      `gorm:"primarykey"`
	CharacterID uint      `gorm:"column:character_id"`
	Content     string    `gorm:"column:content"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type AuditLogEntry struct {
	ID          uint      `gorm:"primarykey"`
	CharacterID uint      `gorm:"column:character_id"`
	Action      string    `gorm:"column:action"`
	Detail      string    `gorm:"column:detail"`
	GameDay     int       `gorm:"column:game_day"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type BankDeposit struct {
	ID          uint      `gorm:"primarykey"`
	CharacterID uint      `gorm:"column:character_id"`
	CoinNotes   string    `gorm:"column:coin_notes"`
	CPValue     int       `gorm:"column:cp_value"`
	DepositDay  int       `gorm:"column:deposit_day"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (AuditLogEntry) TableName() string { return "audit_log" }

func New() (*DB, error) {
	d, err := database.OpenFromDataFolder("dolmenwood")
	if err != nil {
		return nil, err
	}
	if err := d.MigrateFS(context.Background(), migrationsFS, "migrations", "001_baseline.sql"); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{d}, nil
}

func NewMemory() (*DB, error) {
	d, err := database.Open(":memory:")
	if err != nil {
		return nil, err
	}
	if err := d.MigrateFS(context.Background(), migrationsFS, "migrations"); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{d}, nil
}

// --- Character CRUD ---

func (db *DB) CreateCharacter(ch *Character) error {
	ch.CreatedAt = time.Now()
	ch.UpdatedAt = ch.CreatedAt
	if ch.CurrentDay == 0 {
		ch.CurrentDay = 1
	}
	if ch.CalendarStartDay == 0 {
		ch.CalendarStartDay = 1
	}
	if err := db.Create(ch).Error; err != nil {
		return err
	}
	return db.ensureEnchantmentUseCapacity(ch.ID)
}

func (db *DB) GetCharacter(id uint) (*Character, error) {
	var ch Character
	if err := db.First(&ch, id).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (db *DB) ListCharacters() ([]Character, error) {
	var chars []Character
	if err := db.Order("created_at desc").Find(&chars).Error; err != nil {
		return nil, err
	}
	return chars, nil
}

func (db *DB) UpdateCharacter(ch *Character) error {
	ch.UpdatedAt = time.Now()
	return db.Save(ch).Error
}

func (db *DB) DeleteCharacter(id uint) error {
	db.Where("character_id = ?", id).Delete(&Item{})
	db.Where("character_id = ?", id).Delete(&Companion{})
	db.Where("character_id = ?", id).Delete(&Transaction{})
	db.Where("character_id = ?", id).Delete(&XPLogEntry{})
	db.Where("character_id = ?", id).Delete(&Note{})
	db.Where("character_id = ?", id).Delete(&AuditLogEntry{})
	db.Where("character_id = ?", id).Delete(&EnchantmentUse{})
	db.Where("character_id = ?", id).Delete(&PreparedSpell{})
	db.Where("character_id = ?", id).Delete(&BankDeposit{})
	db.Where("employer_id = ?", id).Delete(&RetainerContract{})
	db.Where("retainer_id = ?", id).Delete(&RetainerContract{})
	return db.Delete(&Character{}, id).Error
}

// --- Item CRUD ---

func (db *DB) CreateItem(item *Item) error {
	return db.Create(item).Error
}

func (db *DB) ListItems(characterID uint) ([]Item, error) {
	var items []Item
	if err := db.Where("character_id = ?", characterID).Order("sort_order, id").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (db *DB) UpdateItem(item *Item) error {
	return db.Save(item).Error
}

func (db *DB) GetItem(id uint) (*Item, error) {
	var item Item
	if err := db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (db *DB) DeleteItem(id uint) error {
	// Cascade: move children to deleted item's parent
	item, err := db.GetItem(id)
	if err != nil {
		return db.Delete(&Item{}, id).Error
	}
	// Reparent children to the deleted item's parent
	db.Model(&Item{}).Where("container_id = ?", id).Updates(map[string]any{
		"container_id": item.ContainerID,
		"companion_id": item.CompanionID,
	})
	// Reset coin location if coins were in this container
	db.Model(&Character{}).Where("coin_container_id = ?", id).Updates(map[string]any{
		"coin_container_id": item.ContainerID,
		"coin_companion_id": item.CompanionID,
	})
	return db.Delete(&Item{}, id).Error
}

// TransferItem moves an item (or partial stack) to another character.
func (db *DB) TransferItem(itemID uint, toCharacterID uint, quantity int) error {
	if quantity < 0 {
		return fmt.Errorf("invalid quantity %d", quantity)
	}
	item, err := db.GetItem(itemID)
	if err != nil {
		return err
	}
	sourceCharacterID := item.CharacterID

	originalItem := *item
	if quantity == 0 || quantity >= item.Quantity {
		item.CharacterID = toCharacterID
		item.ContainerID = nil
		item.CompanionID = nil
		item.Location = ""
		if err := db.UpdateItem(item); err != nil {
			return err
		}
		if engine.IsContainer(item.Name) {
			if err := db.Model(&Item{}).Where("container_id = ?", item.ID).Update("character_id", toCharacterID).Error; err != nil {
				return err
			}
		}
		return db.clearCoinLocationForItemMove(sourceCharacterID, &originalItem)
	}

	if strings.EqualFold(item.Name, engine.CoinItemNameStr) {
		remainingNotes, remainingTotal, transferNotes, transferTotal, err := splitCoinNotesByQuantity(item.Notes, quantity)
		if err != nil {
			return err
		}
		item.Notes = remainingNotes
		item.Quantity = remainingTotal
		if err := db.UpdateItem(item); err != nil {
			return err
		}
		return db.CreateItem(&Item{
			CharacterID:    toCharacterID,
			Name:           item.Name,
			Notes:          transferNotes,
			Quantity:       transferTotal,
			IsTiny:         item.IsTiny,
			WeightOverride: item.WeightOverride,
		})
	}

	item.Quantity -= quantity
	if err := db.UpdateItem(item); err != nil {
		return err
	}
	return db.CreateItem(&Item{
		CharacterID:    toCharacterID,
		Name:           item.Name,
		Notes:          item.Notes,
		Quantity:       quantity,
		IsTiny:         item.IsTiny,
		WeightOverride: item.WeightOverride,
	})
}

func (db *DB) clearCoinLocationForItemMove(characterID uint, item *Item) error {
	var ch Character
	if err := db.First(&ch, characterID).Error; err != nil {
		return err
	}
	updates := map[string]any{}
	if strings.EqualFold(item.Name, engine.CoinItemNameStr) {
		if item.ContainerID != nil && ch.CoinContainerID != nil && *ch.CoinContainerID == *item.ContainerID {
			updates["coin_container_id"] = nil
		}
		if item.CompanionID != nil && ch.CoinCompanionID != nil && *ch.CoinCompanionID == *item.CompanionID {
			updates["coin_companion_id"] = nil
		}
	} else if ch.CoinContainerID != nil && *ch.CoinContainerID == item.ID {
		updates["coin_container_id"] = nil
		updates["coin_companion_id"] = nil
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&Character{}).Where("id = ?", characterID).Updates(updates).Error
}

func splitCoinNotesByQuantity(notes string, quantity int) (string, int, string, int, error) {
	coins := engine.ParseCoinNotes(notes)
	available := 0
	for _, qty := range coins {
		available += qty
	}
	if quantity > available {
		return "", 0, "", 0, fmt.Errorf("insufficient coins: have %d, want %d", available, quantity)
	}
	order := []engine.CoinType{engine.PP, engine.GP, engine.EP, engine.SP, engine.CP}
	transfer := make(map[engine.CoinType]int)
	remaining := quantity
	for _, ct := range order {
		if remaining == 0 {
			break
		}
		if coins[ct] <= 0 {
			continue
		}
		take := min(coins[ct], remaining)
		transfer[ct] = take
		coins[ct] -= take
		remaining -= take
	}
	transferNotes := engine.FormatCoinNotes(transfer)
	remainingNotes := engine.FormatCoinNotes(coins)
	remainingTotal := 0
	transferTotal := 0
	for _, qty := range coins {
		remainingTotal += qty
	}
	for _, qty := range transfer {
		transferTotal += qty
	}
	return remainingNotes, remainingTotal, transferNotes, transferTotal, nil
}

// --- Companion CRUD ---

func (db *DB) CreateCompanion(comp *Companion) error {
	return db.Create(comp).Error
}

func (db *DB) ListCompanions(characterID uint) ([]Companion, error) {
	var comps []Companion
	if err := db.Where("character_id = ?", characterID).Find(&comps).Error; err != nil {
		return nil, err
	}
	return comps, nil
}

func (db *DB) UpdateCompanion(comp *Companion) error {
	return db.Save(comp).Error
}

func (db *DB) GetCompanion(id uint) (*Companion, error) {
	var comp Companion
	if err := db.First(&comp, id).Error; err != nil {
		return nil, err
	}
	return &comp, nil
}

func (db *DB) DeleteCompanion(id uint) error {
	// Move companion's items to equipped on character (nil container, nil companion)
	db.Model(&Item{}).Where("companion_id = ?", id).Updates(map[string]any{
		"companion_id": nil,
		"container_id": nil,
	})
	// Reset coin location if coins were on this companion
	db.Model(&Character{}).Where("coin_companion_id = ?", id).Updates(map[string]any{
		"coin_companion_id": nil,
		"coin_container_id": nil,
	})
	return db.Delete(&Companion{}, id).Error
}

// --- Retainer Contract CRUD ---

func (db *DB) CreateRetainerContract(rc *RetainerContract) error {
	if rc.CreatedAt.IsZero() {
		rc.CreatedAt = time.Now()
	}
	return db.Create(rc).Error
}

func (db *DB) ListActiveRetainerContracts(employerID uint) ([]RetainerContract, error) {
	var contracts []RetainerContract
	if err := db.Where("employer_id = ? AND active = 1", employerID).Order("created_at asc, id asc").Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

func (db *DB) GetRetainerContract(id uint) (*RetainerContract, error) {
	var rc RetainerContract
	if err := db.First(&rc, id).Error; err != nil {
		return nil, err
	}
	return &rc, nil
}

func (db *DB) UpdateRetainerContract(rc *RetainerContract) error {
	return db.Save(rc).Error
}

func (db *DB) DeactivateRetainerContract(id uint) error {
	return db.Model(&RetainerContract{}).Where("id = ?", id).Update("active", false).Error
}

// --- Transaction CRUD ---

func (db *DB) CreateTransaction(tx *Transaction) error {
	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now()
	}
	return db.Create(tx).Error
}

func (db *DB) GetTransaction(id uint) (*Transaction, error) {
	var tx Transaction
	if err := db.First(&tx, id).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

func (db *DB) ListTransactions(characterID uint) ([]Transaction, error) {
	var txs []Transaction
	if err := db.Where("character_id = ?", characterID).Order("created_at desc").Find(&txs).Error; err != nil {
		return nil, err
	}
	return txs, nil
}

// --- XP Log ---

func (db *DB) CreateXPLogEntry(entry *XPLogEntry) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	return db.Create(entry).Error
}

func (db *DB) ListXPLog(characterID uint) ([]XPLogEntry, error) {
	var entries []XPLogEntry
	if err := db.Where("character_id = ?", characterID).Order("created_at desc").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// --- Notes ---

func (db *DB) CreateNote(note *Note) error {
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now()
	}
	return db.Create(note).Error
}

func (db *DB) ListNotes(characterID uint) ([]Note, error) {
	var notes []Note
	if err := db.Where("character_id = ?", characterID).Order("created_at desc").Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (db *DB) GetNote(id uint) (*Note, error) {
	var note Note
	if err := db.First(&note, id).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (db *DB) DeleteNote(id uint) error {
	return db.Delete(&Note{}, id).Error
}

// --- Audit Log ---

func (db *DB) AddAuditLog(characterID uint, action, detail string, gameDay int) error {
	entry := &AuditLogEntry{
		CharacterID: characterID,
		Action:      action,
		Detail:      detail,
		GameDay:     gameDay,
		CreatedAt:   time.Now(),
	}
	return db.Create(entry).Error
}

func (db *DB) ListAuditLog(characterID uint) ([]AuditLogEntry, error) {
	var entries []AuditLogEntry
	if err := db.Where("character_id = ?", characterID).Order("created_at desc").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// --- Bank Deposits ---

func (db *DB) CreateBankDeposit(dep *BankDeposit) error {
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = time.Now()
	}
	return db.Create(dep).Error
}

func (db *DB) ListBankDeposits(characterID uint) ([]BankDeposit, error) {
	var deps []BankDeposit
	if err := db.Where("character_id = ?", characterID).Order("deposit_day asc, id asc").Find(&deps).Error; err != nil {
		return nil, err
	}
	return deps, nil
}

func (db *DB) UpdateBankDeposit(dep *BankDeposit) error {
	return db.Save(dep).Error
}

func (db *DB) DeleteBankDeposit(id uint) error {
	return db.Delete(&BankDeposit{}, id).Error
}

// --- Prepared Spells ---

func (db *DB) ListPreparedSpells(characterID uint) ([]PreparedSpell, error) {
	var spells []PreparedSpell
	if err := db.Where("character_id = ?", characterID).Order("id asc").Find(&spells).Error; err != nil {
		return nil, err
	}
	return spells, nil
}

func (db *DB) CreatePreparedSpell(spell *PreparedSpell) error {
	if err := db.Create(spell).Error; err != nil {
		return err
	}
	return db.ensureEnchantmentUseCapacity(spell.CharacterID)
}

func (db *DB) MarkSpellUsed(spellID uint) error {
	return db.Model(&PreparedSpell{}).Where("id = ?", spellID).Update("used", true).Error
}

func (db *DB) ResetSpells(characterID uint) error {
	if err := db.Model(&PreparedSpell{}).Where("character_id = ?", characterID).Update("used", false).Error; err != nil {
		return err
	}
	return db.ResetEnchantmentUses(characterID)
}

func (db *DB) EnchantmentUsesCount(characterID uint) (int, int, error) {
	var total int64
	var used int64
	if err := db.Model(&EnchantmentUse{}).Where("character_id = ?", characterID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	if total == 0 {
		return 0, 0, nil
	}
	if err := db.Model(&EnchantmentUse{}).Where("character_id = ? AND used = ?", characterID, true).Count(&used).Error; err != nil {
		return 0, 0, err
	}
	return int(total), int(used), nil
}

func (db *DB) CreateEnchantmentUse(characterID uint) error {
	var use EnchantmentUse
	if err := db.Where("character_id = ? AND used = ?", characterID, false).Order("id asc").First(&use).Error; err != nil {
		return err
	}
	return db.Model(&EnchantmentUse{}).Where("id = ?", use.ID).Update("used", true).Error
}

func (db *DB) ResetEnchantmentUses(characterID uint) error {
	return db.Model(&EnchantmentUse{}).Where("character_id = ?", characterID).Update("used", false).Error
}

func (db *DB) EnsureEnchantmentUseCapacity(characterID uint) error {
	return db.ensureEnchantmentUseCapacity(characterID)
}

func (db *DB) ensureEnchantmentUseCapacity(characterID uint) error {
	var ch Character
	if err := db.First(&ch, characterID).Error; err != nil {
		return err
	}
	if !strings.EqualFold(ch.Class, "Bard") {
		return nil
	}
	total, _, err := db.EnchantmentUsesCount(characterID)
	if err != nil {
		return err
	}
	needed := ch.Level - total
	for range needed {
		use := &EnchantmentUse{CharacterID: characterID, Used: false, CreatedAt: time.Now()}
		if err := db.Create(use).Error; err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) DeletePreparedSpell(spellID uint) error {
	return db.Delete(&PreparedSpell{}, spellID).Error
}

func (db *DB) GetPreparedSpell(spellID uint) (*PreparedSpell, error) {
	var spell PreparedSpell
	if err := db.First(&spell, spellID).Error; err != nil {
		return nil, err
	}
	return &spell, nil
}

// --- Return to Safety ---

// ReturnToSafety converts found treasure to XP and moves it to the purse.
// xpModPercent is the total XP modifier (human bonus + prime ability modifier).
func (db *DB) ReturnToSafety(characterID uint, xpModPercent int, gameDay int) error {
	ch, err := db.GetCharacter(characterID)
	if err != nil {
		return fmt.Errorf("get character: %w", err)
	}

	// Calculate GP value of found treasure
	foundPurse := engine.CoinPurse{
		CP: ch.FoundCP,
		SP: ch.FoundSP,
		EP: ch.FoundEP,
		GP: ch.FoundGP,
		PP: ch.FoundPP,
	}
	gpValue := engine.CoinPurseGPValue(foundPurse)

	// Apply XP modifiers
	xpGained := engine.ApplyXPModifiers(gpValue, xpModPercent)

	// Create XP log entry
	if err := db.CreateXPLogEntry(&XPLogEntry{
		CharacterID: characterID,
		Amount:      xpGained,
		Description: fmt.Sprintf("Treasure: %d GP value (mod %+d%%)", gpValue, xpModPercent),
	}); err != nil {
		return fmt.Errorf("xp log: %w", err)
	}

	// Zero found treasure (coins stay in inventory; purse is computed)

	ch.FoundCP = 0
	ch.FoundSP = 0
	ch.FoundEP = 0
	ch.FoundGP = 0
	ch.FoundPP = 0

	// Add XP
	ch.TotalXP += xpGained

	if err := db.UpdateCharacter(ch); err != nil {
		return fmt.Errorf("update character: %w", err)
	}

	// Audit log
	if err := db.AddAuditLog(characterID, "return_to_safety",
		fmt.Sprintf("Treasure %d GP value → %d XP", gpValue, xpGained), gameDay); err != nil {
		return fmt.Errorf("audit log: %w", err)
	}

	return nil
}
