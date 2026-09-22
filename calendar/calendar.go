// Package calendar implements RFC 5545 iCalendar generation for dose.
// It applies shift-left working-day scheduling with US federal holiday
// awareness, as specified in functions/calendar-output-spec.md.
package calendar

import (
	"fmt"
	"strings"
	"time"
)

// Event is a single evidence collection event.
type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	DueDate     time.Time `json:"due_date"`
	Owner       string    `json:"owner,omitempty"`
	ControlID   string    `json:"control_id,omitempty"`
	Category    string    `json:"category,omitempty"` // evidence, review, submission, etc.
	Priority    string    `json:"priority,omitempty"`
}

// Options configures calendar generation.
type Options struct {
	Timezone   string // IANA timezone, default: America/New_York
	WorkStart  string // HH:MM, default: 09:00
	WorkEnd    string // HH:MM, default: 17:00
	GapMinutes int    // Minimum gap in minutes between scheduled events (0 = none)
	Year       int    // Reference year for holiday calculation
}

func (o *Options) defaults() {
	if o.Timezone == "" {
		o.Timezone = "America/New_York"
	}
	if o.WorkStart == "" {
		o.WorkStart = "09:00"
	}
	if o.WorkEnd == "" {
		o.WorkEnd = "17:00"
	}
	if o.Year == 0 {
		o.Year = time.Now().Year()
	}
}

// ExportedShiftLeft exposes shiftLeft for testing.
func ExportedShiftLeft(t time.Time, holidays map[string]bool) time.Time {
	return shiftLeft(t, holidays)
}

// ExportedHolidays exposes usHolidays for testing.
func ExportedHolidays(year int) map[string]bool {
	return usHolidays(year)
}

// Generate produces an RFC 5545 iCalendar string and a markdown event list
// from a slice of Evidence Events.
func Generate(events []Event, opts Options) (ics string, md string) {
	opts.defaults()

	holidays := usHolidays(opts.Year)

	var icsEvents []string
	var mdLines []string

	mdLines = append(mdLines, "# Evidence Calendar\n")
	mdLines = append(mdLines, "| Date | Title | Owner | Control | Priority |")
	mdLines = append(mdLines, "|---|---|---|---|---|")

	for _, e := range events {
		// Shift due date to a working day.
		scheduled := shiftLeft(e.DueDate, holidays)

		// Reminder offsets: 1 month and 1 week before.
		oneMonth := shiftLeft(scheduled.AddDate(0, -1, 0), holidays)
		oneWeek := shiftLeft(scheduled.Add(-7*24*time.Hour), holidays)

		// Main event with embedded VALARMs (RFC 5545 §3.6.6).
		icsEvents = append(icsEvents, icsEvent(e, scheduled, []alarmSpec{
			{trigger: scheduled.Sub(oneMonth) * -1, description: "1-month preparation reminder: " + e.Title},
			{trigger: scheduled.Sub(oneWeek) * -1, description: "1-week preparation reminder: " + e.Title},
		}))

		mdLines = append(mdLines, fmt.Sprintf("| %s | %s | %s | %s | %s |",
			scheduled.Format("2006-01-02"), e.Title, e.Owner, e.ControlID, e.Priority))
	}

	var ics_b strings.Builder
	ics_b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Formulary-Labs/dose//EN\r\nCALSCALE:GREGORIAN\r\n")
	for _, ev := range icsEvents {
		ics_b.WriteString(ev)
	}
	ics_b.WriteString("END:VCALENDAR\r\n")

	return ics_b.String(), strings.Join(mdLines, "\n") + "\n"
}

// alarmSpec describes a VALARM trigger relative to the event's DTSTART.
// duration is negative (e.g. -168h = 1 week before).
type alarmSpec struct {
	trigger     time.Duration // negative = before event
	description string
}

// icsEvent produces a single VEVENT block with embedded VALARMs.
// All-day events use DATE value type and do not carry TZID.
func icsEvent(e Event, date time.Time, alarms []alarmSpec) string {
	var b strings.Builder
	b.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s@formulary-labs\r\n", e.ID)
	fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", date.Format("20060102"))
	fmt.Fprintf(&b, "DTEND;VALUE=DATE:%s\r\n", date.AddDate(0, 0, 1).Format("20060102"))
	fmt.Fprintf(&b, "SUMMARY:%s\r\n", foldLine(icsEscape(e.Title)))
	if e.Description != "" {
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", foldLine(icsEscape(e.Description)))
	}
	if e.Category != "" {
		fmt.Fprintf(&b, "CATEGORIES:%s\r\n", icsEscape(e.Category))
	}
	// Embed VALARM subcomponents for reminders.
	for _, a := range alarms {
		b.WriteString("BEGIN:VALARM\r\n")
		b.WriteString("ACTION:DISPLAY\r\n")
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", foldLine(icsEscape(a.description)))
		// Trigger as negative duration relative to DTSTART.
		fmt.Fprintf(&b, "TRIGGER:%s\r\n", formatDuration(a.trigger))
		b.WriteString("END:VALARM\r\n")
	}
	b.WriteString("END:VEVENT\r\n")
	return b.String()
}

