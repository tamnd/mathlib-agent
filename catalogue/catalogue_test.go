package catalogue

import (
	"testing"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/family"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/plan"
)

func TestFirstSentence(t *testing.T) {
	cases := map[string]string{
		"":                                     "",
		"A set is open. More text.":            "A set is open.",
		"Uses `x.1` in it. Then more.":         "Uses `x.1` in it.",
		"Two\nlines here.\n\nSecond paragraph": "Two lines here.",
		"# Heading":                            "",
		"```\ncode\n```":                       "",
		"No full stop":                         "No full stop",
	}
	for in, want := range cases {
		if got := FirstSentence(in); got != want {
			t.Errorf("FirstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuild(t *testing.T) {
	vm, err := blueprint.ParseVolumes([]byte(`volumes:
  - id: V13
    title: General topology
    prefixes: [Mathlib/Topology]
  - id: V05
    title: Groups
    prefixes: [Mathlib/Algebra/Group]
`))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{Header: index.Header{Mathlib: "abc"}, Decls: []index.Decl{
		{Name: "B", Kind: "theorem", Module: "Mathlib.Topology.Basic", Line: 20, TypeHash: "h2"},
		{Name: "A", Kind: "def", Module: "Mathlib.Topology.Basic", Line: 10, TypeHash: "h1", Doc: "The open sets. More."},
		{Name: "C.ext", Kind: "theorem", Module: "Mathlib.Topology.Order", Line: 5, TypeHash: "h3"},
		{Name: "Old", Kind: "theorem", Module: "Mathlib.Topology.Basic", Line: 30, Deprecated: &index.Deprecation{Target: "B"}},
		{Name: "G", Kind: "theorem", Module: "Mathlib.Algebra.Group.Basic", Line: 1},
		{Name: "Stale", Kind: "theorem", Module: "Mathlib.Topology.Order", Line: 9, TypeHash: "new"},
	}}
	fams := []*family.Family{{ID: "F-ext", Text: "An extensionality lemma."}}
	in := Input{
		Index: ix, Volumes: vm, Families: fams,
		Depths: map[string]depth.Result{"B": {Depth: "E3"}},
		Match:  family.Result{Member: map[string]string{"C.ext": "F-ext"}},
		Plans: map[string]*plan.Plan{"V13": {Chapters: []plan.Chapter{{Sections: []plan.Section{
			{ID: "V13.C01.S01", Modules: []string{"Mathlib.Topology.Basic"}},
			{ID: "V13.C01.S02", Modules: []string{"Mathlib.Topology.Order"}, E3: []string{"B"}},
		}}}}},
		Records: map[string][]informal.Record{"V13": {
			{Name: "B", TypeHash: "h2", Text: "A sentence for B.", Checks: informal.Checks{Roundtrip: informal.Pass}},
			{Name: "Stale", TypeHash: "old", Text: "An old sentence."},
		}},
		Date: "2026-11-27",
	}
	h, rows := Build(in, "V13")
	if h.Catalogue != "C13" || h.Title != "General topology" || h.Count != 4 || h.Mathlib != "abc" {
		t.Fatalf("header %+v", h)
	}
	want := []Row{
		{Name: "A", Kind: "def", Module: "Mathlib.Topology.Basic", Source: FromDoc, Text: "The open sets.", Section: "V13.C01.S01"},
		{Name: "B", Kind: "theorem", Module: "Mathlib.Topology.Basic", Depth: "E3", Source: FromE1, Text: "A sentence for B.", Section: "V13.C01.S02"},
		{Name: "C.ext", Kind: "theorem", Module: "Mathlib.Topology.Order", Source: FromFamily, Text: "An extensionality lemma.", Family: "F-ext", Section: "V13.C01.S02"},
		{Name: "Stale", Kind: "theorem", Module: "Mathlib.Topology.Order", Source: FromNone, Section: "V13.C01.S02"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i := range want {
		g, w := rows[i], want[i]
		if g.Name != w.Name || g.Source != w.Source || g.Text != w.Text || g.Section != w.Section || g.Family != w.Family || g.Depth != w.Depth {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
	if h.Sources[FromDoc] != 1 || h.Sources[FromE1] != 1 || h.Sources[FromFamily] != 1 || h.Sources[FromNone] != 1 {
		t.Errorf("sources %v", h.Sources)
	}
	if ID("V05") != "C05" {
		t.Errorf("ID(V05) = %s", ID("V05"))
	}
}
