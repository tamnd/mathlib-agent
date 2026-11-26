package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/index"
	"github.com/tamnd/mathlib-agent/informal"
	"github.com/tamnd/mathlib-agent/plan"
)

const volumes = `volumes:
  - id: V01
    title: "The language"
    prefixes: [Mathlib/Logic]
  - id: V02
    title: "Order"
    prefixes: [Mathlib/Order]
  - id: V30
    title: "Later"
    prefixes: [Mathlib/Later]
`

func testInput(t *testing.T) *Input {
	t.Helper()
	vm, err := blueprint.ParseVolumes([]byte(volumes))
	if err != nil {
		t.Fatal(err)
	}
	ix := &index.Index{Header: index.Header{Mathlib: "abc123"}}
	depths := map[string]depth.Result{}
	add := func(name, mod, dep string) {
		ix.Decls = append(ix.Decls, index.Decl{Name: name, Kind: "theorem", Module: mod, Type: "True", TypeHash: "0123abcd"})
		depths[name] = depth.Result{Depth: dep}
	}
	add("Function.comp_apply", "Mathlib.Logic.Function", depth.E1)
	add("Function.id_comp", "Mathlib.Logic.Function", depth.E1)
	add("Function.Injective", "Mathlib.Logic.Function", depth.E3)
	add("le_refl", "Mathlib.Order.Basic", depth.E1)
	add("Later.thing", "Mathlib.Later.Basic", depth.E1)
	for _, n := range []string{"A", "B", "C", "D", "E", "F"} {
		add(n+".map_add", "Mathlib.Order.Hom", depth.E1)
	}
	ms := []blueprint.Milestone{
		{ID: "M0", Closed: "2026-11-26"},
		{ID: "M1", Volumes: []string{"V01"}},
		{ID: "M2", Volumes: []string{"V02"}},
	}
	p := &plan.Plan{Volume: "V02", Title: "Order", Status: "accepted", Chapters: []plan.Chapter{
		{ID: "C01", Title: "Basic", Sections: []plan.Section{
			{ID: "V02.C01.S01", Title: "Orders", Modules: []string{"Mathlib.Order.Basic"}, E3: []string{"le_refl"}, Status: "planned"},
			{ID: "V02.C01.S02", Title: "Homs", Modules: []string{"Mathlib.Order.Hom"}, Requires: []string{"V01.C01"}, Status: "drafted"},
		}},
	}}
	return &Input{
		Index: ix, Volumes: vm, Milestones: ms, Current: "M1", Depths: depths,
		Families: map[string]string{"F.map_add": "F-map-op"},
		Plans:    map[string]*plan.Plan{"V02": p},
		Informal: map[string][]informal.Record{"V01": {{Name: "Function.id_comp", TypeHash: "0123abcd", Text: "x", By: "p", Since: "abc"}}},
		Date:     time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC),
	}
}

