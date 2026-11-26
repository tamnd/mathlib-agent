// Package blueprint reads the files of the map in tamnd/mathlib/blueprint and checks them.
package blueprint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Volume is one volume of the volume map.
type Volume struct {
	ID       string    `yaml:"id"`
	Title    string    `yaml:"title"`
	Part     string    `yaml:"part"`
	Level    string    `yaml:"level"`
	Prefixes []string  `yaml:"prefixes"`
	Chapters []Chapter `yaml:"chapters"`
}

// Chapter is one chapter of a volume.
type Chapter struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	Prefixes []string `yaml:"prefixes"`
}

// VolumeMap is the file blueprint/volumes.yaml (spec 09.2).
type VolumeMap struct {
	Volumes    []Volume          `yaml:"volumes"`
	Exceptions map[string]string `yaml:"exceptions"`
	Unmapped   string            `yaml:"unmapped"`

	owner map[string]string // prefix to volume id
	order []string          // prefixes, longest first
}

// LoadVolumes reads a volume map.
func LoadVolumes(path string) (*VolumeMap, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return ParseVolumes(b)
}

// ParseVolumes parses a volume map. It returns an error when two volumes claim the same prefix.
func ParseVolumes(b []byte) (*VolumeMap, error) {
	var m VolumeMap
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m.owner = map[string]string{}
	for _, v := range m.Volumes {
		for _, p := range v.Prefixes {
			p = strings.TrimSuffix(p, "/")
			if o, ok := m.owner[p]; ok {
				return nil, fmt.Errorf("the prefix %s is in %s and in %s", p, o, v.ID)
			}
			m.owner[p] = v.ID
			m.order = append(m.order, p)
		}
	}
	sort.SliceStable(m.order, func(i, j int) bool { return len(m.order[i]) > len(m.order[j]) })
	return &m, nil
}

// matches reports whether the prefix p matches the module path mod.
func matches(p, mod string) bool {
	return mod == p || strings.HasPrefix(mod, p+"/")
}

// Home returns the home volume of a module path such as Mathlib/Topology/Basic, and the prefix that gives it. An exception wins over the prefixes. The bool is false when the module has no home.
func (m *VolumeMap) Home(mod string) (vol, prefix string, ok bool) {
	if e, ok := m.Exceptions[mod]; ok {
		vol, _, _ = strings.Cut(e, ".")
		return vol, mod, true
	}
	for _, p := range m.order {
		if matches(p, mod) {
			return m.owner[p], p, true
		}
	}
	return "", "", false
}

// Report is the result of a check of the volume map against a list of modules.
type Report struct {
	Modules  int
	Orphans  []string       // modules with no home volume
	Unused   []string       // prefixes that give a home to no module
	PerVol   map[string]int // modules per volume
	Problems []string       // faults in the map itself
}

var volID = regexp.MustCompile(`^V[0-9]{2}$`)

// Check checks the map itself and then the map against the modules.
func (m *VolumeMap) Check(modules []string) Report {
	r := Report{Modules: len(modules), PerVol: map[string]int{}}
	ids := map[string]bool{}
	for _, v := range m.Volumes {
		if !volID.MatchString(v.ID) {
			r.Problems = append(r.Problems, fmt.Sprintf("the volume id %q is not of the form V00", v.ID))
		}
		if ids[v.ID] {
			r.Problems = append(r.Problems, fmt.Sprintf("the volume id %s is used two times", v.ID))
		}
		ids[v.ID] = true
		if v.Title == "" {
			r.Problems = append(r.Problems, fmt.Sprintf("the volume %s has no title", v.ID))
		}
		for _, p := range v.Prefixes {
			if !strings.HasPrefix(p, "Mathlib/") {
				r.Problems = append(r.Problems, fmt.Sprintf("the prefix %s of %s does not start with Mathlib/", p, v.ID))
			}
		}
	}
	for mod, e := range m.Exceptions {
		vol, _, _ := strings.Cut(e, ".")
		if !ids[vol] {
			r.Problems = append(r.Problems, fmt.Sprintf("the exception %s points to %s, which is not a volume", mod, e))
		}
	}
	used := map[string]bool{}
	for _, mod := range modules {
		vol, p, ok := m.Home(mod)
		if !ok {
			r.Orphans = append(r.Orphans, mod)
			continue
		}
		used[p] = true
		r.PerVol[vol]++
	}
	for _, p := range m.order {
		if !used[p] {
			r.Unused = append(r.Unused, p)
		}
	}
	sort.Strings(r.Orphans)
	sort.Strings(r.Unused)
	sort.Strings(r.Problems)
	return r
}

// OK reports whether the check found no fault.
func (r Report) OK() bool {
	return len(r.Orphans) == 0 && len(r.Unused) == 0 && len(r.Problems) == 0
}

// Modules walks a Mathlib checkout and returns the module paths of the .lean files under Mathlib/, such as Mathlib/Topology/Basic.
func Modules(root string) ([]string, error) {
	var out []string
	base := filepath.Join(root, "Mathlib")
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".lean") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(strings.TrimSuffix(rel, ".lean")))
		return nil
	})
	sort.Strings(out)
	return out, err
}
