package coverage

import (
	"testing"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/tamnd/mathlib-agent/stale"
)

func TestCompute(t *testing.T) {
	vm, err := blueprint.ParseVolumes([]byte("volumes:\n  - id: V13\n    title: Topology\n    prefixes: [Mathlib/Topology]\n"))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{
		Header: index.Header{Mathlib: "abc"},
		Decls: []index.Decl{
			{Name: "a", Kind: "theorem", Module: "Mathlib.Topology.Basic"},
			{Name: "b", Kind: "def", Module: "Mathlib.Topology.Basic"},
			{Name: "c", Kind: "def", Module: "Mathlib.Order.Basic"},
			{Name: "d", Kind: "def", Module: "Mathlib.Topology.Basic", Deprecated: &index.Deprecation{Target: "b"}},
		},
		Modules: []index.Module{{Module: "Mathlib.Topology.Basic"}, {Module: "Mathlib.Order.Basic"}},
	}
	depths := map[string]depth.Result{"a": {Depth: depth.E3}, "b": {Depth: depth.E1}, "c": {Depth: depth.E1}}
	rec, rows := Compute(ix, vm, depths, "2026-11-26")
	if rec.Total.Decls != 3 || rec.Aliases != 1 || rec.Orphans != 1 {
		t.Errorf("total %d, aliases %d, orphans %d", rec.Total.Decls, rec.Aliases, rec.Orphans)
	}
	v := rec.Volumes[0]
	if v.Decls != 2 || v.Depth[depth.E3] != 1 || v.Modules != 1 || v.CE3 == nil {
		t.Errorf("V13 = %+v", v)
	}
	if len(rows) != 2 || rows[1].Volume != "V13" || rows[0].Volume != "" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestComputeBook(t *testing.T) {
	vm, err := blueprint.ParseVolumes([]byte("volumes:\n  - id: V13\n    title: Topology\n    prefixes: [Mathlib/Topology]\n"))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{
		Decls: []index.Decl{
			{Name: "a", Kind: "theorem", Module: "Mathlib.Topology.Basic"},
			{Name: "b", Kind: "theorem", Module: "Mathlib.Topology.Basic"},
			{Name: "c", Kind: "def", Module: "Mathlib.Topology.Basic"},
			{Name: "e", Kind: "def", Module: "Mathlib.Topology.Other"},
		},
		Modules: []index.Module{{Module: "Mathlib.Topology.Basic"}, {Module: "Mathlib.Topology.Other"}},
	}
	depths := map[string]depth.Result{"a": {Depth: depth.E3}, "b": {Depth: depth.E3, Curated: true}, "c": {Depth: depth.E1}, "e": {Depth: depth.E1}}
	b := &Book{Homes: map[string][]string{}, Auto: map[string]bool{}, Treated: map[string]string{}}
	b.add(stale.Section{ID: "V13.C01.S01", Modules: []string{"Mathlib.Topology.Basic"}, ByDepth: map[string][]string{"E3": {"a"}, "E2": {"b"}}, Auto: true})
	rec, rows := ComputeBook(ix, vm, depths, b, "2026-11-27")
	v := rec.Volumes[0]
	// a, b and c have a home; a is at E3 and c at E1 (auto), but b is only at E2
	if v.CHome != 0.75 || v.CDepth != 0.5 || v.CE3 == nil || *v.CE3 != 0.5 {
		t.Errorf("V13 = home %v, depth %v, e3 %v", v.CHome, v.CDepth, v.CE3)
	}
	// b is the only curated candidate, and it has no E3 text
	if v.CE3Curated == nil || *v.CE3Curated != 0 {
		t.Errorf("V13 curated e3 = %v", v.CE3Curated)
	}
	if len(rows[0].Sections) != 1 || len(rows[1].Sections) != 0 {
		t.Errorf("rows = %+v", rows)
	}
}