// formatDuration formats a time.Duration as an RFC 5545 DURATION value.
// Only days and hours are used; sub-day precision is rounded.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "PT0S"
	}
	neg := ""
	if d < 0 {
		neg = "-"
		d = -d
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 && hours == 0 {
		return fmt.Sprintf("%sP%dD", neg, days)
	}
	if days > 0 {
		return fmt.Sprintf("%sP%dDT%dH", neg, days, hours)
	}
	return fmt.Sprintf("%sPT%dH", neg, int(d.Hours()))
}

// icsEscape escapes special characters in iCalendar text values.
func icsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// foldLine wraps long iCalendar property values per RFC 5545 (75 octet limit).
func foldLine(s string) string {
	if len(s) <= 70 {
		return s
	}
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i += 70 {
		end := i + 70
		if end > len(runes) {
			end = len(runes)
		}
		if i > 0 {
			b.WriteString("\r\n ")
		}
		b.WriteString(string(runes[i:end]))
	}
	return b.String()
}

// shiftLeft moves a date to the nearest prior working day if it falls on a
// weekend or holiday. Per the spec: never schedule on weekends or blocked
// holidays.
func shiftLeft(t time.Time, holidays map[string]bool) time.Time {
	// Normalize to date-only.
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	for {
		key := t.Format("2006-01-02")
		wd := t.Weekday()
		if wd != time.Saturday && wd != time.Sunday && !holidays[key] {
			return t
		}
		t = t.AddDate(0, 0, -1)
	}
}

// usHolidays returns a set of US federal + commonly observed holiday dates
// for the given year, formatted as "YYYY-MM-DD".
func usHolidays(year int) map[string]bool {
	h := map[string]bool{}

	add := func(t time.Time) {
		t = nearestWeekday(t)
		h[t.Format("2006-01-02")] = true
	}
	addExact := func(t time.Time) {
		h[t.Format("2006-01-02")] = true
	}

	// New Year's Day.
	add(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC))
	// MLK Day — 3rd Monday in January.
	addExact(nthWeekdayOfMonth(year, 1, time.Monday, 3))
	// Presidents' Day — 3rd Monday in February.
	addExact(nthWeekdayOfMonth(year, 2, time.Monday, 3))
	// Memorial Day — last Monday in May.
	addExact(lastWeekdayOfMonth(year, 5, time.Monday))
	// Juneteenth.
	add(time.Date(year, 6, 19, 0, 0, 0, 0, time.UTC))
	// Independence Day.
	add(time.Date(year, 7, 4, 0, 0, 0, 0, time.UTC))
	// Labor Day — 1st Monday in September.
	addExact(nthWeekdayOfMonth(year, 9, time.Monday, 1))
	// Columbus Day — 2nd Monday in October.
	addExact(nthWeekdayOfMonth(year, 10, time.Monday, 2))
	// Veterans Day.
	add(time.Date(year, 11, 11, 0, 0, 0, 0, time.UTC))
	// Thanksgiving — 4th Thursday in November.
	thanksgiving := nthWeekdayOfMonth(year, 11, time.Thursday, 4)
	addExact(thanksgiving)
	// Day After Thanksgiving.
	addExact(thanksgiving.AddDate(0, 0, 1))
	// Christmas Eve.
	addExact(time.Date(year, 12, 24, 0, 0, 0, 0, time.UTC))
	// Christmas Day.
	add(time.Date(year, 12, 25, 0, 0, 0, 0, time.UTC))
	// New Year's Eve.
	addExact(time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC))

	return h
}

// nearestWeekday shifts a date to the nearest weekday if it falls on a weekend.
func nearestWeekday(t time.Time) time.Time {
	switch t.Weekday() {
	case time.Saturday:
		return t.AddDate(0, 0, -1)
	case time.Sunday:
		return t.AddDate(0, 0, 1)
	}
	return t
}

// nthWeekdayOfMonth returns the nth occurrence of wd in year/month.
func nthWeekdayOfMonth(year int, month time.Month, wd time.Weekday, n int) time.Time {
	t := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	count := 0
	for {
		if t.Weekday() == wd {
			count++
			if count == n {
				return t
			}
		}
		t = t.AddDate(0, 0, 1)
		if t.Month() != month {
			break
		}
	}
	return t
}

// lastWeekdayOfMonth returns the last occurrence of wd in year/month.
func lastWeekdayOfMonth(year int, month time.Month, wd time.Weekday) time.Time {
	// Start from the last day and work backwards.
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC) // last day of month
	for t.Weekday() != wd {
		t = t.AddDate(0, 0, -1)
	}
	return t
}
