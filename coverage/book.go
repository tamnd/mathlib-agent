package coverage

import (
	"io/fs"
	"os"
	"strings"

	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/stale"
)

// Book is what the coverage numbers read from the front matter of the handbook: the home sections of each module, and the depth at which the sections explain each name.
type Book struct {
	Sections int
	Homes    map[string][]string // module to the ids of the sections that list it in modules
	Auto     map[string]bool     // module that a section with "E1: auto" lists
	Treated  map[string]string   // name to the highest depth of the explains lists that have it
}

var rank = map[string]int{depth.E0: 0, depth.E1: 1, depth.E2: 2, depth.E3: 3}

// LoadBook reads the section files of a checkout of tamnd/mathlib-handbook. A section with the status planned does not count.
func LoadBook(dir string) (*Book, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	b := &Book{Homes: map[string][]string{}, Auto: map[string]bool{}, Treated: map[string]string{}}
	err = fs.WalkDir(root.FS(), "content", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") || stale.ID(path) == "" {
			return err
		}
		src, err := root.ReadFile(path)
		if err != nil {
			return err
		}
		s, ok, err := stale.Parse(path, string(src))
		if err != nil || !ok || s.Status == "planned" {
			return err
		}
		b.add(s)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Book) add(s stale.Section) {
	b.Sections++
	for _, m := range s.Modules {
		b.Homes[m] = append(b.Homes[m], s.ID)
		if s.Auto {
			b.Auto[m] = true
		}
	}
	for dp, names := range s.ByDepth {
		for _, n := range names {
			if old, ok := b.Treated[n]; !ok || rank[dp] > rank[old] {
				b.Treated[n] = dp
			}
		}
	}
}

// treatment gives the depth at which the book explains a declaration, and false when the book does not explain it. A name in an explains list has the depth of the list. A declaration of a module with a home section has E1 when the section has "E1: auto", and E0 otherwise (spec 04.4: E0 is the family record of the module).
func (b *Book) treatment(name, module string) (string, bool) {
	if dp, ok := b.Treated[name]; ok {
		return dp, true
	}
	if b.Auto[module] {
		return depth.E1, true
	}
	if len(b.Homes[module]) > 0 {
		return depth.E0, true
	}
	return "", false
}
