// Package index reads the index of Mathlib that scripts/Index.lean of tamnd/mathlib-lean writes (spec 09.3, 10.5).
package index

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Header is the first line of an index file.
type Header struct {
	Index      string `json:"index"` // declarations or modules
	Format     int    `json:"format"`
	Repository string `json:"repository"`
	Mathlib    string `json:"mathlib"`
	Toolchain  string `json:"toolchain"`
	Date       string `json:"date"`
	Count      int    `json:"count"`
}

// Deprecation is the deprecated attribute of a declaration.
type Deprecation struct {
	Target string `json:"target,omitempty"`
	Since  string `json:"since,omitempty"`
}

// Decl is one declaration of the index.
type Decl struct {
	Name       string       `json:"name"`
	Kind       string       `json:"kind"`
	Module     string       `json:"module"`
	Line       int          `json:"line"`
	Type       string       `json:"type,omitempty"`
	TypeHash   string       `json:"typeHash"`
	ValueHash  string       `json:"valueHash,omitempty"`
	Doc        string       `json:"doc,omitempty"`
	Attrs      []string     `json:"-"`
	Deprecated *Deprecation `json:"-"`
	Uses       []string     `json:"uses,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// UnmarshalJSON reads the attrs field, where an attribute is a string or an object such as {"deprecated": {...}}.
func (d *Decl) UnmarshalJSON(b []byte) error {
	type plain Decl
	var raw struct {
		plain
		Attrs []json.RawMessage `json:"attrs"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*d = Decl(raw.plain)
	for _, a := range raw.Attrs {
		var s string
		if json.Unmarshal(a, &s) == nil {
			d.Attrs = append(d.Attrs, s)
			continue
		}
		var o struct {
			Deprecated *Deprecation `json:"deprecated"`
		}
		if err := json.Unmarshal(a, &o); err != nil {
			return fmt.Errorf("the attribute %s of %s: %w", a, d.Name, err)
		}
		if o.Deprecated != nil {
			d.Deprecated = o.Deprecated
			d.Attrs = append(d.Attrs, "deprecated")
		}
	}
	return nil
}

// MarshalJSON writes the declaration in the form that UnmarshalJSON reads.
func (d Decl) MarshalJSON() ([]byte, error) {
	type plain Decl
	var attrs []any
	for _, a := range d.Attrs {
		if a == "deprecated" && d.Deprecated != nil {
			attrs = append(attrs, map[string]any{"deprecated": d.Deprecated})
			continue
		}
		attrs = append(attrs, a)
	}
	return json.Marshal(struct {
		plain
		Attrs []any `json:"attrs,omitempty"`
	}{plain(d), attrs})
}

// Has reports whether the declaration has the attribute a.
func (d *Decl) Has(a string) bool {
	for _, x := range d.Attrs {
		if x == a {
			return true
		}
	}
	return false
}

// Path returns the module as a path, such as Mathlib/Topology/Basic.
func (d *Decl) Path() string { return ModulePath(d.Module) }

// ModulePath turns a module name such as Mathlib.Topology.Basic into a path.
func ModulePath(m string) string { return strings.ReplaceAll(m, ".", "/") }

// Module is one line of modules.jsonl.
type Module struct {
	Module string   `json:"module"`
	Docs   []string `json:"docs"`
}

// Index is an index of Mathlib at one commit.
type Index struct {
	Header  Header
	Decls   []Decl
	Modules []Module

	byName map[string]int
}

// open opens a file, and reads it through gzip when the name ends in .gz.
func open(path string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, ".gz") {
		return f, nil
	}
	z, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{z, closers{z, f}}, nil
}

type closers []io.Closer

func (c closers) Close() error {
	var errs []error
	for _, x := range c {
		errs = append(errs, x.Close())
	}
	return errors.Join(errs...)
}

// readLines reads a JSON lines file. It decodes the header into h and calls each for every other line.
func readLines(path string, h *Header, each func([]byte) error) error {
	r, err := open(path)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1<<20), 64<<20)
	first := true
	for n := 1; s.Scan(); n++ {
		line := s.Bytes()
		if len(line) == 0 {
			continue
		}
		if first {
			first = false
			if err := json.Unmarshal(line, h); err != nil {
				return fmt.Errorf("%s: the header: %w", path, err)
			}
			if h.Format != 1 {
				return fmt.Errorf("%s: format %d; this mlib reads format 1", path, h.Format)
			}
			continue
		}
		if err := each(line); err != nil {
			return fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	if first {
		return fmt.Errorf("%s: the file is empty", path)
	}
	return nil
}

// find returns the first of the names that exists in dir.
func find(dir string, names ...string) (string, error) {
	for _, n := range names {
		p := filepath.Join(dir, n)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s has none of %s", dir, strings.Join(names, ", "))
}

// Load reads index.jsonl and modules.jsonl, or their .gz forms, from a directory.
func Load(dir string) (*Index, error) {
	ix := &Index{byName: map[string]int{}}
	p, err := find(dir, "index.jsonl", "index.jsonl.gz")
	if err != nil {
		return nil, err
	}
	err = readLines(p, &ix.Header, func(line []byte) error {
		var d Decl
		if err := json.Unmarshal(line, &d); err != nil {
			return err
		}
		ix.byName[d.Name] = len(ix.Decls)
		ix.Decls = append(ix.Decls, d)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if ix.Header.Index != "declarations" {
		return nil, fmt.Errorf("%s: the header says %q; want declarations", p, ix.Header.Index)
	}
	if p, err = find(dir, "modules.jsonl", "modules.jsonl.gz"); err != nil {
		return nil, err
	}
	var mh Header
	err = readLines(p, &mh, func(line []byte) error {
		var m Module
		if err := json.Unmarshal(line, &m); err != nil {
			return err
		}
		ix.Modules = append(ix.Modules, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if mh.Mathlib != ix.Header.Mathlib {
		return nil, fmt.Errorf("modules.jsonl is at %s and index.jsonl at %s", mh.Mathlib, ix.Header.Mathlib)
	}
	return ix, nil
}

// Lookup returns the declaration with the name n.
func (ix *Index) Lookup(n string) (*Decl, bool) {
	if ix.byName == nil {
		ix.byName = make(map[string]int, len(ix.Decls))
		for i := range ix.Decls {
			ix.byName[ix.Decls[i].Name] = i
		}
	}
	i, ok := ix.byName[n]
	if !ok {
		return nil, false
	}
	return &ix.Decls[i], true
}

// UsedBy counts, for each declaration, the declarations of other modules whose type or value use it.
func (ix *Index) UsedBy() map[string]int {
	out := map[string]int{}
	for i := range ix.Decls {
		d := &ix.Decls[i]
		for _, u := range d.Uses {
			if t, ok := ix.Lookup(u); ok && t.Module != d.Module {
				out[u]++
			}
		}
	}
	return out
}
