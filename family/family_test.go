package family

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/index"
)

const records = `- id: F-map-op
  pattern: { name: "map_{op}", typeHead: "FunLike" }
  ops: [add, mul]
  text: "A bundled map of a class that preserves {op} sends {op} to {op}."
  depth: E0
- id: F-coe-mk
  pattern: { name: "*.coe_mk", attr: simp }
  text: "The coercion of a structure built with its constructor is the function given to the constructor."
  depth: E0
- id: F-deprecated
  pattern: { deprecated: true }
  text: "A deprecated alias keeps an old name."
  depth: E0
- id: F-to-additive
  pattern: { twin: to_additive }
  text: "The additive twin of a multiplicative declaration."
  depth: E0
`

func load(t *testing.T, s string) []*Family {
	t.Helper()
	path := filepath.Join(t.TempDir(), "families.yaml")
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func testIndex() *index.Index {
	return &index.Index{Decls: []index.Decl{
		{Name: "map_add", Kind: "theorem", Type: "f (x + y) = f x + f y", Uses: []string{"FunLike"}},
		{Name: "map_mul", Kind: "theorem", Type: "f (x * y) = f x * f y", Uses: []string{"FunLike"}},
		{Name: "map_sub", Kind: "theorem", Uses: []string{"FunLike"}},
		{Name: "Foo.map_add", Kind: "theorem", Uses: []string{"Foo"}},
		{Name: "MonoidHom.coe_mk", Kind: "theorem", Attrs: []string{"simp"}},
		{Name: "Foo.Bar.coe_mk", Kind: "theorem"},
		{Name: "Old.name", Kind: "theorem", Deprecated: &index.Deprecation{Target: "New.name"}},
		{Name: "mul_one", Kind: "theorem", Type: "a * 1 = a"},
		{Name: "add_zero", Kind: "theorem", Type: "a + 0 = a"},
		{Name: "Polynomial.divByMonic_one", Kind: "theorem", Type: "p /ₘ 1 = p"},
		{Name: "Polynomial.divByMonic_zero", Kind: "theorem", Type: "p /ₘ 0 = 0"},
	}}
}

func TestMatch(t *testing.T) {
	fs := load(t, records)
	r := Match(testIndex(), fs)
	want := map[string]string{
		"map_add":          "F-map-op",
		"map_mul":          "F-map-op",
		"MonoidHom.coe_mk": "F-coe-mk",
		"Old.name":         "F-deprecated",
		"add_zero":         "F-to-additive",
	}
	for n, id := range want {
		if r.Member[n] != id {
			t.Errorf("%s: family %q, want %q", n, r.Member[n], id)
		}
	}
	for _, n := range []string{"map_sub", "Foo.map_add", "Foo.Bar.coe_mk", "mul_one", "Polynomial.divByMonic_zero"} {
		if id, ok := r.Member[n]; ok {
			t.Errorf("%s: family %q, want none", n, id)
		}
	}
	if r.Twin["add_zero"] != "mul_one" {
		t.Errorf("the source of add_zero is %q, want mul_one", r.Twin["add_zero"])
	}
	if r.Count["F-map-op"] != 2 || len(r.Member) != 5 {
		t.Errorf("counts %v, %d members", r.Count, len(r.Member))
	}
	rec := Summarize(testIndex(), fs, r, "2026-11-26", 1)
	if rec.Members != 5 || len(rec.Families) != 4 || len(rec.Families[0].Sample) != 1 || rec.Families[0].Sample[0] != "map_add" {
		t.Errorf("summary %+v", rec)
	}
}

func TestCheck(t *testing.T) {
	for _, c := range []struct{ yaml, want string }{
		{"- {id: map-op, pattern: {kind: theorem}, text: x, depth: E0}", "form F-name"},
		{"- {id: F-a, pattern: {kind: theorem}, text: x, depth: E0}\n- {id: F-a, pattern: {kind: theorem}, text: x, depth: E0}", "two times"},
		{"- {id: F-a, pattern: {kind: theorem}, depth: E0}", "no text"},
		{"- {id: F-a, pattern: {kind: theorem}, text: x, depth: E1}", "depth"},
		{"- {id: F-a, pattern: {}, text: x, depth: E0}", "no condition"},
		{"- {id: F-a, pattern: {name: \"map_{op}\"}, text: x, depth: E0}", "no ops"},
		{"- {id: F-a, pattern: {twin: to_mul}, text: x, depth: E0}", "to_additive or to_dual"},
	} {
		path := filepath.Join(t.TempDir(), "families.yaml")
		if err := os.WriteFile(path, []byte(c.yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got %v, want an error with %q", c.yaml, err, c.want)
		}
	}
}
