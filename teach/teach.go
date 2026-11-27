// Package teach reads the reading paths of the map and the exercises of the handbook, for the pages /teach/ of the site (spec 13.1).
//
// The reading paths are in blueprint/paths.yaml of tamnd/mathlib (spec 06.6). The exercises are the numbered items of the part "### Exercises" of each section file of tamnd/mathlib-handbook. Each item starts with its grade: (A) routine, (B) needs an idea, (C) hard, (L) in Lean (spec 07.2).
package teach

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Grades are the exercise grades of spec 07.2, in order.
var Grades = []string{"A", "B", "C", "L"}

// GradeName gives the meaning of a grade.
var GradeName = map[string]string{"A": "routine", "B": "needs an idea", "C": "hard", "L": "in Lean"}

// Step is one volume of a reading path. Chapters is [first, last] when the path takes only some chapters, and empty for the whole volume.
type Step struct {
	Volume   string `yaml:"volume"`
	Chapters []int  `yaml:"chapters"`
}

// Range gives the chapters of the step, or "" for the whole volume.
func (s Step) Range() string {
	if len(s.Chapters) != 2 {
		return ""
	}
	return fmt.Sprintf("Ch. %d to %d", s.Chapters[0], s.Chapters[1])
}

// Takes is true when the step takes the chapter n of its volume.
func (s Step) Takes(n int) bool {
	return len(s.Chapters) != 2 || n >= s.Chapters[0] && n <= s.Chapters[1]
}

// Path is one reading path of spec 06.6.
type Path struct {
	ID    string   `yaml:"id"`
	Title string   `yaml:"title"`
	For   string   `yaml:"for"`
	After []string `yaml:"after"`
	Steps []Step   `yaml:"steps"`
	Note  string   `yaml:"note"`
}

