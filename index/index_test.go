package index

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const decls = `{"index":"declarations","format":1,"mathlib":"abc","toolchain":"4.37.0","date":"2026-11-26","count":2}
{"name":"Symmetric","kind":"def","module":"Mathlib.Order.Defs","line":199,"typeHash":"0000aaaa","attrs":[{"deprecated":{"since":"2026-06-10","target":"Std.Symm"}}]}
{"name":"le_refl","kind":"theorem","module":"Mathlib.Order.Basic","line":1,"typeHash":"0000bbbb","attrs":["simp"],"uses":["Symmetric"]}
`

const mods = `{"index":"modules","format":1,"mathlib":"abc","count":1}
{"module":"Mathlib.Order.Basic","docs":["# Order"]}
`

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(decls), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "modules.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	z := gzip.NewWriter(f)
	if _, err := z.Write([]byte(mods)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	ix, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Decls) != 2 || len(ix.Modules) != 1 || ix.Header.Mathlib != "abc" {
		t.Fatalf("index = %+v", ix)
	}
	s, ok := ix.Lookup("Symmetric")
	if !ok || s.Deprecated == nil || s.Deprecated.Target != "Std.Symm" || !s.Has("deprecated") {
		t.Errorf("Symmetric = %+v", s)
	}
	l, _ := ix.Lookup("le_refl")
	if !l.Has("simp") || l.Path() != "Mathlib/Order/Basic" {
		t.Errorf("le_refl = %+v", l)
	}
	if ub := ix.UsedBy(); ub["Symmetric"] != 1 {
		t.Errorf("used by = %v", ub)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back Decl
	if err := json.Unmarshal(b, &back); err != nil || back.Deprecated == nil || back.Deprecated.Since != "2026-06-10" {
		t.Errorf("round trip: %s, %v", b, err)
	}
}

const proofs = `{"index":"proofs","format":1,"mathlib":"abc","count":2}
{"name":"le_refl","proofUses":["Symmetric","lt_irrefl"]}
{"name":"lt_irrefl","proofUses":["le_refl"]}
`

func TestLoadProofs(t *testing.T) {
	dir := t.TempDir()
	three := decls[:len(decls)-1] + "\n" + `{"name":"lt_irrefl","kind":"theorem","module":"Mathlib.Order.Defs","line":9,"typeHash":"0000cccc"}` + "\n"
	for name, body := range map[string]string{"index.jsonl": three, "modules.jsonl": mods, "proofs.jsonl": proofs} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ix, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ix.Proofs {
		t.Fatal("Proofs is false")
	}
	ub := ix.UsedBy()
	// le_refl uses Symmetric in its type and in its proof: one use. lt_irrefl is in another module than le_refl.
	if ub["Symmetric"] != 1 || ub["lt_irrefl"] != 1 || ub["le_refl"] != 1 {
		t.Errorf("used by = %v", ub)
	}
	if err := os.WriteFile(filepath.Join(dir, "proofs.jsonl"), []byte(strings.Replace(proofs, `"abc"`, `"xyz"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("no error for a proofs.jsonl of another commit")
	}
}

func TestLoadBadFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.jsonl"), []byte(`{"index":"declarations","format":9}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("no error for format 9")
	}
}
