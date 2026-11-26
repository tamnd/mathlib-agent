package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMap writes a small map and a module list to a temporary directory.
func writeMap(t *testing.T, modules string) (dir, list string) {
	t.Helper()
	dir = t.TempDir()
	files := map[string]string{
		"blueprint/volumes.yaml":    "volumes:\n  - id: V13\n    title: General topology\n    prefixes: [Mathlib/Topology]\n",
		"blueprint/milestones.yaml": "milestones:\n  - id: M0\n    title: \"M0. Start\"\n    target: 2026-12-20\n    release: v0.1.0\n",
		"modules.txt":               modules,
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, filepath.Join(dir, "modules.txt")
}

func TestVolumesCheck(t *testing.T) {
	dir, list := writeMap(t, "Mathlib.Topology.Basic\n")
	var out, errb bytes.Buffer
	if code := run([]string{"volumes", "check", "-map", dir, "-modules", list}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "1 modules, 1 volumes, 0 orphans") {
		t.Errorf("output: %s", out.String())
	}
}

func TestVolumesOrphans(t *testing.T) {
	dir, list := writeMap(t, "Mathlib.Topology.Basic\nMathlib.Order.Basic\n")
	var out, errb bytes.Buffer
	if code := run([]string{"volumes", "orphans", "-map", dir, "-modules", list}, &out, &errb); code != 1 {
		t.Fatalf("exit %d; want 1", code)
	}
	if strings.TrimSpace(out.String()) != "Mathlib/Order/Basic" {
		t.Errorf("output: %q", out.String())
	}
}

func TestVolumesUsage(t *testing.T) {
	dir, _ := writeMap(t, "")
	for _, args := range [][]string{{"volumes"}, {"volumes", "nope", "-map", dir}, {"volumes", "check", "-map", dir}} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 2 {
			t.Errorf("%v: exit %d; want 2", args, code)
		}
	}
}

func TestCheck(t *testing.T) {
	dir, list := writeMap(t, "Mathlib.Topology.Basic\n")
	var out, errb bytes.Buffer
	if code := run([]string{"check", "-map", dir, "-modules", list}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "1 milestones, 0 problems") {
		t.Errorf("output: %s", out.String())
	}
}
