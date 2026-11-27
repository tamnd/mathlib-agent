package teach

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPaths(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "paths.yaml")
	write(t, p, "paths:\n  - id: U\n    title: Core\n    steps: [{volume: V04, chapters: [1, 4]}, {volume: V05}]\n  - id: A\n    after: [U]\n    steps: [{volume: V08}]\n")
	ps, err := LoadPaths(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[0].Steps[0].Range() != "Ch. 1 to 4" || ps[0].Steps[1].Range() != "" {
		t.Fatalf("paths = %+v", ps)
	}
	if !ps[0].Steps[0].Takes(4) || ps[0].Steps[0].Takes(5) || !ps[0].Steps[1].Takes(9) {
		t.Error("Takes is wrong")
	}
	write(t, p, "paths:\n  - id: A\n    after: [X]\n    steps: []\n")
	if _, err := LoadPaths(p); err == nil {
		t.Error("a path after a missing path loads")
	}
	write(t, p, "paths:\n  - id: A\n    steps: [{volume: V01, chapters: [3]}]\n")
	if _, err := LoadPaths(p); err == nil {
		t.Error("a step with one chapter number loads")
	}
	if ps, err := LoadPaths(filepath.Join(dir, "none.yaml")); err != nil || ps != nil {
		t.Errorf("missing file: %v %v", ps, err)
	}
}

func TestLoadBook(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "content/05-groups/intro.md"), "# Groups\n\nText.\n")
	write(t, filepath.Join(dir, "content/05-groups/04-cosets/intro.md"), "# Cosets and Lagrange\n")
	write(t, filepath.Join(dir, "content/05-groups/04-cosets/01-cosets.md"), "---\ntitle: Cosets\nstatus: drafted\n---\n\n## Cosets\n\n1. (A) not an exercise, it is before the part.\n\n### Exercises\n\n1. (A) List the cosets.\n2. (B) Show a thing. Hint: use $H$.\n3. (L) Prove it in Lean.\n\n### Notes and history\n\n4. (C) not an exercise.\n")
	write(t, filepath.Join(dir, "content/05-groups/04-cosets/02-order.md"), "---\ntitle: \"The order\"\nstatus: checked\n---\n\n### Exercises\n\n1. (C) Hard.\n")
	b, err := LoadBook(dir)
	if err != nil {
		t.Fatal(err)
	}
	v := b["V05"]
	if v == nil || v.Title != "Groups" || len(v.Chapters) != 1 {
		t.Fatalf("volume = %+v", v)
	}
	c := v.Chapters[0]
	if c.N != 4 || c.Title != "Cosets and Lagrange" || len(c.Sections) != 2 {
		t.Fatalf("chapter = %+v", c)
	}
	s := c.Sections[0]
	if s.ID != "V05.C04.S01" || s.Title != "Cosets" || s.Status != "drafted" || len(s.Exercises) != 3 {
		t.Fatalf("section = %+v", s)
	}
	if s.Exercises[1].Grade != "B" || s.Exercises[1].Text != "Show a thing. Hint: use $H$." {
		t.Errorf("exercise = %+v", s.Exercises[1])
	}
	if c.Sections[1].Title != "The order" {
		t.Errorf("quoted title = %q", c.Sections[1].Title)
	}
	n := v.Count()
	if n["A"] != 1 || n["B"] != 1 || n["C"] != 1 || n["L"] != 1 {
		t.Errorf("count = %v", n)
	}
	if b, err := LoadBook(""); err != nil || len(b) != 0 {
		t.Errorf("empty dir: %v %v", b, err)
	}
}
