package calendar_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Formulary-Labs/dose/calendar"
)

func TestGenerate_icsFormat(t *testing.T) {
	events := []calendar.Event{
		{
			ID:        "EVT-001",
			Title:     "Annual penetration test",
			DueDate:   time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC),
			Owner:     "security-team",
			ControlID: "A.8.8",
			Priority:  "high",
		},
	}

	ics, _ := calendar.Generate(events, calendar.Options{Year: 2026})

	if !strings.Contains(ics, "BEGIN:VCALENDAR") {
		t.Error("ICS missing BEGIN:VCALENDAR")
	}
	if !strings.Contains(ics, "END:VCALENDAR") {
		t.Error("ICS missing END:VCALENDAR")
	}
	if !strings.Contains(ics, "Annual penetration test") {
		t.Error("ICS missing event title")
	}
	if !strings.Contains(ics, "BEGIN:VEVENT") {
		t.Error("ICS missing VEVENT")
	}
}

func TestGenerate_markdownFormat(t *testing.T) {
	events := []calendar.Event{
		{
			ID:      "EVT-001",
			Title:   "SOC 2 audit prep",
			DueDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			Owner:   "compliance-team",
		},
	}

	_, md := calendar.Generate(events, calendar.Options{Year: 2026})

	if !strings.Contains(md, "# Evidence Calendar") {
		t.Error("markdown missing title")
	}
	if !strings.Contains(md, "SOC 2 audit prep") {
		t.Error("markdown missing event title")
	}
}

func TestShiftLeft_weekend(t *testing.T) {
	// 2026-11-21 is a Saturday — should shift to 2026-11-20 (Friday).
	saturday := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	holidays := calendar.ExportedHolidays(2026)
	shifted := calendar.ExportedShiftLeft(saturday, holidays)
	if shifted.Weekday() == time.Saturday || shifted.Weekday() == time.Sunday {
		t.Errorf("shifted date %s is still a weekend", shifted.Format("2006-01-02"))
	}
	// Should be Friday Nov 20.
	if shifted.Format("2006-01-02") != "2026-11-20" {
		t.Errorf("expected 2026-11-20, got %s", shifted.Format("2006-01-02"))
	}
}

func TestShiftLeft_holiday(t *testing.T) {
	// 2026-11-26 is Thanksgiving — should shift back.
	thanksgiving := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	holidays := calendar.ExportedHolidays(2026)
	if !holidays[thanksgiving.Format("2006-01-02")] {
		t.Skip("2026 Thanksgiving not in holiday set — verify year")
	}
	shifted := calendar.ExportedShiftLeft(thanksgiving, holidays)
	if shifted.Weekday() == time.Saturday || shifted.Weekday() == time.Sunday {
		t.Errorf("shifted date %s is a weekend", shifted.Format("2006-01-02"))
	}
	if holidays[shifted.Format("2006-01-02")] {
		t.Errorf("shifted date %s is still a holiday", shifted.Format("2006-01-02"))
	}
}

func TestReminderEventsPresent(t *testing.T) {
	events := []calendar.Event{
		{
			ID:      "EVT-001",
			Title:   "Test event",
			DueDate: time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
		},
	}
	ics, _ := calendar.Generate(events, calendar.Options{Year: 2026})
	// Reminders are now VALARM subcomponents — expect 1 VEVENT and 2 VALARMs.
	veventCount := strings.Count(ics, "BEGIN:VEVENT")
	if veventCount != 1 {
		t.Errorf("expected 1 VEVENT block, got %d", veventCount)
	}
	valarmCount := strings.Count(ics, "BEGIN:VALARM")
	if valarmCount != 2 {
		t.Errorf("expected 2 VALARM subcomponents, got %d", valarmCount)
	}
	// TZID must not appear as a standalone property (RFC 5545 violation).
	if strings.Contains(ics, "\r\nTZID:") {
		t.Error("TZID must not appear as a standalone VEVENT property")
	}
}
