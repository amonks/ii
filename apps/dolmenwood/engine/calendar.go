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

type CalendarDate struct {
	Year  int
	Month int
	Day   int
}

var monthLengths = []int{30, 28, 30, 29, 29, 30, 31, 29, 28, 30, 28, 30}

const daysPerYear = 352

func MonthName(month int) (string, error) {
	if month < 1 || month > len(calendarMonthNames) {
		return "", fmt.Errorf("invalid month %d", month)
	}
	return calendarMonthNames[month-1], nil
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
