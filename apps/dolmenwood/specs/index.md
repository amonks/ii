# Dolmenwood Specs

### [Architecture](architecture.md)

Overall application structure: the three-layer architecture (db, engine, server), tech stack (Go + templ + HTMX + SQLite + Tailscale), package layout, data flow from HTTP request through DB and engine to rendered templates, HTMX partial-update patterns, build system, and entrypoint behavior.

### [Database Layer](database.md)

SQLite persistence via GORM. Covers the 9-table schema (characters, items, companions, retainer_contracts, transactions, xp_log, notes, audit_log, bank_deposits), all model fields, cascade delete behaviors (character, item, companion), item transfers between characters, the ReturnToSafety business logic, and the migration strategy using best-effort ALTERs.

### [Ability Scores, Saves, and Magic Resistance](abilities-and-saves.md)

The B/X ability modifier table, prime ability XP modifier brackets, armor class computation (including Breggle fur bonus and magic armor), the five saving throw targets (generic for all classes via `ClassSaveTargets`), conditional save bonuses aggregated from kindred traits, class traits, and moon signs, and magic resistance by kindred and wisdom.

### [Kindred and Class Traits](traits.md)

Traits for all six kindreds (Human, Elf, Grimalkin, Mossling, Woodgrue, Breggle) and nine classes (Fighter, Bard, Enchanter, Knight, Cleric, Friar, Magician, Thief, Hunter). Documents level-dependent traits like Breggle horn damage scaling, Breggle gaze uses, Knight knighthood/monster slayer unlocks, and Cleric holy order. Also covers XP modifiers by kindred and the `ClassPrimes` function.

### [Calendar and Moon System](calendar-and-moon.md)

The Dolmenwood calendar with 12 months, 352-day year, and 7-day week. Wysendays (holy days) that sit outside the normal weekday cycle. Game day to calendar date mapping. The twelve moons with their waxing/full/waning phases, moon sign determination from birthday, and moon sign gameplay effects.

### [Equipment and Encumbrance](equipment-and-encumbrance.md)

The item catalog (24 weapons, 8 armor types, 80+ general items, 12 containers), magic bonus prefix parsing, the slot-based encumbrance system, item slot cost resolution order, bundled items, the container/companion hierarchy, stowed capacity from personal containers (capped at 16), speed calculation from equipped and stowed slot tiers, and coin weight.

### [Wealth and Banking](wealth-and-banking.md)

The five coin denominations and conversion rates, coin purse operations, inventory coin management (the consolidated "Coins" item with denomination notes), transaction/coin expression parsing and formatting, found treasure tracking and the return-to-safety XP conversion flow, and the banking system with deposit maturity (30-day rule), withdrawal planning (mature lots first, 10% fee on immature), and the bank UI.

### [XP and Advancement](xp-and-advancement.md)

XP modifier calculation from prime abilities and kindred, level-up detection via generic `ClassLevelForXP` (for all 9 classes), the 15-level advancement tables, generic class functions (`ClassAttackBonus`, `ClassSaveTargets`, `ClassPrimes`, `ClassSpecificColumns`), and the return-to-safety flow that converts found treasure GP value to XP.

### [Companions](companions.md)

The seven companion breeds (six mounts/pack animals plus Townsfolk retainers) with their stat blocks, saddle mechanics (no saddle = 0 capacity, riding = 5, pack = full), horse barding (+2 AC), retainer loyalty, companion inventory and how companion items interact with the encumbrance system, companion cascade delete behavior, and the distinction between townsfolk retainers (Companion records) and adventurer retainers (Character records).

### [Retainer Contracts](retainer-contracts.md)

The adventurer retainer system: the `retainer_contracts` table linking employer and retainer Characters, contract terms (loot share %, XP share %, daily wage), the hiring and dismissal flows, RetainerView on the employer's sheet with full inline stats, retainer independence as standalone playable characters, and the character list display.

### [Item Transfers](item-transfers.md)

Planned work for phase 3: how items move between characters (employer ↔ retainer): the `TransferItem` DB operation, full vs. partial transfers, the retainer inventory display on the employer's sheet, transfer UI with give/take buttons, move target integration, audit trail, and encumbrance impact.

### [Class-Specific Subsystems](class-subsystems.md)
Full mechanical support for all 9 classes: spellcasting system (Cleric, Friar, Magician, Enchanter) with spell slots, preparation, and casting; Fighter combat talents; Thief backstab and skills; Hunter animal companion; glamours (Enchanter, Elf, Grimalkin, Woodgrue); turning undead (Cleric, Friar); Friar's Armour of Faith and herbalism; Bard enchantment and counter charm; and implementation priority.

### [HTTP Handlers](http-handlers.md)

All routes and their handler implementations. Covers character creation with class/kindred selection, the smart item input parser, item movement and splitting logic, the treasure/transaction system with undo, store buying, bank withdrawal handler, calendar/day advancement, adventurer retainer hire/dismiss/update handlers, and the HTMX partial-update and OOB swap patterns used throughout.

### [Views and Templates](views-and-templates.md)

The `CharacterView` mega-view-model (including `RetainerView` for adventurer retainers) and how `buildCharacterView()` assembles it from DB data and engine computations. The inventory tree structure, move targets, speed charts, and store catalog enrichment. All 15 templ template files (including `retainers.templ`), the page structure, and the design system in styles.templ with its component library.

### [Store](store.md)

The in-game shop system: the hardcoded 5-group store catalog, buying flow with coin deduction and changemaking (preserving PP, using GP/SP/CP), companion breed purchases, selling at half price, spendable coin finding (prefers coins on character over those in containers/companions), and the store item ID format.
