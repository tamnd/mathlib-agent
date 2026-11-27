package stale

import (
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/drift"
)

const section = `---
title: Injective functions
kind: theory
modules:
  - Mathlib.Logic.Function.Basic
explains:
  E3: [Equiv]
  E2: [Function.Bijective]
  E1: auto
mathlib: aaaaaaaaaa
status: reviewed
reviewed_by: [a person]
---

## Injective functions

The theorem <?decl Function.Injective.comp?> composes. The span ` + "`f : α → β`" + ` is not a name, and ` + "`Equiv.symm`" + ` is one.
`

func TestID(t *testing.T) {
	for path, want := range map[string]string{
		"content/13-general-topology/05-compactness/02-metric.md": "V13.C05.S02",
		"content/13-general-topology/05-compactness/intro.md":     "",
		"content/13-general-topology/intro.md":                    "",
	} {
		if got := ID(path); got != want {
			t.Errorf("ID(%s) = %q, want %q", path, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	s, ok, err := Parse("content/01-a/03-b/01-c.md", section)
	if err != nil || !ok {
		t.Fatalf("Parse: %v %v", ok, err)
	}
	if s.ID != "V01.C03.S01" || s.Status != "reviewed" || s.Mathlib != "aaaaaaaaaa" {
		t.Errorf("section = %+v", s)
	}
	if strings.Join(s.Explains, " ") != "Equiv Function.Bijective" {
		t.Errorf("explains = %v", s.Explains)
	}
	if strings.Join(s.Names, " ") != "Equiv.symm Function.Injective.comp" {
		t.Errorf("names = %v", s.Names)
	}
	if _, ok, _ := Parse("x.md", "# no front matter\n"); ok {
		t.Error("a file with no front matter is a section")
	}
}

func TestFindAndApply(t *testing.T) {
	s, _, _ := Parse("content/01-a/03-b/01-c.md", section)
	chain := []drift.Record{
		{From: "aaaaaaaaaa", To: "bbbbbbbbbb", Date: "2026-12-01", Items: []drift.Item{
			{Class: drift.Renamed, Name: "Equiv", To: "Equiv'", Severity: drift.Rename},
			{Class: drift.DocChanged, Name: "Function.Bijective", Severity: drift.Info},
			{Class: drift.TypeChanged, Name: "Equiv.symm", Module: "Mathlib.Logic.Equiv.Defs", Severity: drift.Minor},
		}},
		{From: "bbbbbbbbbb", To: "cccccccccc", Date: "2026-12-08", Items: []drift.Item{
			{Class: drift.Removed, Name: "Other.thm", Module: "Mathlib.Logic.Function.Basic", Severity: drift.Breaking},
			{Class: drift.TypeChanged, Name: "Unrelated", Module: "Mathlib.Order.Basic", Severity: drift.Breaking},
		}},
	}
	m, ok := Find(s, chain)
	if !ok {
		t.Fatal("the section is not stale")
	}
	if len(m.Items) != 2 || m.Since != "bbbbbbbbbb" || m.From != "reviewed" || !m.Breaking() {
		t.Fatalf("mark = %+v", m)
	}
	if m.Items[0].Why != "the text uses Equiv.symm" || m.Items[1].Commit != "cccccccccc" {
		t.Errorf("items = %+v", m.Items)
	}
	if _, ok := Find(s, chain[:0]); ok {
		t.Error("no drift makes the section stale")
	}
	out, err := Apply(section, m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "status: stale\nstale_since: bbbbbbbbbb\nstale_from: reviewed\nreviewed_by:") {
		t.Errorf("front matter:\n%s", out)
	}
	// a second mark keeps the first stale_since and stale_from
	s2, _, _ := Parse("content/01-a/03-b/01-c.md", out)
	if s2.Status != "stale" || s2.From != "reviewed" || s2.Since != "bbbbbbbbbb" {
		t.Errorf("stale section = %+v", s2)
	}
	m2, _ := Find(s2, chain[1:])
	if m2.Since != "bbbbbbbbbb" || m2.From != "reviewed" {
		t.Errorf("second mark = %+v", m2)
	}
	out2, _ := Apply(out, m2)
	if out2 != out {
		t.Errorf("a second mark changes the file:\n%s", out2)
	}
}
