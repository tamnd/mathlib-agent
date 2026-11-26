package depth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/mathlib-agent/index"
)

func TestMainNames(t *testing.T) {
	doc := "# Compact sets\n\n## Main definitions\n\n* `IsCompact`: a set is compact.\n\n## Main results\n\n* `isCompact_iff`.\n\n## Notation\n\n* `Foo` is not main.\n"
	got := MainNames([]string{doc})
	want := []string{"IsCompact", "isCompact_iff"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("MainNames = %v; want %v", got, want)
	}
}

func TestLoadLists(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "1:\n  title  : The Irrationality of the Square Root of 2\n  decl   : irrational_sqrt_two\nAlgebra:\n  group: 'Group'\n"
	if err := os.WriteFile(filepath.Join(dir, "docs", "100.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLists(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(l["irrational_sqrt_two"]) != 1 || len(l["Group"]) != 1 {
		t.Errorf("lists = %v", l)
	}
	if _, ok := l["Algebra"]; ok {
		t.Error("a mapping key is in the lists")
	}
}

func TestCompute(t *testing.T) {
	ix := &index.Index{
		Decls: []index.Decl{
			{Name: "Group", Kind: "class", Module: "Mathlib.A"},
			{Name: "Foo.bar", Kind: "theorem", Module: "Mathlib.A"},
			{Name: "Foo.mem_bar", Kind: "theorem", Module: "Mathlib.A"},
			{Name: "Foo.simpy", Kind: "theorem", Module: "Mathlib.A", Attrs: []string{"simp"}},
			{Name: "Foo.inst", Kind: "instance", Module: "Mathlib.A"},
			{Name: "Foo.named", Kind: "theorem", Module: "Mathlib.A", Doc: "The Bolzano-Weierstrass theorem."},
			{Name: "Foo.used", Kind: "theorem", Module: "Mathlib.A"},
			{Name: "Foo.plain", Kind: "def", Module: "Mathlib.A"},
			{Name: "Foo.old", Kind: "theorem", Module: "Mathlib.A", Deprecated: &index.Deprecation{Target: "Foo.bar"}},
			{Name: "B.one", Kind: "theorem", Module: "Mathlib.B", Uses: []string{"Foo.used"}},
			{Name: "B.two", Kind: "theorem", Module: "Mathlib.B", Uses: []string{"Foo.used"}},
		},
		Modules: []index.Module{{Module: "Mathlib.A", Docs: []string{"## Main results\n\n* `Foo.bar`\n"}}},
	}
	got := Compute(ix, Lists{"Group": {"overview"}}, 0.02, 2)
	want := map[string]string{
		"Group": E3, "Foo.bar": E3, "Foo.mem_bar": E0, "Foo.simpy": E0, "Foo.inst": E0,
		"Foo.named": E2, "Foo.used": E2, "Foo.plain": E1, "Foo.old": E0, "B.one": E1,
	}
	for n, w := range want {
		if got[n].Depth != w {
			t.Errorf("%s: %s (%s); want %s", n, got[n].Depth, got[n].Reason, w)
		}
	}
}
