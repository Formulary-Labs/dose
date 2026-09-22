// dose produces evidence collection calendars from evidence window data.
//
// Usage:
//
//	dose [flags] --events <events.json>
//
// Outputs an RFC 5545 .ics file and/or a markdown event list.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Formulary-Labs/dose/calendar"
	"github.com/Formulary-Labs/substrate/exit"
	"github.com/Formulary-Labs/substrate/format"
	"github.com/Formulary-Labs/substrate/provenance"
)

const version = "0.1.0"

func main() {
	var (
		eventsFlag    = flag.String("events", "", "Path to JSON events file [{id, title, due_date, owner, ...}]")
		programFlag   = flag.String("program", "", "Program slug for provenance logging")
		fmtFlag       = flag.String("format", "ics", "Output format: ics (default), md, both")
		outputFlag    = flag.String("output", "", "Output file path (stdout if empty, or base path for --format both)")
		timezoneFlag  = flag.String("timezone", "America/New_York", "IANA timezone")
		yearFlag      = flag.Int("year", time.Now().Year(), "Reference year for holiday calculation")
		workStartFlag = flag.String("work-start", "09:00", "Work day start HH:MM (default 09:00)")
		workEndFlag   = flag.String("work-end", "17:00", "Work day end HH:MM (default 17:00)")
		gapFlag       = flag.Int("gap-minutes", 0, "Minimum gap in minutes between scheduled events (0 = none)")
		versionFlag   = flag.Bool("version", false, "Print version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dose version %s\n", version)
		os.Exit(exit.OK)
	}

	if *eventsFlag == "" {
		fmt.Fprintln(os.Stderr, `{"error": "--events is required", "code": 2}`)
		flag.Usage()
		os.Exit(exit.ToolError)
	}

	data, err := os.ReadFile(*eventsFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": %q, "code": 2}`+"\n", err.Error())
		os.Exit(exit.ToolError)
	}

	var events []calendar.Event
	if err := json.Unmarshal(data, &events); err != nil {
		fmt.Fprintf(os.Stderr, `{"error": "parsing events: %v", "code": 2}`+"\n", err)
		os.Exit(exit.ToolError)
	}

	opts := calendar.Options{
		Timezone:   *timezoneFlag,
		Year:       *yearFlag,
		WorkStart:  *workStartFlag,
		WorkEnd:    *workEndFlag,
		GapMinutes: *gapFlag,
	}

	ics, md := calendar.Generate(events, opts)

	f, err := format.Parse(*fmtFlag)
	if err != nil && *fmtFlag != "both" {
		f = "ics"
	}

	switch {
	case *fmtFlag == "both":
		writeOutput(ics, *outputFlag+".ics")
		writeOutput(md, *outputFlag+".md")
	case f == "md" || *fmtFlag == "md":
		writeOutput(md, *outputFlag)
	default:
		writeOutput(ics, *outputFlag)
	}

	if *programFlag != "" {
		_ = provenance.Write("logs/provenance.jsonl", provenance.Entry{
			Spec:        "functions/calendar-output-spec.md",
			Output:      *eventsFlag,
			OutputType:  "other",
			Program:     *programFlag,
			Purpose:     fmt.Sprintf("dose: %d evidence events, shift-left scheduled", len(events)),
			Reusability: provenance.Instance,
			QualityGate: provenance.Pass,
			Tool:        "dose",
			ToolVersion: version,
		})
	}
}

func writeOutput(content, path string) {
	if path == "" {
		fmt.Print(content)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(exit.ToolError)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
}

func usage() {
	fmt.Fprintln(os.Stderr, `dose — evidence collection calendar

Usage:
  dose [flags] --events <events.json>

Flags:
  --events string     Path to JSON events file (required)
  --program string    Program slug for provenance logging
  --format string     Output format: ics (default), md, both
  --output string     Output file path (stdout if empty; base path for --format both)
  --timezone string   IANA timezone (default: America/New_York)
  --year int          Reference year for holiday calculation
  --version           Print version and exit

Events JSON format:
  [
    {
      "id": "EVIDENCE-001",
      "title": "Annual penetration test",
      "due_date": "2026-11-15T00:00:00Z",
      "owner": "security-team",
      "control_id": "A.8.8",
      "priority": "high"
    }
  ]

Examples:
  dose --events events.json --format ics > calendar.ics
  dose --events events.json --format md
  dose --events events.json --format both --output evidence-calendar`)
}
