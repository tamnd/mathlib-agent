package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/plan"
	"github.com/tamnd/mathlib-agent/stale"
)

func TestStale(t *testing.T) {
	mapDir, book := t.TempDir(), t.TempDir()
	rec := drift.Record{From: "aaaaaaaaaa", To: "bbbbbbbbbb", Date: "2026-12-01", Items: []drift.Item{
		{Class: drift.TypeChanged, Name: "Equiv.symm", Module: "Mathlib.Logic.Equiv.Defs", Severity: drift.Breaking},
	}}
	if err := writeJSON(filepath.Join(mapDir, "records", "drift", "2026-12-01.json"), rec); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{Volume: "V01", Chapters: []plan.Chapter{{ID: "C03", Sections: []plan.Section{{ID: "V01.C03.S01", Status: "reviewed"}, {ID: "V01.C03.S02", Status: "drafted"}}}}}
	if err := p.Save(plan.Path(mapDir, "V01")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(book, "content", "01-the-language", "03-functions")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	sec := "---\ntitle: Equivalences\nmodules: []\nexplains:\n  E3: [Equiv.symm]\nmathlib: aaaaaaaaaa\nstatus: reviewed\n---\n\nText.\n"
	other := "---\ntitle: Other\nmodules: []\nexplains: {}\nmathlib: aaaaaaaaaa\nstatus: drafted\n---\n\nThe name `Nat.succ`.\n"
	if err := os.WriteFile(filepath.Join(dir, "01-equivalences.md"), []byte(sec), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02-other.md"), []byte(other), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"stale", "-map", mapDir, "-book", book}, &out, &errb); code != 0 {
		t.Fatalf("exit code %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "2 sections at Mathlib bbbbbbbbbb, 1 stale") || !strings.Contains(out.String(), "V01.C03.S01  breaking") {
		t.Errorf("output:\n%s", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "01-equivalences.md")); string(b) != sec {
		t.Error("a run with no -apply writes the section")
	}
	out.Reset()
	if code := run([]string{"stale", "-map", mapDir, "-book", book, "-apply", "-date", "2026-12-02"}, &out, &errb); code != 0 {
		t.Fatalf("exit code %d: %s", code, errb.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "01-equivalences.md"))
	if !strings.Contains(string(b), "status: stale\nstale_since: bbbbbbbbbb\nstale_from: reviewed\n") {
		t.Errorf("section:\n%s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "02-other.md")); string(b) != other {
		t.Error("the run writes a section that is not stale")
	}
	q, err := plan.Load(plan.Path(mapDir, "V01"))
	if err != nil || q.Chapters[0].Sections[0].Status != "stale" || q.Chapters[0].Sections[1].Status != "drafted" {
		t.Errorf("plan: %+v %v", q, err)
	}
	r, err := stale.Load(stale.Path(mapDir))
	if err != nil || len(r.Marks) != 1 || r.Marks[0].Items[0].Commit != "bbbbbbbbbb" || r.Date != "2026-12-02" {
		t.Errorf("record: %+v %v", r, err)
	}
	if code := run([]string{"stale", "-map", mapDir}, &out, &errb); code != 2 {
		t.Errorf("no -book: exit code %d, want 2", code)
	}
}
