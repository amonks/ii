# Calendar and Moon System

## Dolmenwood Calendar (`engine/calendar.go`)

### Months

The Dolmenwood year has 12 months totaling **352 days**:

| # | Month      | Days |
|---|------------|------|
| 1 | Grimvold   | 30   |
| 2 | Lymewald   | 28   |
| 3 | Haggryme   | 30   |
| 4 | Symswald   | 29   |
| 5 | Harchment  | 29   |
| 6 | Iggwyld    | 30   |
| 7 | Chysting   | 31   |
| 8 | Lillipythe | 29   |
| 9 | Haelhold   | 28   |
| 10| Reedwryme  | 30   |
| 11| Obthryme   | 28   |
| 12| Braghold   | 30   |

### Weekdays

7-day week cycle: Colly, Chime, Hayme, Moot, Frisk, Eggfast, Sunning.

### Wysendays (Holy Days)

Wysendays are special days at the end of certain months. They **do not advance the weekday cycle** -- they sit outside the normal 7-day week. After a wysenday, the next regular day continues with the weekday that would have followed.

| Month      | Wysendays                    |
|------------|------------------------------|
| Grimvold   | Hanglemas, Dyboll's Day      |
| Haggryme   | Rumpus Night                 |
| Symswald   | Quintilis                    |
| Harchment  | Fools' Day                   |
| Iggwyld    | Midsummer, St. Ambrose's Day |
| Chysting   | Briar Day, Breeze Day, Corn Day |
| Lillipythe | Samhain                      |
| Haelhold   | Harvest Home                 |
| Reedwryme  | Cocking Day, Cocking Night   |
| Braghold   | Yule, Year's End             |

### Key Functions

- `DayOfYear(month, day)` / `DateFromDayOfYear(dayOfYear)` -- Convert between month/day and sequential day-of-year (1-352)
- `WysendayCountThrough(month, day)` -- Counts total wysendays up to a given date. Critical because wysendays don't count as weekdays.
- `WeekdayForDayOfYear(dayOfYear)` -- Computes weekday by subtracting wysenday count from day-of-year, then mod 7
- `WysendayName(month, day)` -- Returns wysenday name for a date, or empty string
- `CalendarDateForGameDay(startDayOfYear, gameDay)` -- Converts a game day counter to a `CalendarDate{Year, Month, Day}`, handling year rollover
- `CalendarDisplayForGameDay(startDayOfYear, gameDay)` -- Full display: date, month name, weekday name, wysenday flag

### Game Day Mapping

The game tracks time via a `CurrentDay` counter (integer, incremented during play). A `CalendarStartDay` anchors this counter to the Dolmenwood calendar:

- `StartDayOfYearForGameDay(gameDay, month, day)` -- Given a current game day and its calendar date, calculates the starting day-of-year offset
- From this offset, any game day can be converted to a calendar date

### Data Types

- `CalendarDate{Year, Month, Day}` -- A date in the Dolmenwood calendar
- `CalendarDisplay{Date, MonthName, Weekday, Wysenday, IsWysenday}` -- Full display info

## Moon Signs (`engine/moon.go`)

### The Twelve Moons

Each moon roughly corresponds to one calendar month. Each has three phases (Waxing, Full, Waning) with unique gameplay effects:

| Moon            | Corresponding Month |
|-----------------|-------------------|
| Grinning Moon   | Grimvold          |
| Dead Moon       | Lymewald          |
| Beast Moon      | Haggryme          |
| Squamous Moon   | Symswald          |
| Knight's Moon   | Harchment         |
| Rotting Moon     | Iggwyld           |
| Maiden's Moon   | Chysting          |
| Witch's Moon    | Lillipythe        |
| Robber's Moon   | Haelhold          |
| Goat Moon       | Reedwryme         |
| Narrow Moon     | Obthryme          |
| Black Moon      | Braghold          |

### Moon Sign Determination

`MoonSignFromBirthday(month, day)` -- Given a birthday (month name + day), returns a `MoonSign{Moon, Phase, Effect}`. The 36 moon ranges (12 moons x 3 phases) are defined with start/end date boundaries. Some ranges wrap around year boundaries (e.g., Black Moon Waning spans Braghold into Grimvold).

### Gameplay Effects

Each moon sign phase has a unique effect string, such as:
- Saving throw bonuses/penalties
- Reaction roll modifiers
- Initiative modifiers
- Skill check bonuses

Moon sign effects that mention "saving throw" are picked up by `ConditionalSaveBonuses()` in `save_bonuses.go` and displayed alongside save targets on the character sheet.

### Data Types

- `MoonSign{Moon, Phase, Effect}` -- A character's birth moon sign
- `Month{Name, Days}` -- Calendar month (note: duplicates data from `calendar.go` using string-based names)

### Calendar/Moon Duplication

`moon.go` maintains its own `calendarMonths` variable with month names and lengths, duplicating data from `calendar.go`. The `moon.go` version uses string-based month names for lookups, while `calendar.go` uses integer indices.
