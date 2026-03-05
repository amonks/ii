CREATE TABLE IF NOT EXISTS characters (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	class TEXT NOT NULL,
	kindred TEXT NOT NULL,
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
	calendar_start_day INTEGER NOT NULL DEFAULT 1,
	birthday_month TEXT NOT NULL DEFAULT '',
	birthday_day INTEGER NOT NULL DEFAULT 0,
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
	level INTEGER NOT NULL DEFAULT 1,
	attack TEXT NOT NULL DEFAULT '',
	morale INTEGER NOT NULL DEFAULT 0,
	has_barding INTEGER NOT NULL DEFAULT 0,
	saddle_type TEXT NOT NULL DEFAULT '',
	loyalty INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS companions_by_character ON companions(character_id);

CREATE TABLE IF NOT EXISTS retainer_contracts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	employer_id INTEGER NOT NULL,
	retainer_id INTEGER NOT NULL,
	loot_share_pct REAL NOT NULL DEFAULT 15.0,
	xp_share_pct REAL NOT NULL DEFAULT 50.0,
	daily_wage_cp INTEGER NOT NULL DEFAULT 0,
	hired_on_day INTEGER NOT NULL DEFAULT 1,
	active INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_retainer_contracts_employer ON retainer_contracts(employer_id);
CREATE INDEX IF NOT EXISTS idx_retainer_contracts_retainer ON retainer_contracts(retainer_id);

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

CREATE TABLE IF NOT EXISTS prepared_spells (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	name TEXT NOT NULL,
	spell_level INTEGER NOT NULL DEFAULT 1,
	used INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_prepared_spells_character ON prepared_spells(character_id);

CREATE TABLE IF NOT EXISTS enchantment_uses (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	character_id INTEGER NOT NULL REFERENCES characters(id),
	used INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_enchantment_uses_character ON enchantment_uses(character_id);
