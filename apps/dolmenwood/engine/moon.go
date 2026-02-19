package engine

import "strings"

type Month struct {
	Name string
	Days int
}

type MoonSign struct {
	Moon   string
	Phase  string
	Effect string
}

type monthDay struct {
	Month string
	Day   int
}

type moonRange struct {
	Moon   string
	Phase  string
	Start  monthDay
	End    monthDay
	Effect string
}

var calendarMonths = []Month{
	{Name: "Grimvold", Days: 30},
	{Name: "Lymewald", Days: 28},
	{Name: "Haggryme", Days: 30},
	{Name: "Symswald", Days: 29},
	{Name: "Harchment", Days: 29},
	{Name: "Iggwyld", Days: 30},
	{Name: "Chysting", Days: 31},
	{Name: "Lillipythe", Days: 29},
	{Name: "Haelhold", Days: 28},
	{Name: "Reedwryme", Days: 30},
	{Name: "Obthryme", Days: 28},
	{Name: "Braghold", Days: 30},
}

var moonRanges = []moonRange{
	{
		Moon:   "Grinning moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Grimvold", Day: 4},
		End:    monthDay{Month: "Grimvold", Day: 17},
		Effect: "There is a 50% chance that guardian undead will ignore your presence. (Though they act normally if you provoke them.)",
	},
	{
		Moon:   "Grinning moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Grimvold", Day: 18},
		End:    monthDay{Month: "Grimvold", Day: 20},
		Effect: "+1 bonus to saving throws against the powers of undead monsters.",
	},
	{
		Moon:   "Grinning moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Grimvold", Day: 21},
		End:    monthDay{Month: "Lymewald", Day: 3},
		Effect: "+1 bonus to attack rolls against undead monsters.",
	},
	{
		Moon:   "Dead moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Lymewald", Day: 4},
		End:    monthDay{Month: "Lymewald", Day: 16},
		Effect: "+1 bonus to attack and damage rolls the round after killing a foe.",
	},
	{
		Moon:   "Dead moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Lymewald", Day: 17},
		End:    monthDay{Month: "Lymewald", Day: 19},
		Effect: "If killed by non-magical means, you return to life in 1 turn with 1 hit point. Your CON and WIS are permanently reduced by 50% (minimum 3). This only takes effect once ever.",
	},
	{
		Moon:   "Dead moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Lymewald", Day: 20},
		End:    monthDay{Month: "Haggryme", Day: 4},
		Effect: "Undead monsters attack all others in your party before attacking you.",
	},
	{
		Moon:   "Beast moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Haggryme", Day: 5},
		End:    monthDay{Month: "Haggryme", Day: 18},
		Effect: "+1 reaction bonus when interacting with dogs and horses.",
	},
	{
		Moon:   "Beast moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Haggryme", Day: 19},
		End:    monthDay{Month: "Haggryme", Day: 21},
		Effect: "Wild animals attack all others in your party before attacking you.",
	},
	{
		Moon:   "Beast moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Haggryme", Day: 22},
		End:    monthDay{Month: "Symswald", Day: 3},
		Effect: "+1 bonus to attack rolls against wolves and bears.",
	},
	{
		Moon:   "Squamous moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Symswald", Day: 4},
		End:    monthDay{Month: "Symswald", Day: 17},
		Effect: "If you are afflicted by poison, its effects are delayed by one turn.",
	},
	{
		Moon:   "Squamous moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Symswald", Day: 18},
		End:    monthDay{Month: "Symswald", Day: 20},
		Effect: "+2 bonus to saving throws against the powers of dragons and wyrms. This includes their breath attacks and magical powers.",
	},
	{
		Moon:   "Squamous moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Symswald", Day: 21},
		End:    monthDay{Month: "Harchment", Day: 4},
		Effect: "+1 bonus to attack rolls against serpents and wyrms.",
	},
	{
		Moon:   "Knight's moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Harchment", Day: 5},
		End:    monthDay{Month: "Harchment", Day: 17},
		Effect: "+1 reaction bonus when interacting with nobles.",
	},
	{
		Moon:   "Knight's moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Harchment", Day: 18},
		End:    monthDay{Month: "Harchment", Day: 20},
		Effect: "+1 AC bonus against metal weapons.",
	},
	{
		Moon:   "Knight's moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Harchment", Day: 21},
		End:    monthDay{Month: "Iggwyld", Day: 4},
		Effect: "In melee with knights or soldiers, you act first on a tied initiative, as if you had won initiative.",
	},
	{
		Moon:   "Rotting moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Iggwyld", Day: 5},
		End:    monthDay{Month: "Iggwyld", Day: 18},
		Effect: "+1 reaction bonus when interacting with sentient fungi.",
	},
	{
		Moon:   "Rotting moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Iggwyld", Day: 19},
		End:    monthDay{Month: "Iggwyld", Day: 21},
		Effect: "+2 bonus to AC against attacks by fungal monsters.",
	},
	{
		Moon:   "Rotting moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Iggwyld", Day: 22},
		End:    monthDay{Month: "Chysting", Day: 3},
		Effect: "In your presence, fungal monsters suffer a -1 penalty to attacks and damage.",
	},
	{
		Moon:   "Maiden's moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Chysting", Day: 4},
		End:    monthDay{Month: "Chysting", Day: 17},
		Effect: "+1 reaction bonus when interacting with demi-fey.",
	},
	{
		Moon:   "Maiden's moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Chysting", Day: 18},
		End:    monthDay{Month: "Chysting", Day: 20},
		Effect: "+2 bonus to saving throws against charms and glamours.",
	},
	{
		Moon:   "Maiden's moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Chysting", Day: 21},
		End:    monthDay{Month: "Lillipythe", Day: 2},
		Effect: "+1 bonus to attack and damage rolls against shape-changers and those cloaked with illusions.",
	},
	{
		Moon:   "Witch's moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Lillipythe", Day: 3},
		End:    monthDay{Month: "Lillipythe", Day: 15},
		Effect: "When you receive healing magic, the number of hit points you gain is increased by one.",
	},
	{
		Moon:   "Witch's moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Lillipythe", Day: 16},
		End:    monthDay{Month: "Lillipythe", Day: 18},
		Effect: "+1 bonus to saving throws against divine magic.",
	},
	{
		Moon:   "Witch's moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Lillipythe", Day: 19},
		End:    monthDay{Month: "Haelhold", Day: 2},
		Effect: "+1 bonus to attack rolls against witches and divine spell casters.",
	},
	{
		Moon:   "Robber's moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Haelhold", Day: 3},
		End:    monthDay{Month: "Haelhold", Day: 16},
		Effect: "+1 reaction bonus when interacting with Chaotic persons.",
	},
	{
		Moon:   "Robber's moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Haelhold", Day: 17},
		End:    monthDay{Month: "Haelhold", Day: 19},
		Effect: "+1 bonus to AC against attacks by Chaotic persons.",
	},
	{
		Moon:   "Robber's moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Haelhold", Day: 20},
		End:    monthDay{Month: "Reedwryme", Day: 3},
		Effect: "+1 bonus to attack rolls against Chaotic persons.",
	},
	{
		Moon:   "Goat moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Reedwryme", Day: 4},
		End:    monthDay{Month: "Reedwryme", Day: 17},
		Effect: "+1 reaction bonus when interacting with goat-people.",
	},
	{
		Moon:   "Goat moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Reedwryme", Day: 18},
		End:    monthDay{Month: "Reedwryme", Day: 20},
		Effect: "Goat-people attack all others in your party before attacking you.",
	},
	{
		Moon:   "Goat moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Reedwryme", Day: 21},
		End:    monthDay{Month: "Obthryme", Day: 3},
		Effect: "+1 bonus to attack rolls against goat-people.",
	},
	{
		Moon:   "Narrow moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Obthryme", Day: 4},
		End:    monthDay{Month: "Obthryme", Day: 17},
		Effect: "+1 reaction bonus when interacting with fairies, but suffer a -1 penalty to all saving throws against fairy magic.",
	},
	{
		Moon:   "Narrow moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Obthryme", Day: 18},
		End:    monthDay{Month: "Obthryme", Day: 20},
		Effect: "If you are afflicted by a curse or geas, there is a 1-in-4 chance of the caster also being affected by their own magic.",
	},
	{
		Moon:   "Narrow moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Obthryme", Day: 21},
		End:    monthDay{Month: "Braghold", Day: 4},
		Effect: "+1 bonus to attack rolls against fairies and demi-fey.",
	},
	{
		Moon:   "Black moon",
		Phase:  "Waxing",
		Start:  monthDay{Month: "Braghold", Day: 5},
		End:    monthDay{Month: "Braghold", Day: 18},
		Effect: "Your chance of detecting secret doors when searching is increased by 1-in-6.",
	},
	{
		Moon:   "Black moon",
		Phase:  "Full",
		Start:  monthDay{Month: "Braghold", Day: 19},
		End:    monthDay{Month: "Braghold", Day: 21},
		Effect: "+2 bonus to AC and saving throw when surprised.",
	},
	{
		Moon:   "Black moon",
		Phase:  "Waning",
		Start:  monthDay{Month: "Braghold", Day: 22},
		End:    monthDay{Month: "Grimvold", Day: 3},
		Effect: "+2 bonus to saving throws versus illusions or glamours.",
	},
}

