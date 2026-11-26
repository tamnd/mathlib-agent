package edition

import (
	"path/filepath"
	"strings"
	"testing"
)

const sha = "fc4f59e577feb1a4956671bc04ac18aaaf8cd842"

func rec(tag string) Record {
	return Record{Edition: tag, Mathlib: sha, Lean: tag, TagDate: "2026-11-16", Opened: "2026-11-26"}
}

func TestAddAndSave(t *testing.T) {
	var l Log
	l, skipped, err := l.Add(rec("v4.36.0"))
	if err != nil || skipped != "" {
		t.Fatalf("add: %v %q", err, skipped)
	}
	if l[0].State != Open {
		t.Fatalf("state %q, want open", l[0].State)
	}
	l, skipped, err = l.Add(rec("v4.37.0"))
	if err != nil {
		t.Fatal(err)
	}
	if skipped != "v4.36.0" || l[0].State != Skipped || l[1].State != Open {
		t.Fatalf("the old edition did not close as skipped: %+v", l)
	}
	if _, _, err := l.Add(rec("v4.37.0")); err == nil {
		t.Fatal("a tag two times must fail")
	}
	path := filepath.Join(t.TempDir(), "records", "editions.json")
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 || back[1].Edition != "v4.37.0" {
		t.Fatalf("load gave %+v", back)
	}
}

func TestLoadMissing(t *testing.T) {
	l, err := Load(filepath.Join(t.TempDir(), "editions.json"))
	if err != nil || l != nil {
		t.Fatalf("a missing log must be empty: %v %v", l, err)
	}
}

func TestValidate(t *testing.T) {
	patch := rec("v4.36.1")
	twoOpen := Log{rec("v4.36.0"), rec("v4.37.0")}
	twoOpen[0].State, twoOpen[1].State = Open, Open
	released := rec("v4.36.0")
	released.State = Released
	badDate := rec("v4.36.0")
	badDate.State, badDate.Opened = Open, "26 Nov"
	for _, c := range []struct {
		name string
		log  Log
		want string
	}{
		{"patch tag", Log{patch}, "not of the form"},
		{"two open", twoOpen, "at most one"},
		{"released with no date", Log{released}, "release date"},
		{"bad date", Log{badDate}, "YYYY-MM-DD"},
		{"bad state", Log{rec("v4.36.0")}, "is not open"},
	} {
		err := c.log.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", c.name, err, c.want)
		}
	}
}

func TestDaysBetween(t *testing.T) {
	d, err := DaysBetween("2026-11-16", "2026-11-26")
	if err != nil || d != 10 {
		t.Fatalf("got %d %v, want 10", d, err)
	}
}
