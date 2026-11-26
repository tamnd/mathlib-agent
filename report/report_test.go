package report

import (
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/agent"
	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/edition"
)

func TestWeek(t *testing.T) {
	for _, c := range []struct{ week, mon, sun string }{
		{"2026-W48", "2026-11-23", "2026-11-29"},
		{"2027-W01", "2027-01-04", "2027-01-10"},
		{"2026-W53", "2026-12-28", "2027-01-03"},
	} {
		mon, sun, err := Week(c.week)
		if err != nil {
			t.Fatal(err)
		}
		if day(mon) != c.mon || day(sun) != c.sun {
			t.Errorf("%s: got %s to %s, want %s to %s", c.week, day(mon), day(sun), c.mon, c.sun)
		}
		if WeekOf(mon) != c.week {
			t.Errorf("WeekOf(%s) = %s", day(mon), WeekOf(mon))
		}
	}
	for _, bad := range []string{"2027-3", "2027-W54", "2027-W00", "W03"} {
		if _, _, err := Week(bad); err == nil {
			t.Errorf("%s: no error", bad)
		}
	}
}

func input() Input {
	return Input{
		Tool: "mlib test",
		Drift: []drift.Record{
			{Date: "2026-10-01", From: "aaaaaaaaaaaa", To: "bbbbbbbbbbbb", Counts: map[string]int{"added": 1}},
			{Date: "2026-11-24", From: "bbbbbbbbbbbb", To: "cccccccccccc", Counts: map[string]int{"renamed": 1, "added": 2},
				Items: []drift.Item{{Class: drift.Renamed, Name: "Foo.old", To: "Foo.new", Severity: drift.Rename}}},
		},
		Editions: edition.Log{
			{Edition: "v4.35.0", Mathlib: "bbbbbbbbbbbb", TagDate: "2026-10-01", Opened: "2026-10-02", Released: "2026-10-10", State: "released"},
			{Edition: "v4.36.0", Mathlib: "cccccccccccc", TagDate: "2026-11-16", Opened: "2026-11-26", State: "open"},
		},
		Runs:   []*agent.Run{{ID: "r1", Role: "writer", Date: "2026-11-25", Status: agent.Passed}, {ID: "r2", Role: "writer", Date: "2026-11-26", Status: agent.Failed}},
		Ledger: []agent.Entry{{Date: "2026-11-25", Run: "r1", Role: "writer", Cost: 1.5, TokensIn: 100, TokensOut: 50}},
	}
}

func TestWeekly(t *testing.T) {
	got, err := Weekly(input(), "2026-W48")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Weekly report 2026-W48",
		"The last sync ran on 2026-11-24, from bbbbbbbbbb to cccccccccc. At the end of the week it is 5 days old.",
		"| added | 2 |",
		"- `Foo.old` to `Foo.new` (decl)",
		"| writer | 0 | 0 | 1 | 1 | 1.50 |",
		"The spend is 1.50 dollars for 150 tokens.",
		"- v4.36.0: opened on 2026-11-26.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report has no %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Alert") {
		t.Error("a sync of 5 days gives no alert")
	}
	got, err = Weekly(input(), "2027-W02")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "**Alert.**") || !strings.Contains(got, "No role ran.") {
		t.Errorf("a late week must give the alert and no runs:\n%s", got)
	}
}

func TestEdition(t *testing.T) {
	got, err := Edition(input(), "v4.36.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Edition v4.36.0", "| State | open |", "The drift since v4.35.0 comes from 1 drift records.", "| renamed | 1 |"} {
		if !strings.Contains(got, want) {
			t.Errorf("the report has no %q:\n%s", want, got)
		}
	}
	got, err = Edition(input(), "v4.35.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "9 days after the tag, within the 14 days") || !strings.Contains(got, "This is the first edition") {
		t.Errorf("first edition:\n%s", got)
	}
	if _, err := Edition(input(), "v9.0.0"); err == nil {
		t.Error("an unknown edition gives no error")
	}
}
