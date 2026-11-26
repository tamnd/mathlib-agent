package plan

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
)

const volumes = `volumes:
  - id: V01
    title: Language
    level: foundation
    prefixes: [Mathlib/Logic]
  - id: V13
    title: General topology
    level: undergraduate
    prefixes: [Mathlib/Topology/Basic, Mathlib/Topology/Compactness]
unmapped: fault
`

// fixture makes an index with three modules of V13 and one of V01. Each module of V13 has one E3 theorem and 30 E1 declarations, so a module is 1.8 pages.
func fixture(t *testing.T) Input {
	t.Helper()
	vm, err := blueprint.ParseVolumes([]byte(volumes))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{Header: index.Header{Mathlib: "e54979a532247ae4eb9d5578a8fd40a1bc332101"}}
	depths := map[string]depth.Result{}
	add := func(mod, name, d string, uses ...string) {
		ix.Decls = append(ix.Decls, index.Decl{Name: name, Kind: "theorem", Module: mod, Uses: uses})
		depths[name] = depth.Result{Depth: d}
	}
	for _, m := range []string{"Mathlib.Logic.Basic", "Mathlib.Topology.Basic", "Mathlib.Topology.Compactness.Compact", "Mathlib.Topology.Compactness.Lindelof"} {
		ix.Modules = append(ix.Modules, index.Module{Module: m})
	}
	add("Mathlib.Logic.Basic", "not_not", depth.E3)
	add("Mathlib.Topology.Basic", "IsOpen", depth.E3, "not_not", "not_not")
	add("Mathlib.Topology.Compactness.Compact", "IsCompact", depth.E3, "IsOpen", "IsOpen", "IsOpen")
	add("Mathlib.Topology.Compactness.Lindelof", "IsLindelof", depth.E3, "IsOpen")
	for _, m := range ix.Modules[1:] {
		for i := range 30 {
			add(m.Module, fmt.Sprintf("%s.e1_%d", m.Module, i), depth.E1)
		}
	}
	return Input{Index: ix, Volumes: vm, Depths: depths, Families: map[string]string{"IsOpen": "F-test"}}
}

func TestMake(t *testing.T) {
	o := Defaults
	o.SectionPages, o.ChapterPages, o.MinUses = 3, 1, 2
	p, err := Make(fixture(t), "V13", o)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Chapters) != 2 {
		t.Fatalf("%d chapters, want 2: %+v", len(p.Chapters), p.Chapters)
	}
	c1, c2 := p.Chapters[0], p.Chapters[1]
	if c1.Title != "Basic" || c2.Title != "Compactness" {
		t.Errorf("titles %q and %q", c1.Title, c2.Title)
	}
	if len(c2.Sections) != 1 || len(c2.Sections[0].Modules) != 2 || c2.Sections[0].Counts[depth.E3] != 2 {
		t.Errorf("the sections of C02: %+v", c2.Sections)
	}
	if got := c2.Requires; len(got) != 1 || got[0] != "V13.C01" {
		t.Errorf("C02 requires %v, want [V13.C01]", got)
	}
	if got := c1.Requires; len(got) != 1 || got[0] != "V01" {
		t.Errorf("C01 requires %v, want [V01]", got)
	}
	if f := c1.Sections[0].Families; len(f) != 1 || f[0] != "F-test" {
		t.Errorf("C01 families %v", f)
	}
	if p.Pages != 5.4 {
		t.Errorf("pages %v, want 5.4", p.Pages)
	}
	path := Path(t.TempDir(), "V13")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil || back.Chapters[1].Sections[0].ID != "V13.C02.S01" || filepath.Base(path) != "V13.yaml" {
		t.Fatalf("load: %v %+v", err, back)
	}
	if _, err := Make(fixture(t), "V99", o); err == nil {
		t.Error("an unknown volume must fail")
	}
}

func TestPagesAndTitles(t *testing.T) {
	if got := Pages(map[string]int{depth.E3: 10, depth.E2: 10, depth.E1: 30}); got != 10.5 {
		t.Errorf("pages %v, want 10.5", got)
	}
	if got := title("Mathlib/Topology/MetricSpace"); got != "Metric space" {
		t.Errorf("title %q", got)
	}
	if got := sectionTitle([]string{"Mathlib.Topology.Compactness.Compact", "Mathlib.Topology.Compactness.Lindelof"}); got != "Compactness" {
		t.Errorf("section title %q", got)
	}
	if got := titles([]string{"a/Baire", "a/Bornology", "other"}); got != "Baire, Bornology and other modules" {
		t.Errorf("titles %q", got)
	}
}
