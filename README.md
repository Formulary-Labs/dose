# dose

Evidence collection calendar generator for compliance programs.

```bash
go get github.com/Formulary-Labs/dose
```

## What it does

`dose` converts a list of evidence collection events into an RFC 5545 iCalendar file (`.ics`) and a Markdown table. It schedules each event on a working day by shifting due dates that fall on weekends or US federal holidays to the nearest prior working day. Each event also produces two reminder events — one month and one week in advance.

The result is a calendar file you can import directly into Google Calendar, Outlook, or Apple Calendar, and a Markdown table you can include in a program status document.

## Usage

```go
import "github.com/Formulary-Labs/dose/calendar"

ics, md, err := calendar.Generate([]calendar.Event{
    {
        ID:          "EV-001",
        Title:       "Access log review",
        Description: "Quarterly access log review for logical access controls",
        DueDate:     time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
        Owner:       "security-team",
        ControlID:   "A.8.3",
        Category:    "access-control",
        Priority:    "high",
    },
    {
        ID:       "EV-002",
        Title:    "Vendor security questionnaire",
        DueDate:  time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC), // Thanksgiving — shifts left
        Owner:    "vendor-management",
        Priority: "medium",
    },
}, calendar.Options{
    Timezone:  "America/New_York",
    WorkStart: "09:00",
    WorkEnd:   "17:00",
    Year:      2026,
})
```

`Options` defaults: `America/New_York`, `09:00`–`17:00`, current year.

## Shift-left scheduling

`dose` moves any due date that falls on a weekend or US federal holiday to the nearest prior working day. Evidence deadlines land on days when someone can actually act on them.

Holidays covered:

New Year's Day · MLK Day · Presidents' Day · Memorial Day · Juneteenth · Independence Day · Labor Day · Columbus Day · Veterans Day · Thanksgiving · Day after Thanksgiving · Christmas Eve · Christmas Day · New Year's Eve

The shift is computed per-event. If November 27 is Thanksgiving, the event moves to November 26. The reminder events shift independently by the same logic.

## What each event produces in the calendar

Each evidence item generates three calendar entries in the `.ics` output:

1. Main event on the adjusted due date
2. `PREP:` reminder event one month prior (also adjusted to a working day)
3. `PREP:` reminder event one week prior (also adjusted to a working day)

All three entries share the event `ID` as a prefix, so they group correctly in calendar applications.

## Output

### iCalendar

Standard RFC 5545 format. `PRODID` is `Formulary-Labs/dose`. Import the `.ics` file directly into any calendar application.

```
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Formulary-Labs/dose//EN
CALSCALE:GREGORIAN
BEGIN:VEVENT
...
END:VEVENT
END:VCALENDAR
```

### Markdown table

```markdown
| Date | Title | Owner | Control | Priority |
|---|---|---|---|---|
| 2026-09-30 | Access log review | security-team | A.8.3 | high |
```

Dates in the Markdown table reflect the shifted working-day date, not the original due date.

## Pipeline context

`dose` runs during program intake or at the start of an audit cycle, when evidence windows are being planned. The `.ics` output goes to the program manager's calendar. The Markdown table feeds `exhibit` as the evidence calendar section of the auditor dashboard.

```bash
dose --events evidence-events.json --year 2026 > evidence-calendar.ics
```

## License

Apache License 2.0