// LoadPaths reads blueprint/paths.yaml. A missing file gives no paths.
func LoadPaths(path string) ([]Path, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f struct {
		Paths []Path `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	ids := map[string]bool{}
	for _, p := range f.Paths {
		if p.ID == "" || ids[p.ID] {
			return nil, fmt.Errorf("%s: the path id %q is empty or not unique", path, p.ID)
		}
		ids[p.ID] = true
		for _, s := range p.Steps {
			if len(s.Chapters) != 0 && (len(s.Chapters) != 2 || s.Chapters[0] < 1 || s.Chapters[1] < s.Chapters[0]) {
				return nil, fmt.Errorf("%s: path %s, %s: chapters must be [first, last]", path, p.ID, s.Volume)
			}
		}
	}
	for _, p := range f.Paths {
		for _, a := range p.After {
			if !ids[a] {
				return nil, fmt.Errorf("%s: path %s comes after %s, which is not a path", path, p.ID, a)
			}
		}
	}
	return f.Paths, nil
}

// Exercise is one exercise of a section.
type Exercise struct {
	N     int
	Grade string
	Text  string
}

// Section is a section of the handbook with its exercises.
type Section struct {
	ID        string // V05.C04.S01
	Title     string
	Status    string
	Exercises []Exercise
}

// Count gives the number of exercises of each grade.
func (s *Section) Count() map[string]int {
	out := map[string]int{}
	for _, e := range s.Exercises {
		out[e.Grade]++
	}
	return out
}

// Chapter is a chapter of a volume.
type Chapter struct {
	N        int
	Title    string
	Sections []*Section
}

// Count gives the number of exercises of each grade in the chapter.
func (c *Chapter) Count() map[string]int {
	out := map[string]int{}
	for _, s := range c.Sections {
		for g, n := range s.Count() {
			out[g] += n
		}
	}
	return out
}

// Volume is a volume of the handbook that has text.
type Volume struct {
	ID       string // V05
	Dir      string // 05-groups
	Title    string
	Chapters []*Chapter
}

// Count gives the number of exercises of each grade in the volume.
func (v *Volume) Count() map[string]int {
	out := map[string]int{}
	for _, c := range v.Chapters {
		for g, n := range c.Count() {
			out[g] += n
		}
	}
	return out
}

// Book is the text of the handbook, by volume id.
type Book map[string]*Volume

var (
	itemRE  = regexp.MustCompile(`^(\d+)\. \(([ABCL])\) (.+)$`)
	titleRE = regexp.MustCompile(`(?m)^title: *"?(.*?)"? *$`)
	statRE  = regexp.MustCompile(`(?m)^status: *(\S+)`)
	headRE  = regexp.MustCompile(`(?m)^# (.+)$`)
)

// num reads the number of a folder or file name NN-slug.
func num(name string) (int, bool) {
	if len(name) < 3 || name[2] != '-' {
		return 0, false
	}
	n, err := strconv.Atoi(name[:2])
	return n, err == nil
}

// LoadBook reads the folder content/ of a checkout of tamnd/mathlib-handbook. An empty dir gives an empty book.
func LoadBook(dir string) (Book, error) {
	book := Book{}
	if dir == "" {
		return book, nil
	}
	content := filepath.Join(dir, "content")
	vols, err := os.ReadDir(content)
	if err != nil {
		return nil, err
	}
	for _, ve := range vols {
		vn, ok := num(ve.Name())
		if !ok || !ve.IsDir() {
			continue
		}
		v := &Volume{ID: fmt.Sprintf("V%02d", vn), Dir: ve.Name()}
		v.Title = firstHeading(filepath.Join(content, ve.Name(), "intro.md"), ve.Name()[3:])
		chs, err := os.ReadDir(filepath.Join(content, ve.Name()))
		if err != nil {
			return nil, err
		}
		for _, ce := range chs {
			cn, ok := num(ce.Name())
			if !ok || !ce.IsDir() {
				continue
			}
			cdir := filepath.Join(content, ve.Name(), ce.Name())
			c := &Chapter{N: cn, Title: firstHeading(filepath.Join(cdir, "intro.md"), ce.Name()[3:])}
			files, err := os.ReadDir(cdir)
			if err != nil {
				return nil, err
			}
			for _, fe := range files {
				sn, ok := num(fe.Name())
				if !ok || fe.IsDir() || !strings.HasSuffix(fe.Name(), ".md") {
					continue
				}
				b, err := os.ReadFile(filepath.Clean(filepath.Join(cdir, fe.Name())))
				if err != nil {
					return nil, err
				}
				c.Sections = append(c.Sections, parseSection(fmt.Sprintf("%s.C%02d.S%02d", v.ID, cn, sn), string(b)))
			}
			v.Chapters = append(v.Chapters, c)
		}
		sort.Slice(v.Chapters, func(i, j int) bool { return v.Chapters[i].N < v.Chapters[j].N })
		book[v.ID] = v
	}
	return book, nil
}

func firstHeading(path, fallback string) string {
	b, err := os.ReadFile(filepath.Clean(path))
	if err == nil {
		if m := headRE.FindStringSubmatch(string(b)); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return strings.ReplaceAll(fallback, "-", " ")
}

// parseSection reads the title, the status and the exercises of a section file.
func parseSection(id, src string) *Section {
	s := &Section{ID: id}
	body := src
	if strings.HasPrefix(src, "---\n") {
		if i := strings.Index(src[4:], "\n---\n"); i >= 0 {
			fm := src[4 : 4+i]
			body = src[4+i+5:]
			if m := titleRE.FindStringSubmatch(fm); m != nil {
				s.Title = m[1]
			}
			if m := statRE.FindStringSubmatch(fm); m != nil {
				s.Status = m[1]
			}
		}
	}
	in := false
	for line := range strings.SplitSeq(body, "\n") {
		switch {
		case strings.HasPrefix(line, "### "):
			in = strings.TrimSpace(line[4:]) == "Exercises"
		case strings.HasPrefix(line, "## "):
			in = false
		case in:
			if m := itemRE.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				s.Exercises = append(s.Exercises, Exercise{N: n, Grade: m[2], Text: m[3]})
			}
		}
	}
	return s
}
