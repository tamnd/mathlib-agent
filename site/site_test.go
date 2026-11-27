package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/tamnd/mathlib-agent/teach"
)

const testMap = `
volumes:
  - id: V04
    title: Order and lattices
    part: Part I
    level: undergraduate
    prefixes: [Mathlib/Order]
exceptions: {}
unmapped: fault
`

func testInput(t *testing.T) Input {
	t.Helper()
	vm, err := blueprint.ParseVolumes([]byte(testMap))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{
		Header: index.Header{Index: "declarations", Format: 1, Mathlib: "4472534b607bbff9621756cca1be340c60e7272a", Toolchain: "4.37.0-rc1", Date: "2026-11-26"},
		Decls: []index.Decl{
			{Name: "le_trans", Kind: "theorem", Module: "Mathlib.Order.Basic", Line: 20, Type: "a ≤ b → b ≤ c → a ≤ c", Doc: "The **transitivity** of `≤`. <script>x</script>"},
			{Name: "le_trans'", Kind: "theorem", Module: "Mathlib.Order.Basic", Line: 10, Type: "b ≤ c → a ≤ b → a ≤ c"},
			{Name: "old_trans", Kind: "theorem", Module: "Mathlib.Order.Basic", Line: 30, Deprecated: &index.Deprecation{Target: "le_trans", Since: "2026-01-01"}},
		},
		Modules: []index.Module{{Module: "Mathlib.Order.Basic", Docs: []string{"# Basic order\n\nText."}}},
	}
	depths := map[string]depth.Result{"le_trans": {Depth: depth.E3, Reason: "curated"}, "le_trans'": {Depth: depth.E1, Reason: "default"}}
	rec := drift.Record{Date: "2026-11-27", From: "aaaaaaaaaaaa", To: "bbbbbbbbbbbb", Counts: map[string]int{"added": 1}, Items: []drift.Item{{Class: "added", Name: "le_trans", Severity: "info"}}}
	return Input{Index: ix, Map: vm, Depths: depths, Drift: []drift.Record{rec}}
}

