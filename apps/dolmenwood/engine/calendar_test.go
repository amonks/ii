package engine

import "testing"

func TestDayOfYear(t *testing.T) {
	tests := []struct {
		month int
		day   int
		want  int
	}{
		{month: 1, day: 1, want: 1},
		{month: 1, day: 30, want: 30},
		{month: 2, day: 1, want: 31},
		{month: 2, day: 28, want: 58},
		{month: 12, day: 30, want: 352},
	}
	for _, tt := range tests {
		got, err := DayOfYear(tt.month, tt.day)
		if err != nil {
			t.Fatalf("DayOfYear(%d, %d) error: %v", tt.month, tt.day, err)
		}
		if got != tt.want {
			t.Errorf("DayOfYear(%d, %d) = %d, want %d", tt.month, tt.day, got, tt.want)
		}
	}
}

func TestDateFromDayOfYear(t *testing.T) {
	tests := []struct {
		dayOfYear int
		wantMonth int
		wantDay   int
	}{
		{dayOfYear: 1, wantMonth: 1, wantDay: 1},
		{dayOfYear: 31, wantMonth: 2, wantDay: 1},
		{dayOfYear: 58, wantMonth: 2, wantDay: 28},
		{dayOfYear: 352, wantMonth: 12, wantDay: 30},
	}
	for _, tt := range tests {
		month, day, err := DateFromDayOfYear(tt.dayOfYear)
		if err != nil {
			t.Fatalf("DateFromDayOfYear(%d) error: %v", tt.dayOfYear, err)
		}
		if month != tt.wantMonth || day != tt.wantDay {
			t.Errorf("DateFromDayOfYear(%d) = %d/%d, want %d/%d", tt.dayOfYear, month, day, tt.wantMonth, tt.wantDay)
		}
	}
}

func TestMonthName(t *testing.T) {
	name, err := MonthName(1)
	if err != nil {
		t.Fatalf("MonthName: %v", err)
	}
	if name != "Grimvold" {
		t.Errorf("name = %q, want Grimvold", name)
	}

	if _, err := MonthName(0); err == nil {
		t.Fatal("expected error for invalid month")
	}
}

func TestDayOfYearRejectsInvalidDate(t *testing.T) {
	if _, err := DayOfYear(0, 1); err == nil {
		t.Fatal("expected error for invalid month")
	}
	if _, err := DayOfYear(2, 30); err == nil {
		t.Fatal("expected error for invalid day")
	}
}

func TestDateFromDayOfYearRejectsInvalidDay(t *testing.T) {
	if _, _, err := DateFromDayOfYear(0); err == nil {
		t.Fatal("expected error for invalid day of year")
	}
	if _, _, err := DateFromDayOfYear(400); err == nil {
		t.Fatal("expected error for invalid day of year")
	}
}

func TestCalendarDateForGameDay(t *testing.T) {
	date, err := CalendarDisplayForGameDay(1, 1)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Date.Month != 1 || date.Date.Day != 1 || date.Date.Year != 1 {
		t.Errorf("date = %+v, want month 1 day 1 year 1", date.Date)
	}
	if date.IsWysenday {
		t.Errorf("expected non-wysenday for game day 1")
	}
	if date.Weekday != "Colly" {
		t.Errorf("weekday = %q, want Colly", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 29)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if !date.IsWysenday {
		t.Fatalf("expected wysenday for game day 29")
	}
	if date.Wysenday != "Hanglemas" {
		t.Errorf("wysenday name = %q, want Hanglemas", date.Wysenday)
	}
	if date.Weekday != "Sunning" {
		t.Errorf("weekday = %q, want Sunning", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 30)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if !date.IsWysenday {
		t.Fatalf("expected wysenday for game day 30")
	}
	if date.Wysenday != "Dyboll's Day" {
		t.Errorf("wysenday name = %q, want Dyboll's Day", date.Wysenday)
	}
	if date.Weekday != "Sunning" {
		t.Errorf("weekday = %q, want Sunning", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 31)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Date.Month != 2 || date.Date.Day != 1 || date.Date.Year != 1 {
		t.Errorf("date = %+v, want month 2 day 1 year 1", date.Date)
	}
	if date.Weekday != "Colly" {
		t.Errorf("weekday = %q, want Colly", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 32)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Chime" {
		t.Errorf("weekday = %q, want Chime", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 33)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Hayme" {
		t.Errorf("weekday = %q, want Hayme", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 34)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Moot" {
		t.Errorf("weekday = %q, want Moot", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 35)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Frisk" {
		t.Errorf("weekday = %q, want Frisk", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 36)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Eggfast" {
		t.Errorf("weekday = %q, want Eggfast", date.Weekday)
	}
	date, err = CalendarDisplayForGameDay(1, 37)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Sunning" {
		t.Errorf("weekday = %q, want Sunning", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 38)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Colly" {
		t.Errorf("weekday = %q, want Colly", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 39)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Chime" {
		t.Errorf("weekday = %q, want Chime", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 40)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Hayme" {
		t.Errorf("weekday = %q, want Hayme", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 41)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Moot" {
		t.Errorf("weekday = %q, want Moot", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(1, 42)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Weekday != "Frisk" {
		t.Errorf("weekday = %q, want Frisk", date.Weekday)
	}

	date, err = CalendarDisplayForGameDay(300, 60)
	if err != nil {
		t.Fatalf("CalendarDisplayForGameDay: %v", err)
	}
	if date.Date.Year != 2 {
		t.Errorf("date.Year = %d, want 2", date.Date.Year)
	}
}

func TestStartDayOfYearForGameDay(t *testing.T) {
	start, err := StartDayOfYearForGameDay(10, 2, 5)
	if err != nil {
		t.Fatalf("StartDayOfYearForGameDay: %v", err)
	}
	if start != 26 {
		t.Errorf("start = %d, want 26", start)
	}

	start, err = StartDayOfYearForGameDay(2, 1, 1)
	if err != nil {
		t.Fatalf("StartDayOfYearForGameDay: %v", err)
	}
	if start != 352 {
		t.Errorf("start = %d, want 352", start)
	}
}
