package coverage

import (
	"testing"

	"github.com/tamnd/mathlib-agent/internal/blueprint"
	"github.com/tamnd/mathlib-agent/internal/depth"
	"github.com/tamnd/mathlib-agent/internal/index"
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
