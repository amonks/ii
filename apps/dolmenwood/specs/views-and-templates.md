# Views and Templates

## View Model (`server/views.go`)

### `CharacterView`

The central view model assembled by `buildCharacterView()`. Contains all data needed to render the full character sheet. This function:

1. Loads all related data from DB (items, companions, transactions, XP log, notes, audit log, bank deposits, retainer contracts)
2. Computes calendar display from game day
3. Converts DB items to engine items for calculations
4. Computes encumbrance via `engine.CalculateEncumbrance()`
5. Builds companion views with breed-derived stats
6. Aggregates coin items from inventory, computes purse = inventory - found
7. Determines AC from equipped armor + DEX
8. Computes weapons from equipped items
9. Determines attack bonus, save targets, and class feature counts (combat talents, enchanter glamours, bard enchantment uses, thief backstab, bard/hunter skills) via `ClassAttackBonus` / `ClassSaveTargets` and skill helpers
10. Determines XP modifier (using `ClassPrimes`) and level-up eligibility
11. Builds inventory tree, move targets, store catalog
12. Computes speed from encumbrance slots
13. Builds bank deposit views with maturity info
14. Loads active retainer contracts and builds retainer views

### Key View Types

- **`InventoryItem`** -- Wraps `db.Item` with computed `Slots`, `BundleSize`, `Children` (nested tree), `Capacity`, `UsedSlots`, `SellPriceCP`, `CoinValueLabel`
- **`CompanionInventory`** -- Groups items under a companion with `UsedSlots`
- **`CompanionView`** -- Wraps `db.Companion` with engine-derived stats (AC, speed, load, saves, attack, morale, loyalty)
- **`RetainerView`** -- Wraps a retainer contract + retainer Character with computed combat stats (AC, attack bonus, saves, speed, weapons), class/kindred traits, class features (combat talents, glamours, enchantment uses, skill targets, backstab), loyalty, and weapon summaries
- **`BankDepositView`** -- Wraps `BankDeposit` with `IsMature`, `DaysUntilMature`, `GPValue`
- **`MoveTarget`** -- Dropdown target for moving items ("Equipped", "Backpack", "Bessie (Mule)", etc.)
- **`StoreGroup` / `StoreItem`** -- Store catalog groups with item details
- **`SpeedChartCell`** -- For the visual encumbrance speed chart (filled/unfilled, colored by speed tier)

### RetainerView

```
type RetainerView struct {
    Contract        db.RetainerContract
    Character       *db.Character
    Items           []db.Item
    EquippedItems   []InventoryItem
    CompanionGroups []CompanionInventory
    EquippedSlots   int
    StowedSlots     int
    StowedCapacity  int
    MoveTargets     []MoveTarget
    AC              int
    AttackBonus     int
    Saves           engine.SaveTargets
    Speed           int
    Loyalty         int
    Weapons         []engine.EquippedWeapon
    KindredTraits   []engine.Trait
    ClassTraits     []engine.Trait
}
```

For each active retainer contract, `buildCharacterView` loads the retainer's Character and computes their stats using the same engine functions as the employer (ClassAttackBonus, ClassSaveTargets, CharacterAC, etc.). It also loads the retainer's inventory and companion data, computes encumbrance and inventory trees, and builds move targets scoped to the retainer's items.

### Inventory Tree

Items are organized into a tree structure for display:
- Root items are items with no `ContainerID` and no `CompanionID`
- Children of containers are nested under their parent
- Separate sections: equipped items on character, items per companion

### Move Targets

The move-to dropdown includes:
- "Equipped" (on character)
- Each personal container by name (e.g., "Backpack")
- Each companion by name (e.g., "Bessie (Mule)")
- Containers on companions
- "Bank" for coin deposits

### Speed Charts

`EquippedSpeedChart()` / `StowedSpeedChart()` -- Generate colored cell arrays for visual speed brackets. Each cell is colored green (40'), amber (30'), orange (20'), or red (10') based on its position in the slot range.

### Store Catalog

`buildStoreGroups()` -- Hardcoded catalog with 5 groups:
- Adventuring Gear (56 items)
- Weapons (18 items)
- Ammunition (3 items)
- Armour (7 items)
- Horses and Vehicles (17 items)

Each item is enriched with weight, bulk, damage, AC, qualities, load capacity, cargo capacity, container capacity from the engine.

## Template Files (`.templ`)

