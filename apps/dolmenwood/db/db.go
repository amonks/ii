package db

import (
	"fmt"
	"time"

	"monks.co/apps/dolmenwood/engine"
	"monks.co/pkg/database"
)

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
	ID          uint   `gorm:"primarykey"`
	CharacterID uint   `gorm:"column:character_id"`
	Name        string `gorm:"column:name"`
	Breed       string `gorm:"column:breed"`
	HPCurrent   int    `gorm:"column:hp_current"`
	HPMax       int    `gorm:"column:hp_max"`
	HasBarding  bool   `gorm:"column:has_barding"`
	SaddleType  string `gorm:"column:saddle_type"` // "", "riding", "pack"
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

type XPLogEntry struct {
	ID          uint      `gorm:"primarykey"`
	CharacterID uint      `gorm:"column:character_id"`
	Amount      int       `gorm:"column:amount"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (XPLogEntry) TableName() string { return "xp_log" }

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

const schema = `
CREATE TABLE IF NOT EXISTS characters (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	class TEXT NOT NULL DEFAULT 'Knight',
	kindred TEXT NOT NULL DEFAULT 'Human',
	level INTEGER NOT NULL DEFAULT 1,
	str INTEGER NOT NULL DEFAULT 10,
	dex INTEGER NOT NULL DEFAULT 10,
	con INTEGER NOT NULL DEFAULT 10,
	int_ INTEGER NOT NULL DEFAULT 10,
	wis INTEGER NOT NULL DEFAULT 10,
	cha INTEGER NOT NULL DEFAULT 10,
	hp_current INTEGER NOT NULL DEFAULT 0,
	hp_max INTEGER NOT NULL DEFAULT 0,
	armor_name TEXT NOT NULL DEFAULT '',
	armor_base_ac INTEGER NOT NULL DEFAULT 10,
	has_shield INTEGER NOT NULL DEFAULT 0,
	alignment TEXT NOT NULL DEFAULT '',
	background TEXT NOT NULL DEFAULT '',
	liege TEXT NOT NULL DEFAULT '',
	found_cp INTEGER NOT NULL DEFAULT 0,
	found_sp INTEGER NOT NULL DEFAULT 0,
	found_ep INTEGER NOT NULL DEFAULT 0,
	found_gp INTEGER NOT NULL DEFAULT 0,
	found_pp INTEGER NOT NULL DEFAULT 0,
	purse_cp INTEGER NOT NULL DEFAULT 0,
	purse_sp INTEGER NOT NULL DEFAULT 0,
	purse_ep INTEGER NOT NULL DEFAULT 0,
	purse_gp INTEGER NOT NULL DEFAULT 0,
	purse_pp INTEGER NOT NULL DEFAULT 0,
	coin_companion_id INTEGER REFERENCES companions(id),
	coin_container_id INTEGER REFERENCES items(id),
	coins_migrated INTEGER NOT NULL DEFAULT 0,
	total_xp INTEGER NOT NULL DEFAULT 0,
	current_day INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME,
	updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	name TEXT NOT NULL,
	weight_override INTEGER,
	quantity INTEGER NOT NULL DEFAULT 1,
	location TEXT NOT NULL DEFAULT 'stowed',
	notes TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0,
	container_id INTEGER REFERENCES items(id),
	companion_id INTEGER REFERENCES companions(id),
	is_tiny INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS items_by_character ON items(character_id);

CREATE TABLE IF NOT EXISTS companions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	name TEXT NOT NULL,
	breed TEXT NOT NULL DEFAULT '',
	hp_current INTEGER NOT NULL DEFAULT 0,
	hp_max INTEGER NOT NULL DEFAULT 0,
	ac INTEGER NOT NULL DEFAULT 10,
	speed INTEGER NOT NULL DEFAULT 40,
	load_capacity INTEGER NOT NULL DEFAULT 0,
	has_barding INTEGER NOT NULL DEFAULT 0,
	saddle_type TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS companions_by_character ON companions(character_id);

CREATE TABLE IF NOT EXISTS transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	amount INTEGER NOT NULL,
	coin_type TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	is_found_treasure INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS transactions_by_character ON transactions(character_id);

CREATE TABLE IF NOT EXISTS xp_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	amount INTEGER NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS xp_log_by_character ON xp_log(character_id);

CREATE TABLE IF NOT EXISTS notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	content TEXT NOT NULL,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS notes_by_character ON notes(character_id);

CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	action TEXT NOT NULL,
	detail TEXT NOT NULL DEFAULT '',
	game_day INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS audit_log_by_character ON audit_log(character_id);

CREATE TABLE IF NOT EXISTS bank_deposits (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	coin_notes TEXT NOT NULL DEFAULT '',
	cp_value INTEGER NOT NULL DEFAULT 0,
	deposit_day INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS bank_deposits_by_character ON bank_deposits(character_id);
`

const migrations = `
-- Replace slot_cost with weight_override (nullable, coins per unit)
ALTER TABLE items ADD COLUMN weight_override INTEGER;
ALTER TABLE items DROP COLUMN slot_cost;
`

const migrationContainerHierarchy = `
ALTER TABLE items ADD COLUMN container_id INTEGER REFERENCES items(id);
ALTER TABLE items ADD COLUMN companion_id INTEGER REFERENCES companions(id);
`

const migrationTinyItems = `
ALTER TABLE items ADD COLUMN is_tiny INTEGER NOT NULL DEFAULT 0;
`

const migrationCompanionSaddleType = `
ALTER TABLE companions ADD COLUMN saddle_type TEXT NOT NULL DEFAULT '';
`

const migrationCoinLocation = `
ALTER TABLE characters ADD COLUMN coin_companion_id INTEGER REFERENCES companions(id);
ALTER TABLE characters ADD COLUMN coin_container_id INTEGER REFERENCES items(id);
`

const migrationCoinsMigrated = `
ALTER TABLE characters ADD COLUMN coins_migrated INTEGER NOT NULL DEFAULT 0;
`

const migrationCurrentDay = `
ALTER TABLE characters ADD COLUMN current_day INTEGER NOT NULL DEFAULT 1;
`

const migrationAuditLogGameDay = `
ALTER TABLE audit_log ADD COLUMN game_day INTEGER NOT NULL DEFAULT 0;
`

const migrationBankDeposits = `
CREATE TABLE IF NOT EXISTS bank_deposits (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	coin_notes TEXT NOT NULL DEFAULT '',
	cp_value INTEGER NOT NULL DEFAULT 0,
	deposit_day INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS bank_deposits_by_character ON bank_deposits(character_id);
`

func New() (*DB, error) {
	d, err := database.OpenFromDataFolder("dolmenwood")
	if err != nil {
		return nil, err
	}
	if err := d.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	// Best-effort migrations for existing DBs; ignore errors from already-applied migrations.
	d.Exec(migrations)
	d.Exec(migrationContainerHierarchy)
	d.Exec(migrationTinyItems)
	d.Exec(migrationCompanionSaddleType)
	d.Exec(migrationCoinLocation)
	d.Exec(migrationCoinsMigrated)
	d.Exec(migrationCurrentDay)
	d.Exec(migrationAuditLogGameDay)
	d.Exec(migrationBankDeposits)
	return &DB{d}, nil
}

func NewMemory() (*DB, error) {
	d, err := database.Open(":memory:")
	if err != nil {
		return nil, err
	}
	if err := d.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &DB{d}, nil
}

// --- Character CRUD ---

func (db *DB) CreateCharacter(ch *Character) error {
	ch.CreatedAt = time.Now()
	ch.UpdatedAt = ch.CreatedAt
	return db.Create(ch).Error
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