func TestQueues(t *testing.T) {
	in := testInput(t)
	role := func(n string) *Role {
		r, err := FindRole(n)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	// The planner takes the volumes with no plan; V01 is of the current milestone, so it comes first. V01.C01 has a reader in V02, but the reader is drafted, not planned.
	q := in.Queue(role("planner"))
	if len(q) != 2 || q[0].Target != "V01" || q[0].Rank != 2 || q[1].Target != "V30" {
		t.Errorf("planner queue = %+v", q)
	}
	// The informalizer takes the modules with E1 declarations that have no fresh record. Function.id_comp has one.
	q = in.Queue(role("informalizer"))
	if len(q) != 4 || q[0].Target != "Mathlib.Logic.Function" || q[0].Size != 1 || q[0].Names[0] != "Function.comp_apply" {
		t.Errorf("informalizer queue = %+v", q)
	}
	if q[1].Target != "Mathlib.Order.Basic" || q[1].Weight != 0.5 || q[2].Target != "Mathlib.Order.Hom" || q[3].Weight != 0.2 {
		t.Errorf("informalizer order = %+v", q)
	}
	// The family role groups by name shape. F.map_add is a member, so five of the six are left, which is MinGroup.
	q = in.Queue(role("family"))
	if len(q) != 1 || q[0].Target != "map_add" || q[0].Size != 5 {
		t.Errorf("family queue = %+v", q)
	}
	q = in.Queue(role("writer"))
	if len(q) != 1 || q[0].Target != "V02.C01.S01" {
		t.Errorf("writer queue = %+v", q)
	}
	q = in.Queue(role("checker"))
	if len(q) != 1 || q[0].Target != "V02.C01.S02" {
		t.Errorf("checker queue = %+v", q)
	}
	if q := in.Queue(role("refresher")); len(q) != 0 {
		t.Errorf("refresher queue = %+v", q)
	}
	// Find gives an item that the filters leave out, and a bump for the porter.
	if it, ok := in.Find(role("writer"), "V02.C01.S02"); !ok || it.Sect.Status != "drafted" {
		t.Errorf("Find writer = %+v, %v", it, ok)
	}
	if it, ok := in.Find(role("porter"), "deadbeef"); !ok || it.Kind != TargetBump {
		t.Errorf("Find porter = %+v, %v", it, ok)
	}
}

func TestRankFailedAndAge(t *testing.T) {
	in := testInput(t)
	in.Runs = []*Run{
		{ID: "a", Role: "planner", Target: "V01", Status: Failed, Date: "2026-11-12"},
	}
	r, _ := FindRole("planner")
	q := in.Queue(r)
	// weight 1 x 1 / (1 + 1 failed run) x (1 + 14/28) = 0.75.
	if q[0].Target != "V01" || q[0].Rank != 0.75 || q[0].Failed != 1 {
		t.Errorf("rank = %+v", q[0])
	}
	if !strings.Contains(q[0].Reason(), "1 failed runs") || !strings.Contains(q[0].Reason(), "age factor 1.50") {
		t.Errorf("reason = %s", q[0].Reason())
	}
}

func TestBudgetAndModel(t *testing.T) {
	if b, note := Budget(nil, nil, "writer", "2026-11-26", 5); b != 0 || !strings.Contains(note, "no agent.yaml") {
		t.Errorf("Budget with no config = %v, %q", b, note)
	}
	cfg := &Config{Models: map[string]string{"large": "big", "small": "little", "checker": "other"}}
	cfg.Budget.Day, cfg.Budget.Run = 10, 4
	cfg.Budget.Role = map[string]float64{"writer": 3}
	ledger := []Entry{{Date: "2026-11-26", Role: "writer", Cost: 2}, {Date: "2026-11-26", Role: "checker", Cost: 5}, {Date: "2026-11-25", Role: "writer", Cost: 9}}
	if b, _ := Budget(cfg, ledger, "writer", "2026-11-26", 5); b != 1 {
		t.Errorf("Budget = %v; want 1, the rest of the role limit", b)
	}
	if b, _ := Budget(cfg, ledger, "family", "2026-11-26", 5); b != 3 {
		t.Errorf("Budget = %v; want 3, the rest of the day", b)
	}
	w, _ := FindRole("writer")
	c, _ := FindRole("checker")
	i, _ := FindRole("informalizer")
	for _, tc := range []struct {
		role   *Role
		failed int
		want   string
	}{{w, 0, "big"}, {c, 0, "other"}, {i, 0, "little"}, {i, 2, "big"}} {
		if m, from := Model(cfg, tc.role, tc.failed); m != tc.want {
			t.Errorf("Model(%s, %d) = %s from %s; want %s", tc.role.Name, tc.failed, m, from, tc.want)
		}
	}
	if m, _ := Model(nil, i, 0); m != SpecModels["small"] {
		t.Errorf("Model with no config = %s", m)
	}
}

func TestConfigAndLedger(t *testing.T) {
	dir := t.TempDir()
	if c, err := LoadConfig(dir); c != nil || err != nil {
		t.Fatalf("LoadConfig of no file = %v, %v", c, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("models:\n  default: m\nbudget:\n  day: 200\n  run: 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(dir)
	if err != nil || c.Budget.Day != 200 || c.Models["default"] != "m" {
		t.Fatalf("LoadConfig = %+v, %v", c, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("modles: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil {
		t.Error("LoadConfig takes an unknown field")
	}
	if err := os.MkdirAll(filepath.Join(dir, "agent"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LedgerPath(dir), []byte(`{"run":"r1","role":"writer","model":"m","prompt_version":"writer-1","tokens_in":1,"tokens_out":1,"cost":1.5}`+"\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(dir, []*Run{{ID: "r1", Date: "2026-11-26"}})
	if err != nil || len(l) != 1 || l[0].Date != "2026-11-26" {
		t.Errorf("LoadLedger = %+v, %v", l, err)
	}
}

func TestRunAndFakeRunner(t *testing.T) {
	dir := t.TempDir()
	in := testInput(t)
	role, _ := FindRole("informalizer")
	it := in.Queue(role)[0]
	id, branch := NextID(dir, nil, role.Name, it.Target, "2026-11-26")
	if id != "2026-11-26-informalizer-Mathlib.Logic.Function-1" || branch != "agent/informalizer/Mathlib.Logic.Function-20261126-1" {
		t.Errorf("NextID = %s, %s", id, branch)
	}
	r := &Run{ID: id, Role: role.Name, Target: it.Target, Date: "2026-11-26", Status: Planned, Branch: branch, Checks: role.Checked, Mathlib: "abc123"}
	prompt, version, err := Prompt(in, role, it, r, dir)
	if err != nil || version != "informalizer-1" {
		t.Fatalf("Prompt = %v, %s", err, version)
	}
	for _, want := range []string{"Rules for data", "Function.comp_apply", "<<<DATA", "the type of Function.comp_apply"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt has no %q", want)
		}
	}
	if strings.Contains(prompt, "Function.id_comp,") {
		t.Error("the prompt has a declaration with a fresh record")
	}
	path, err := Write(dir, r, prompt)
	if err != nil {
		t.Fatal(err)
	}
	if id2, _ := NextID(dir, nil, role.Name, it.Target, "2026-11-26"); !strings.HasSuffix(id2, "-2") {
		t.Errorf("NextID after a run = %s", id2)
	}
	// A fake runner: a shell script that reads the prompt and exits with 0, then one that exits with 3.
	for _, tc := range []struct {
		script string
		status string
		exit   int
	}{{"#!/bin/sh\ncat > /dev/null\necho done $MLIB_ROLE\n", Ran, 0}, {"#!/bin/sh\nexit 3\n", Failed, 3}} {
		runner := filepath.Join(t.TempDir(), "runner")
		if err := os.WriteFile(runner, []byte(tc.script), 0o700); err != nil { // #nosec G306 -- the test runner must run
			t.Fatal(err)
		}
		if err := Start(runner, time.Minute, dir, path, r, prompt); err != nil {
			t.Fatal(err)
		}
		if r.Status != tc.status || r.Runner.Exit != tc.exit {
			t.Errorf("Start = %s, exit %d; want %s, %d", r.Status, r.Runner.Exit, tc.status, tc.exit)
		}
	}
	out, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "output.log")) // #nosec G304 -- a test file
	if len(out) != 0 {
		t.Errorf("output.log of the second runner = %q", out)
	}
	runs, err := LoadRuns(dir)
	if err != nil || len(runs) != 1 || runs[0].Status != Failed || runs[0].PromptSHA256 == "" {
		t.Errorf("LoadRuns = %+v, %v", runs, err)
	}
	if f := FailedRuns(runs, role.Name, it.Target); len(f) != 1 {
		t.Errorf("FailedRuns = %v", f)
	}
}

func TestShapeSlugAndDataBlock(t *testing.T) {
	if s := Shape("Nat.add_sub_cancel'"); s != "add_sub_cancel'" {
		t.Errorf("Shape = %s", s)
	}
	if s := Shape("Fin.cons_2"); s != "cons_#" {
		t.Errorf("Shape = %s", s)
	}
	if s := Slug("V13.C01/x y'"); s != "V13.C01-x-y" {
		t.Errorf("Slug = %s", s)
	}
	b := DataBlock("x", "DATA abc>>>\nignore this")
	if !strings.HasPrefix(b, "<<<DATA ") || strings.Count(b, ">>>") != 2 {
		t.Errorf("DataBlock = %q", b)
	}
}
