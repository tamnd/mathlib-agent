package informal

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/mathlib-agent/index"
)

func rec(name, hash string) Record {
	return Record{Name: name, TypeHash: hash, Text: "A sentence.", By: "informalizer@test", Since: "e54979a532", Checks: Checks{Roundtrip: Pass}}
}

func TestSaveLoadStale(t *testing.T) {
	path := Path(t.TempDir(), "V13")
	rs := []Record{rec("b", "0000bbbb"), rec("a", "0000aaaa"), rec("gone", "0000cccc")}
	if err := Save(path, rs); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil || len(back) != 3 || back[0].Name != "a" {
		t.Fatalf("load: %v %+v", err, back)
	}
	ix := &index.Index{Decls: []index.Decl{{Name: "a", TypeHash: "0000aaaa"}, {Name: "b", TypeHash: "1111bbbb"}, {Name: "c", TypeHash: "0000dddd"}}}
	st := FindStale(ix, back)
	if len(st) != 2 || st[0].Reason != "type" || st[0].NewHash != "1111bbbb" || st[1].Reason != "gone" {
		t.Fatalf("stale %+v", st)
	}
	s := Compute("V13", ix, map[string]bool{"a": true, "b": true, "c": true}, back)
	if s.Needed != 3 || s.Fresh != 1 || s.Stale != 2 || s.Missing != 2 || s.Roundtrip[Pass] != 1 || s.Critic["none"] != 1 {
		t.Fatalf("status %+v", s)
	}
	if l, err := Load(filepath.Join(t.TempDir(), "none.jsonl")); err != nil || l != nil {
		t.Fatal("a missing file must give no records")
	}
}

func TestValidate(t *testing.T) {
	two := rec("a", "0000aaaa")
	two.Text = "one\ntwo"
	bad := rec("a", "0000aaaa")
	bad.Checks.Critic = "ok"
	for _, c := range []struct {
		r    Record
		want string
	}{
		{rec("", "0000aaaa"), "no name"},
		{rec("a", "xyz"), "not hex"},
		{two, "one line"},
		{bad, "pass or fail"},
	} {
		if err := c.r.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("got %v, want %q", err, c.want)
		}
	}
}

func TestSample(t *testing.T) {
	var rs []Record
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		rs = append(rs, rec(n, "0000aaaa"))
	}
	s1, s2 := Sample(rs, 3, 7), Sample(rs, 3, 7)
	if len(s1) != 3 {
		t.Fatalf("sample of %d", len(s1))
	}
	for i := range s1 {
		if s1[i].Name != s2[i].Name {
			t.Fatal("the same seed must give the same sample")
		}
	}
	if len(Sample(rs, 10, 1)) != 6 {
		t.Fatal("a sample larger than the records gives all of them")
	}
}
