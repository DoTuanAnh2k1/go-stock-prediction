// Package market_calendar checks whether a financial market is open on a given date.
// Logic mirrors prediction/src/utils/market_calendar.py — no external dependencies.
package market_calendar

import (
	"time"
)

var ict = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
var et = time.FixedZone("America/New_York", -4*3600) // EDT (summer); EST is -5

// IsMarketOpen returns true if the given market accepts trades at time `when` (ICT naive → converted to ET).
// CRYPTO always returns true. GOLD follows Mon-Fri (no holiday check). NASDAQ/SP500 follow NYSE calendar.
func IsMarketOpen(market string, when time.Time) bool {
	switch market {
	case "CRYPTO":
		return true
	case "GOLD":
		et := toET(when)
		return et.Weekday() >= time.Monday && et.Weekday() <= time.Friday
	case "NASDAQ", "NASDAQ100", "SP500":
		return isNYSETradingDay(toET(when))
	default:
		return true
	}
}

// IsMarketOpenNow returns IsMarketOpen using the current ICT wall clock.
func IsMarketOpenNow(market string) bool {
	return IsMarketOpen(market, time.Now().In(ict))
}

// toET converts any time to its ET (EDT, UTC-4) representation.
// Go stores time internally as UTC; .In() applies the target offset without
// any additional arithmetic — correct regardless of the input timezone.
func toET(t time.Time) time.Time {
	return t.In(et)
}

func isNYSETradingDay(t time.Time) bool {
	d := t.Weekday()
	if d == time.Saturday || d == time.Sunday {
		return false
	}
	return !isNYSEHoliday(t)
}

func isNYSEHoliday(t time.Time) bool {
	y, m, day := t.Year(), t.Month(), t.Day()
	holidays := nyseHolidays(y)
	for _, h := range holidays {
		hy, hm, hd := h.Date()
		if hy == y && hm == m && hd == day {
			return true
		}
	}
	return false
}

func nyseHolidays(year int) []time.Time {
	var h []time.Time

	// Fixed holidays with observed rule
	for _, md := range [][2]int{{1, 1}, {6, 19}, {7, 4}, {12, 25}} {
		h = append(h, observed(time.Date(year, time.Month(md[0]), md[1], 0, 0, 0, 0, time.UTC)))
	}

	// Nth-weekday holidays
	h = append(h, nthWeekday(year, time.January, time.Monday, 3))   // MLK Day
	h = append(h, nthWeekday(year, time.February, time.Monday, 3))  // Presidents' Day
	h = append(h, lastWeekday(year, time.May, time.Monday))         // Memorial Day
	h = append(h, nthWeekday(year, time.September, time.Monday, 1)) // Labor Day
	h = append(h, nthWeekday(year, time.November, time.Thursday, 4)) // Thanksgiving
	h = append(h, easterSunday(year).AddDate(0, 0, -2))             // Good Friday

	return h
}

// observed applies NYSE's observed-holiday rule: Sun→Mon, Sat→Fri.
func observed(d time.Time) time.Time {
	switch d.Weekday() {
	case time.Sunday:
		return d.AddDate(0, 0, 1)
	case time.Saturday:
		return d.AddDate(0, 0, -1)
	}
	return d
}

func nthWeekday(year int, month time.Month, wd time.Weekday, n int) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(wd) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+(n-1)*7)
}

func lastWeekday(year int, month time.Month, wd time.Weekday) time.Time {
	// Last day of month
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	offset := (int(last.Weekday()) - int(wd) + 7) % 7
	return last.AddDate(0, 0, -offset)
}

// easterSunday computes Easter using the Anonymous Gregorian algorithm.
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
