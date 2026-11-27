package site

import (
	"fmt"
	"sort"

	"github.com/tamnd/mathlib-agent/teach"
)

// The pages /teach/ of spec 13.1: the reading paths as courses, and the exercises by grade.

type gradeCount struct {
	Grade, Name string
	N           int
}

func counts(m map[string]int) []gradeCount {
	out := make([]gradeCount, 0, len(teach.Grades))
	for _, g := range teach.Grades {
		out = append(out, gradeCount{g, teach.GradeName[g], m[g]})
	}
	return out
}

func total(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// stepView is one volume of a course.
type stepView struct {
	Volume   string
	Title    string
	Range    string
	Written  bool // the handbook has text for the volume
	Known    bool // the volume map has the volume, so it has an area page
	Chapters []chapterView
	Sections int
	Counts   []gradeCount
	Total    int
}

type chapterView struct {
	N         int
	Title     string
	Sections  []*teach.Section
	Counts    []gradeCount
	Total     int
	Statuses  string
	Volume    string
	Exercises bool
}

type courseView struct {
	teach.Path
	Before   []teach.Path
	Steps    []stepView
	Written  int
	Sections int
	Counts   []gradeCount
	Total    int
}

type exerciseVolume struct {
	ID, Title string
	Chapters  []chapterView
	Counts    []gradeCount
	Total     int
	ByGrade   []gradeBlock
}

type gradeBlock struct {
	Grade, Name string
	Items       []exerciseItem
}

type exerciseItem struct {
	Section, Title string
	N              int
	Text           string
}

func chapterOf(vol string, c *teach.Chapter) chapterView {
	m := c.Count()
	st := map[string]int{}
	for _, s := range c.Sections {
		st[s.Status]++
	}
	keys := make([]string, 0, len(st))
	for k := range st {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := ""
	for i, k := range keys {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%d %s", st[k], k)
	}
	return chapterView{N: c.N, Title: c.Title, Sections: c.Sections, Counts: counts(m), Total: total(m), Statuses: s, Volume: vol, Exercises: total(m) > 0}
}

// course makes the view of one path.
func (b *builder) course(p teach.Path, byID map[string]teach.Path, byVol map[string]*volRow) courseView {
	cv := courseView{Path: p}
	for _, a := range p.After {
		cv.Before = append(cv.Before, byID[a])
	}
	all := map[string]int{}
	for _, s := range p.Steps {
		sv := stepView{Volume: s.Volume, Range: s.Range(), Title: s.Volume}
		if v := byVol[s.Volume]; v != nil {
			sv.Title, sv.Known = v.Title, true
		}
		m := map[string]int{}
		if v := b.in.Book[s.Volume]; v != nil {
			sv.Written = true
			cv.Written++
			for _, c := range v.Chapters {
				if !s.Takes(c.N) {
					continue
				}
				sv.Chapters = append(sv.Chapters, chapterOf(s.Volume, c))
				sv.Sections += len(c.Sections)
				for g, n := range c.Count() {
					m[g] += n
					all[g] += n
				}
			}
		}
		sv.Counts, sv.Total = counts(m), total(m)
		cv.Sections += sv.Sections
		cv.Steps = append(cv.Steps, sv)
	}
	cv.Counts, cv.Total = counts(all), total(all)
	return cv
}

// exercises makes the view of the exercises of one volume, by chapter and by grade.
func exercises(v *teach.Volume) exerciseVolume {
	ev := exerciseVolume{ID: v.ID, Title: v.Title}
	m := v.Count()
	ev.Counts, ev.Total = counts(m), total(m)
	byGrade := map[string][]exerciseItem{}
	for _, c := range v.Chapters {
		ev.Chapters = append(ev.Chapters, chapterOf(v.ID, c))
		for _, s := range c.Sections {
			for _, e := range s.Exercises {
				byGrade[e.Grade] = append(byGrade[e.Grade], exerciseItem{Section: s.ID, Title: s.Title, N: e.N, Text: e.Text})
			}
		}
	}
	for _, g := range teach.Grades {
		if len(byGrade[g]) > 0 {
			ev.ByGrade = append(ev.ByGrade, gradeBlock{Grade: g, Name: teach.GradeName[g], Items: byGrade[g]})
		}
	}
	return ev
}

// teachPages gives the pages /teach/, /teach/<path>/, /teach/exercises/ and /teach/exercises/<volume>/.
func (b *builder) teachPages(byVol map[string]*volRow) []namedPage {
	if len(b.in.Paths) == 0 {
		return nil
	}
	byID := map[string]teach.Path{}
	for _, p := range b.in.Paths {
		byID[p.ID] = p
	}
	var courses []courseView
	var out []namedPage
	for _, p := range b.in.Paths {
		cv := b.course(p, byID, byVol)
		courses = append(courses, cv)
		out = append(out, namedPage{"course", page{
			Path: "teach/" + p.ID + "/", Section: "teach", Title: "Path " + p.ID + ". " + p.Title,
			Description: fmt.Sprintf("The reading path %s of the Mathlib Handbook as a course, for %s: the volumes, the chapters and the exercises by grade.", p.ID, p.For),
			Data:        cv,
		}})
	}
	ids := make([]string, 0, len(b.in.Book))
	for id := range b.in.Book {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var vols []exerciseVolume
	for _, id := range ids {
		ev := exercises(b.in.Book[id])
		vols = append(vols, ev)
		out = append(out, namedPage{"exvolume", page{
			Path: "teach/exercises/" + id + "/", Section: "teach", Title: "The exercises of " + id + ". " + ev.Title,
			Description: fmt.Sprintf("The %d exercises of volume %s of the Mathlib Handbook, by grade.", ev.Total, id),
			Data:        ev,
		}})
	}
	all := map[string]int{}
	for _, id := range ids {
		for g, n := range b.in.Book[id].Count() {
			all[g] += n
		}
	}
	out = append(out,
		namedPage{"teach", page{
			Path: "teach/", Section: "teach", Title: "Teach",
			Description: "The reading paths of the Mathlib Handbook as courses, and the exercises by grade, for teachers.",
			Data:        struct{ Courses []courseView }{courses},
		}},
		namedPage{"exercises", page{
			Path: "teach/exercises/", Section: "teach", Title: "The exercises by grade",
			Description: "The exercises of the Mathlib Handbook, by volume, chapter and grade.",
			Data: struct {
				Volumes []exerciseVolume
				Counts  []gradeCount
				Total   int
			}{vols, counts(all), total(all)},
		}},
	)
	return out
}

type namedPage struct {
	name string
	p    page
}