Templates use the `templ` language (Go-based, compiles to Go). 15 template files.

### Page Structure

- **`layout.templ`** -- HTML skeleton: head (fonts, Tailwind, HTMX, custom CSS), nav bar ("Dolmenwood" heading), centered main container. Title format: "{name} -- Dolmenwood"
- **`sheet.templ`** -- Character sheet layout: wraps all sections in `SheetBody`. Sections in order: DayCounter, Stats, Traits, Inventory, Store, Encumbrance, Companions, Retainers, Wealth, Bank, XP, Advancement, Notes, FullLog
- **`list.templ`** -- Character index: list of character cards (with retainers shown under their employer) + "New Character" form with class and kindred selection

### Section Templates

Each section is a collapsible card (using `CardDisclosure` from `styles.templ`):

- **`stats.templ`** -- Ability scores (6 boxes with modifiers), combat (HP form, AC breakdown, attack bonus, weapons), birthday selectors, saves (5 targets + magic resistance + conditional bonuses), speed breakdown, alignment/background/liege.
- **`traits.templ`** -- Kindred traits list, class traits list, class features (combat talents, glamours), moon sign display
- **`skills.templ`** -- Skill targets sections for thief, bard, and hunter classes
- **`inventory.templ`** -- Equipped items section, companion inventory sections, add-item forms. Item rows show: name, badges (quantity, tiny, slots, capacity), weapon damage, armor AC, inline note editing, action buttons (split, move, sell, decrement, delete)
- **`retainers.templ`** -- Adventurer retainer section: per-retainer stat block (name, class/level, HP, AC, attack, saves, speed, loyalty), class feature callouts (backstab, skill targets, combat talents, glamours), weapons list, contract terms display, link to retainer's own sheet, inline inventory list (equipped, stowed slots, companion gear) with "Take" buttons and an add-item form, "Hire Adventurer" form
- **`wealth.templ`** -- Total coins overview, purse/found treasure boxes, treasure form, "Return to Safety" button, transaction log with undo
- **`bank.templ`** -- Deposit list with maturity badges, withdraw form
- **`xp.templ`** -- Progress bar, level-up button (pulsing when eligible), add XP form, XP log
- **`advancement.templ`** -- Class advancement table with current level highlighted
- **`companions.templ`** -- Per-companion stat grid (HP, AC, speed, load), attack, saves, morale/loyalty, edit form, add companion form
- **`notes.templ`** -- Note list with delete buttons, add note form
- **`store.templ`** -- Collapsible store groups, item rows with buy buttons

### Design System (`styles.templ`)

~833 lines of reusable templ components:

**Structure:**
- `Card`, `CardDisclosure` (collapsible with show/hide), `CardSection`, `InfoBox`, `WarningBox`

**Layout:**
- `Header`, `Row`, `Grid` (2-6 columns), `FormRow`, `Cluster`, `Indent`, `VStack`, `InlineActions`

**Typography:**
- `Heading` (Kyrios font), `HeadingLg`, `Text`, `Label`, `SectionHeader`, `ValueText`
- `Accent` with color variants: gold, danger, info, purple, muted, amber, weapon, armor

**Data Display:**
- `Billboard`, `ProgressBar`, `SpeedChart`, `SpeedCell`, `LegendSwatch`, `CoinDisplay`

**Tables:**
- `DataTable`, `StyledTable`, `THeadRow`, `TRow` (with highlight), `TCell`

**Badges:**
- `Badge`, `BadgeSuccess`, `BadgeWarning`, `BadgeDanger`, `BadgeGold`

**Buttons:**
- `BtnPrimary`, `BtnSecondary`, `BtnDelete`, `BtnDecrement`, `BtnSell`, `BtnWide` (variants: primary, success, pulse), `BtnAction` (variants: found, withdraw, purse)

**Forms:**
- `FormInput`, `FormInputNumber`, `FormInputSmall`, `FormSelect`, `MoveSelect`, `SplitInput`, `InlineNoteInput`, `FormInputDayAdvance`

**Disclosure:**
- `Details`, `Summary`, `DetailsBody`, `LogEntry`

### Fonts (`server/font.go`)

4 embedded woff2 fonts:
- `ATKyriosStandard-Medium` -- Headings (Kyrios font family)
- `martina-plantijn-bold` / `martina-plantijn-regular` -- Body text
- `AtTextual-Retina` -- Form inputs

Served with `Cache-Control: public, max-age=31536000, immutable`.
