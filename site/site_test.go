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
