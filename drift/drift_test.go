package drift

import (
	"testing"

	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
)

func TestCompare(t *testing.T) {
	old := &index.Index{
		Header: index.Header{Mathlib: "a", Toolchain: "4.37.0"},
		Decls: []index.Decl{
			{Name: "keep", Module: "Mathlib.A", TypeHash: "1"},
			{Name: "gone", Module: "Mathlib.A", TypeHash: "2"},
			{Name: "oldName", Module: "Mathlib.A", TypeHash: "3"},
			{Name: "aliased", Module: "Mathlib.A", TypeHash: "4"},
			{Name: "typed", Module: "Mathlib.A", TypeHash: "5"},
			{Name: "main", Module: "Mathlib.A", TypeHash: "6"},
			{Name: "moves", Module: "Mathlib.A", TypeHash: "7", Doc: "x"},
			{Name: "valued", Module: "Mathlib.A", TypeHash: "8", ValueHash: "v1"},
		},
		Modules: []index.Module{{Module: "Mathlib.A"}},
	}
	cur := &index.Index{
		Header: index.Header{Mathlib: "b", Toolchain: "4.37.0"},
		Decls: []index.Decl{
			{Name: "keep", Module: "Mathlib.A", TypeHash: "1"},
			{Name: "newName", Module: "Mathlib.A", TypeHash: "3"},
			{Name: "aliased", Module: "Mathlib.A", TypeHash: "4", Deprecated: &index.Deprecation{Target: "aliasTarget", Since: "2026-11-01"}, Attrs: []string{"deprecated"}},
			{Name: "aliasTarget", Module: "Mathlib.A", TypeHash: "4"},
			{Name: "typed", Module: "Mathlib.A", TypeHash: "50"},
			{Name: "main", Module: "Mathlib.A", TypeHash: "60"},
			{Name: "moves", Module: "Mathlib.B", TypeHash: "7", Doc: "y", Attrs: []string{"simp"}},
			{Name: "valued", Module: "Mathlib.A", TypeHash: "8", ValueHash: "v2"},
			{Name: "fresh", Module: "Mathlib.B", TypeHash: "9"},
		},
		Modules: []index.Module{{Module: "Mathlib.B"}},
	}
	r := Compare(old, cur, Options{Depths: map[string]depth.Result{"main": {Depth: depth.E3}}, Added: true})
	want := map[string]int{Removed: 1, Renamed: 2, TypeChanged: 2, Moved: 1, AttrChanged: 1, DocChanged: 1, ValueChanged: 1, Added: 2, ModuleAdded: 1, ModuleRemoved: 1}
	for c, n := range want {
		if r.Counts[c] != n {
			t.Errorf("%s: %d; want %d (%v)", c, r.Counts[c], n, r.Items)
		}
	}
	for _, it := range r.Items {
		if it.Class == TypeChanged && it.Name == "main" && it.Severity != Breaking {
			t.Errorf("main: severity %s; want breaking", it.Severity)
		}
		if it.Class == Renamed && it.Name == "oldName" && it.To != "newName" {
			t.Errorf("oldName renamed to %s", it.To)
		}
	}
}

func TestCompareAddedCountOnly(t *testing.T) {
	old := &index.Index{Header: index.Header{Toolchain: "x"}}
	cur := &index.Index{Header: index.Header{Toolchain: "x"}, Decls: []index.Decl{{Name: "a"}, {Name: "b"}}}
	r := Compare(old, cur, Options{})
	if r.Counts[Added] != 2 || len(r.Items) != 0 {
		t.Errorf("counts %v, items %v", r.Counts, r.Items)
	}
}
