package blueprint

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `volumes:
  - id: V13
    title: General topology
    prefixes:
      - Mathlib/Topology
  - id: V14
    title: Topological algebra
    prefixes:
      - Mathlib/Topology/Algebra
      - Mathlib/Topology/Gone
exceptions:
  Mathlib/Topology/Algebra/Odd: V13.C01
unmapped: fault
`

func TestHome(t *testing.T) {
	m, err := ParseVolumes([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"Mathlib/Topology/Basic":         "V13",
		"Mathlib/Topology":               "V13",
		"Mathlib/Topology/Algebra/Group": "V14",
		"Mathlib/Topology/Algebra":       "V14",
		"Mathlib/Topology/AlgebraX":      "V13",
		"Mathlib/Topology/Algebra/Odd":   "V13",
	}
	for mod, want := range cases {
		got, _, ok := m.Home(mod)
		if !ok || got != want {
			t.Errorf("Home(%s) = %s, %v; want %s", mod, got, ok, want)
		}
	}
	if _, _, ok := m.Home("Mathlib/Order/Basic"); ok {
		t.Error("Mathlib/Order/Basic has a home; want none")
	}
}

func TestCheck(t *testing.T) {
	m, err := ParseVolumes([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	r := m.Check([]string{"Mathlib/Topology/Basic", "Mathlib/Topology/Algebra/Group", "Mathlib/Order/Basic"})
	if len(r.Orphans) != 1 || r.Orphans[0] != "Mathlib/Order/Basic" {
		t.Errorf("orphans = %v", r.Orphans)
	}
	if len(r.Unused) != 1 || r.Unused[0] != "Mathlib/Topology/Gone" {
		t.Errorf("unused = %v", r.Unused)
	}
	if r.PerVol["V13"] != 1 || r.PerVol["V14"] != 1 {
		t.Errorf("per volume = %v", r.PerVol)
	}
	if r.OK() {
		t.Error("OK is true; want false")
	}
}

func TestDuplicatePrefix(t *testing.T) {
	_, err := ParseVolumes([]byte("volumes:\n  - id: V01\n    title: A\n    prefixes: [Mathlib/Logic]\n  - id: V02\n    title: B\n    prefixes: [Mathlib/Logic]\n"))
	if err == nil {
		t.Fatal("no error for a prefix in two volumes")
	}
}

func TestProblems(t *testing.T) {
	m, err := ParseVolumes([]byte("volumes:\n  - id: X1\n    prefixes: [Logic]\nexceptions:\n  Mathlib/A: V99.C01\n"))
	if err != nil {
		t.Fatal(err)
	}
	r := m.Check(nil)
	if len(r.Problems) != 4 {
		t.Errorf("problems = %v; want 4", r.Problems)
	}
}

func TestModules(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Mathlib/A/B.lean", "Mathlib/C.lean", "Mathlib/README.md"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Modules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "Mathlib/A/B" || got[1] != "Mathlib/C" {
		t.Errorf("modules = %v", got)
	}
}
