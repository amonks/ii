package engine

import "fmt"

var calendarMonthNames = []string{
	"Grimvold",
	"Lymewald",
	"Haggryme",
	"Symswald",
	"Harchment",
	"Iggwyld",
	"Chysting",
	"Lillipythe",
	"Haelhold",
	"Reedwryme",
	"Obthryme",
	"Braghold",
}

var calendarWeekdayNames = []string{
	"Colly",
	"Chime",
	"Hayme",
	"Moot",
	"Frisk",
	"Eggfast",
	"Sunning",
}

var calendarWysendays = map[int][]string{
	1:  {"Hanglemas", "Dyboll's Day"},
	3:  {"Yarl's Day", "The Day of Virgins"},
	4:  {"Hopfast"},
	5:  {"Smithing"},
	6:  {"Shortening", "Longshank's Day"},
	7:  {"Bradging", "Copsewallow", "Chalice"},
	8:  {"Old Dobey's Day"},
	10: {"Shub's Eve", "Druden Day"},
	12: {"The Day of Doors", "Dolmenday"},
}

type CalendarDate struct {
	Year  int
	Month int
	Day   int
}

type CalendarDisplay struct {
	Date       CalendarDate
	MonthName  string
	Weekday    string
	Wysenday   string
	IsWysenday bool
}

var monthLengths = []int{30, 28, 30, 29, 29, 30, 31, 29, 28, 30, 28, 30}

const daysPerYear = 352
const daysPerWeek = 7

func MonthName(month int) (string, error) {
	if month < 1 || month > len(calendarMonthNames) {
		return "", fmt.Errorf("invalid month %d", month)
	}
	return calendarMonthNames[month-1], nil
}

func WeekdayName(weekday int) (string, error) {
	if weekday < 1 || weekday > len(calendarWeekdayNames) {
		return "", fmt.Errorf("invalid weekday %d", weekday)
	}
	return calendarWeekdayNames[weekday-1], nil
}

func WysendayName(month, day int) (string, bool) {
	entries, ok := calendarWysendays[month]
	if !ok {
		return "", false
	}
	start := monthLengths[month-1] - len(entries) + 1
	index := day - start
	if index < 0 || index >= len(entries) {
		return "", false
	}
	return entries[index], true
}

func WysendayCountThrough(month int, day int) (int, error) {
	if month < 1 || month > len(monthLengths) {
		return 0, fmt.Errorf("invalid month %d", month)
	}
	maxDay := monthLengths[month-1]
	if day < 1 || day > maxDay {
		return 0, fmt.Errorf("invalid day %d for month %d", day, month)
	}
	count := 0
	for i := 1; i <= month; i++ {
		entries := calendarWysendays[i]
		if len(entries) == 0 {
			continue
		}
		start := monthLengths[i-1] - len(entries) + 1
		if i == month {
			if day >= start {
				count += day - start + 1
			}
			continue
		}
		count += len(entries)
	}
	return count, nil
}

func WeekdayForDayOfYear(dayOfYear int) (int, error) {
	if dayOfYear < 1 || dayOfYear > daysPerYear {
		return 0, fmt.Errorf("invalid day of year %d", dayOfYear)
	}
	month, day, err := DateFromDayOfYear(dayOfYear)
	if err != nil {
		return 0, err
	}
	wysendayCount, err := WysendayCountThrough(month, day)
	if err != nil {
		return 0, err
	}
	if wysendayCount > dayOfYear {
		return 0, fmt.Errorf("invalid wysenday count for day of year %d", dayOfYear)
	}
	weekdayIndex := dayOfYear - wysendayCount
	return ((weekdayIndex - 1) % daysPerWeek) + 1, nil
}

func DayOfYear(month, day int) (int, error) {
	if month < 1 || month > len(monthLengths) {
		return 0, fmt.Errorf("invalid month %d", month)
	}
	maxDay := monthLengths[month-1]
	if day < 1 || day > maxDay {
		return 0, fmt.Errorf("invalid day %d for month %d", day, month)
	}
	offset := 0
	for i := 0; i < month-1; i++ {
		offset += monthLengths[i]
	}
	return offset + day, nil
}

func DateFromDayOfYear(dayOfYear int) (int, int, error) {
	if dayOfYear < 1 || dayOfYear > daysPerYear {
		return 0, 0, fmt.Errorf("invalid day of year %d", dayOfYear)
	}
	remaining := dayOfYear
	for month, length := range monthLengths {
		if remaining <= length {
			return month + 1, remaining, nil
		}
		remaining -= length
	}
	return 0, 0, fmt.Errorf("invalid day of year %d", dayOfYear)
}

func StartDayOfYearForGameDay(gameDay int, month int, day int) (int, error) {
	if gameDay < 1 {
		return 0, fmt.Errorf("invalid game day %d", gameDay)
	}
	dayOfYear, err := DayOfYear(month, day)
	if err != nil {
		return 0, err
	}
	startDay := dayOfYear - (gameDay - 1)
	for startDay <= 0 {
		startDay += daysPerYear
	}
	for startDay > daysPerYear {
		startDay -= daysPerYear
	}
	return startDay, nil
}

func CalendarDateForGameDay(startDayOfYear int, gameDay int) (CalendarDate, error) {
	if startDayOfYear < 1 || startDayOfYear > daysPerYear {
		return CalendarDate{}, fmt.Errorf("invalid start day %d", startDayOfYear)
	}
	if gameDay < 1 {
		return CalendarDate{}, fmt.Errorf("invalid game day %d", gameDay)
	}
	dayIndex := (startDayOfYear - 1) + (gameDay - 1)
	year := (dayIndex / daysPerYear) + 1
	dayOfYear := (dayIndex % daysPerYear) + 1
	month, day, err := DateFromDayOfYear(dayOfYear)
	if err != nil {
		return CalendarDate{}, err
	}
	return CalendarDate{Year: year, Month: month, Day: day}, nil
}

func CalendarDisplayForGameDay(startDayOfYear int, gameDay int) (CalendarDisplay, error) {
	date, err := CalendarDateForGameDay(startDayOfYear, gameDay)
	if err != nil {
		return CalendarDisplay{}, err
	}
	monthName, err := MonthName(date.Month)
	if err != nil {
		return CalendarDisplay{}, err
	}
	dayOfYear, err := DayOfYear(date.Month, date.Day)
	if err != nil {
		return CalendarDisplay{}, err
	}
	weekdayIndex, err := WeekdayForDayOfYear(dayOfYear)
	if err != nil {
		return CalendarDisplay{}, err
	}
	weekdayName, err := WeekdayName(weekdayIndex)
	if err != nil {
		return CalendarDisplay{}, err
	}
	wysendayName, isWysenday := WysendayName(date.Month, date.Day)
	return CalendarDisplay{
		Date:       date,
		MonthName:  monthName,
		Weekday:    weekdayName,
		Wysenday:   wysendayName,
		IsWysenday: isWysenday,
	}, nil
}
