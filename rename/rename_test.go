package rename

import (
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/informal"
)

func chain() []drift.Record {
	return []drift.Record{
		{From: "bbbbbbbbbb", To: "cccccccccc", Items: []drift.Item{
			{Class: drift.Renamed, Name: "Nat.sub_le_new", To: "Nat.sub_le_newer", Note: "deprecated alias since 2026-11-01"},
			{Class: drift.Moved, Name: "Real.pi_pos", Module: "Mathlib.Analysis.Pi", To: "Mathlib.Analysis.Real.Pi"},
		}},
		{From: "aaaaaaaaaa", To: "bbbbbbbbbb", Items: []drift.Item{
			{Class: drift.Renamed, Name: "Nat.sub_le_old", To: "Nat.sub_le_new", Note: "deprecated alias since 2026-10-01"},
			{Class: drift.Renamed, Name: "guess", To: "guess2", Note: "same type, no alias: a proposal for a person to accept"},
			{Class: drift.ModuleRenamed, Name: "Mathlib.Old.Mod", To: "Mathlib.New.Mod"},
			{Class: drift.TypeChanged, Name: "x"},
		}},
	}
}

func TestChainAndTable(t *testing.T) {
	c, err := Chain(chain(), "aaaaaaa", "cccccccccc")
	if err != nil || len(c) != 2 || c[0].From != "aaaaaaaaaa" {
		t.Fatalf("Chain = %+v, %v", c, err)
	}
	if _, err := Chain(chain(), "aaaaaaa", "ddddddd"); err == nil {
		t.Error("Chain to an unknown commit has no error")
	}
	tab := Table(c, false)
	want := map[string]string{"Nat.sub_le_old": "Nat.sub_le_newer", "Mathlib.Old.Mod": "Mathlib.New.Mod", "Real.pi_pos": "Mathlib.Analysis.Real.Pi"}
	if len(tab) != 3 {
		t.Fatalf("Table = %+v", tab)
	}
	for _, e := range tab {
		if want[e.Old] != e.New {
			t.Errorf("entry %+v", e)
		}
	}
	if len(Table(c, true)) != 4 {
		t.Error("Table with proposals does not take the proposal")
	}
}

const section = "---\n" +
	"title: Nat.sub_le_old in words\n" +
	"modules:\n" +
	"  - Mathlib.Old.Mod\n" +
	"  - Mathlib.Analysis.Pi\n" +
	"explains:\n" +
	"  E3: [Nat.sub_le_old, Real.pi_pos]\n" +
	"---\n" +
	"The lemma `Nat.sub_le_old` says that Nat.sub_le_old is true.\n" +
	"::: mathlib\n" +
	"`Nat.sub_le_old` · `Mathlib.Old.Mod`\n" +
	"\n" +
	"<?decl Nat.sub_le_old?>\n" +
	":::\n" +
	"```lean\n" +
	"example := Nat.sub_le_old (Nat.sub_le_old_x)\n" +
	"```\n"

func TestMarkdown(t *testing.T) {
	c, _ := Chain(chain(), "aaaaaaaaaa", "cccccccccc")
	out, ch := Markdown("s.md", section, Table(c, false))
	for _, want := range []string{
		"title: Nat.sub_le_old in words",
		"  - Mathlib.New.Mod\n  - Mathlib.Analysis.Pi\n  - Mathlib.Analysis.Real.Pi\n",
		"E3: [Nat.sub_le_newer, Real.pi_pos]",
		"The lemma `Nat.sub_le_newer` says that Nat.sub_le_old is true.",
		"`Nat.sub_le_newer` · `Mathlib.New.Mod`",
		"<?decl Nat.sub_le_newer?>",
		"example := Nat.sub_le_newer (Nat.sub_le_old_x)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output has no %q:\n%s", want, out)
		}
	}
	if len(ch) != 8 {
		t.Errorf("changes = %d: %+v", len(ch), ch)
	}
	if again, ch2 := Markdown("s.md", out, Table(c, false)); again != out || len(ch2) != 0 {
		t.Errorf("a second rewrite changes %+v", ch2)
	}
}

func TestLean(t *testing.T) {
	c, _ := Chain(chain(), "aaaaaaaaaa", "cccccccccc")
	src := "module\npublic import Mathlib.Old.Mod\nimport Mathlib.Old.ModX\n\nexample : True := by\n  exact Nat.sub_le_old.trans (Nat.sub_le_old)\n-- Nat.sub_le_old.\n"
	out, ch := Lean("E.lean", src, Table(c, false))
	for _, want := range []string{"public import Mathlib.New.Mod\n", "import Mathlib.Old.ModX\n", "exact Nat.sub_le_old.trans (Nat.sub_le_newer)", "-- Nat.sub_le_newer.\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output has no %q:\n%s", want, out)
		}
	}
	if len(ch) != 3 {
		t.Errorf("changes = %+v", ch)
	}
}

func TestInformal(t *testing.T) {
	c, _ := Chain(chain(), "aaaaaaaaaa", "cccccccccc")
	rs := []informal.Record{{Name: "Nat.sub_le_old"}, {Name: "other"}}
	rs, ch := Informal("V01.jsonl", rs, Table(c, false))
	if rs[0].Name != "Nat.sub_le_newer" || len(rs[0].Formerly) != 1 || rs[0].Formerly[0] != "Nat.sub_le_old" || len(ch) != 1 {
		t.Errorf("Informal = %+v, %+v", rs, ch)
	}
	rs = []informal.Record{{Name: "Nat.sub_le_old"}, {Name: "Nat.sub_le_newer"}}
	rs, ch = Informal("V01.jsonl", rs, Table(c, false))
	if rs[0].Name != "Nat.sub_le_old" || !strings.Contains(ch[0].New, "not changed") {
		t.Errorf("Informal with a clash = %+v, %+v", rs, ch)
	}
}

func TestReplaceNames(t *testing.T) {
	m := map[string]string{"foo": "bar", "Nat.x": "Nat.y"}
	for in, want := range map[string]string{
		"foo foo_1 Nat.foo afoo": "bar foo_1 Nat.foo afoo",
		"(Nat.x) Nat.x' Nat.xs":  "(Nat.y) Nat.x' Nat.xs",
		"h.foo":                  "h.foo",
		"foo.":                   "bar.",
		"∀ x, foo x":             "∀ x, bar x",
	} {
		if got, _ := replaceNames(in, m); got != want {
			t.Errorf("replaceNames(%q) = %q; want %q", in, got, want)
		}
	}
}