func TestBuildAndCheck(t *testing.T) {
	out := filepath.Join(t.TempDir(), "public")
	st, err := Build(testInput(t), Config{Out: out, URL: "https://mathlib.tamnd.com", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Modules != 1 || st.Decls != 2 {
		t.Fatalf("stats: %+v", st)
	}
	r, err := Check(out, "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Problems) > 0 {
		t.Fatalf("problems: %v", r.Problems)
	}
	page, err := os.ReadFile(filepath.Join(out, "module", "Mathlib.Order.Basic", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	for _, want := range []string{`id="le_trans"`, `id="le_trans&#39;"`, "<strong>transitivity</strong>", `href="/area/V04/"`, "#L20", "Deprecated aliases", `href="/decl/le_trans/"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the module page has no %q", want)
		}
	}
	if strings.Contains(s, "<script>x</script>") {
		t.Error("the docstring added raw HTML to the page")
	}
	if strings.Index(s, `id="le_trans&#39;"`) > strings.Index(s, `id="le_trans"`) {
		t.Error("the declarations are not in the order of the source")
	}
	for _, p := range []string{"drift/2026-11-27/index.html", "area/V04/index.html", "api/coverage.json", "assets/fonts/InterVariable.woff2"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s", p)
		}
	}
}

func TestShards(t *testing.T) {
	out := filepath.Join(t.TempDir(), "public")
	if _, err := Build(testInput(t), Config{Out: out}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"le_trans", "le_trans'", "old_trans"} {
		data, err := os.ReadFile(filepath.Join(out, "decl", "shard", Shard(n)+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if m[n] != "Mathlib.Order.Basic" {
			t.Errorf("the shard of %s gives %q", n, m[n])
		}
	}
}

// TestShardValue fixes the hash, so that a change of Shard that lookup.js does not follow fails. The FNV-1a hash of the empty string is the offset basis 0x811c9dc5.
func TestShardValue(t *testing.T) {
	if got := Shard(""); got != "c5" {
		t.Errorf("Shard(\"\") = %s, want c5 (0x811c9dc5)", got)
	}
}

func TestCheckFindsBrokenLink(t *testing.T) {
	out := filepath.Join(t.TempDir(), "public")
	if _, err := Build(testInput(t), Config{Out: out}); err != nil {
		t.Fatal(err)
	}
	bad := `<html><head><title>x</title></head><body><a href="/nowhere/">x</a></body></html>`
	if err := os.WriteFile(filepath.Join(out, "bad.html"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := Check(out, "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Problems) != 1 || !strings.Contains(r.Problems[0].Text, "/nowhere/") {
		t.Fatalf("problems: %v", r.Problems)
	}
}

func TestNum(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 335808: "335,808", -1234: "-1,234"} {
		if got := num(n); got != want {
			t.Errorf("num(%d) = %s, want %s", n, got, want)
		}
	}
}

func TestTeach(t *testing.T) {
	in := testInput(t)
	in.Paths = []teach.Path{
		{ID: "U", Title: "The undergraduate core", For: "an undergraduate degree", Steps: []teach.Step{{Volume: "V04", Chapters: []int{1, 1}}, {Volume: "V99"}}},
		{ID: "A", Title: "Algebra", For: "an algebraist", After: []string{"U"}, Steps: []teach.Step{{Volume: "V04"}}},
	}
	in.Book = teach.Book{"V04": &teach.Volume{ID: "V04", Title: "Order and lattices", Chapters: []*teach.Chapter{
		{N: 1, Title: "Orders", Sections: []*teach.Section{{ID: "V04.C01.S01", Title: "Preorders", Status: "drafted", Exercises: []teach.Exercise{{N: 1, Grade: "A", Text: "Show that ≤ is reflexive."}, {N: 2, Grade: "L", Text: "Prove it in Lean."}}}}},
		{N: 2, Title: "Monotone maps", Sections: []*teach.Section{{ID: "V04.C02.S01", Title: "Monotone", Status: "checked", Exercises: []teach.Exercise{{N: 1, Grade: "C", Text: "A hard one."}}}}},
	}}}
	out := filepath.Join(t.TempDir(), "public")
	if _, err := Build(in, Config{Out: out, URL: "https://mathlib.tamnd.com", Version: "test"}); err != nil {
		t.Fatal(err)
	}
	r, err := Check(out, "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Problems) > 0 {
		t.Fatalf("problems: %v", r.Problems)
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	u := read("teach/U/index.html")
	for _, want := range []string{"Ch. 1 to 1", "not written yet", "1. Orders", "1 drafted", `href="/teach/exercises/V04/"`} {
		if !strings.Contains(u, want) {
			t.Errorf("the page of path U has no %q", want)
		}
	}
	if strings.Contains(u, "Monotone maps") {
		t.Error("path U takes chapter 2 of V04")
	}
	if a := read("teach/A/index.html"); !strings.Contains(a, `href="/teach/U/"`) || !strings.Contains(a, "Monotone maps") {
		t.Error("the page of path A has no link to U or no chapter 2")
	}
	ex := read("teach/exercises/V04/index.html")
	for _, want := range []string{"Grade A: routine", "Grade C: hard", "Grade L: in Lean", "V04.C01.S01, exercise 1", "Show that ≤ is reflexive."} {
		if !strings.Contains(ex, want) {
			t.Errorf("the exercises page has no %q", want)
		}
	}
	if strings.Contains(ex, "Grade B") {
		t.Error("the exercises page has a grade with no exercise")
	}
	if h := read("teach/index.html"); !strings.Contains(h, `href="/teach/A/"`) || !strings.Contains(read("index.html"), `href="/teach/"`) {
		t.Error("no link to the courses or no menu item Teach")
	}
}