func Months() []Month {
	out := make([]Month, len(calendarMonths))
	copy(out, calendarMonths)
	return out
}

func DaysInMonth(month string) (int, bool) {
	for _, m := range calendarMonths {
		if strings.EqualFold(m.Name, month) {
			return m.Days, true
		}
	}
	return 0, false
}

func MoonSignFromBirthday(month string, day int) (MoonSign, bool) {
	dayOfYearValue, ok := dayOfYear(month, day)
	if !ok {
		return MoonSign{}, false
	}
	for _, r := range moonRanges {
		start, okStart := dayOfYear(r.Start.Month, r.Start.Day)
		end, okEnd := dayOfYear(r.End.Month, r.End.Day)
		if !okStart || !okEnd {
			continue
		}
		if start <= end {
			if dayOfYearValue < start || dayOfYearValue > end {
				continue
			}
		} else {
			if dayOfYearValue < start && dayOfYearValue > end {
				continue
			}
		}
		return MoonSign{Moon: r.Moon, Phase: r.Phase, Effect: r.Effect}, true
	}
	return MoonSign{}, false
}

func dayOfYear(month string, day int) (int, bool) {
	if day <= 0 {
		return 0, false
	}
	count := 0
	for _, m := range calendarMonths {
		if strings.EqualFold(m.Name, month) {
			if day > m.Days {
				return 0, false
			}
			return count + day, true
		}
		count += m.Days
	}
	return 0, false
}
