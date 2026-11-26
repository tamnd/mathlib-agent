package blueprint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Milestone is one row of blueprint/milestones.yaml.
type Milestone struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Target  string `yaml:"target"`
	Release string `yaml:"release"`
	// Closed is the date when the milestone closed. An open milestone has no date.
	Closed string `yaml:"closed,omitempty"`
	// Volumes are the volumes that the milestone works on. They set the weight of a volume in the work queue (spec 11.5).
	Volumes []string `yaml:"volumes,omitempty,flow"`
}

// Current gives the first milestone that is not closed, or nil when all are closed.
func Current(ms []Milestone) *Milestone {
	for i := range ms {
		if ms[i].Closed == "" {
			return &ms[i]
		}
	}
	return nil
}

// Find gives the milestone with the id, or nil.
func Find(ms []Milestone, id string) *Milestone {
	for i := range ms {
		if ms[i].ID == id {
			return &ms[i]
		}
	}
	return nil
}

// Weight gives the weight of a volume in the work queue when cur is the current milestone (spec 11.5): 1 for a volume of the current milestone, 0.5 for a volume of the next one, and 0.2 for the other volumes.
func Weight(ms []Milestone, cur, vol string) float64 {
	for i := range ms {
		if ms[i].ID != cur {
			continue
		}
		if slices.Contains(ms[i].Volumes, vol) {
			return 1
		}
		if i+1 < len(ms) && slices.Contains(ms[i+1].Volumes, vol) {
			return 0.5
		}
	}
	return 0.2
}

// LoadMilestones reads blueprint/milestones.yaml.
func LoadMilestones(path string) ([]Milestone, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var f struct {
		Milestones []Milestone `yaml:"milestones"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return f.Milestones, nil
}

var (
	msID    = regexp.MustCompile(`^M[0-9]+$`)
	release = regexp.MustCompile(`^v[0-9]+\.([0-9]+|x)\.[0-9]+$|^v[0-9]+\.x$`)
)

// CheckMilestones returns the faults of the milestones: ids out of order, a title that does not start with the id, a target that is not a date or "open", a release that is not a version, a closed date that is not a date, a closed milestone after an open one, and a volume id that is not V00 to V99.
func CheckMilestones(ms []Milestone) []string {
	var out []string
	var last time.Time
	open := ""
	for i, m := range ms {
		if m.Closed != "" {
			if _, err := time.Parse(time.DateOnly, m.Closed); err != nil {
				out = append(out, fmt.Sprintf("the closed date of %s is %q; want YYYY-MM-DD", m.ID, m.Closed))
			}
			if open != "" {
				out = append(out, fmt.Sprintf("%s is closed but %s before it is open", m.ID, open))
			}
		} else if open == "" {
			open = m.ID
		}
		for _, v := range m.Volumes {
			if !volID.MatchString(v) {
				out = append(out, fmt.Sprintf("%s has the volume %q; want V00 to V99", m.ID, v))
			}
		}
		if !msID.MatchString(m.ID) || m.ID != fmt.Sprintf("M%d", i) {
			out = append(out, fmt.Sprintf("milestone %d has the id %q; want M%d", i, m.ID, i))
		}
		if !strings.HasPrefix(m.Title, m.ID+". ") {
			out = append(out, fmt.Sprintf("the title of %s does not start with %q", m.ID, m.ID+". "))
		}
		if m.Target != "open" {
			t, err := time.Parse("2006-01-02", m.Target)
			switch {
			case err != nil:
				out = append(out, fmt.Sprintf("the target of %s is %q; want a date or open", m.ID, m.Target))
			case !t.After(last):
				out = append(out, fmt.Sprintf("the target of %s is not after the target of the milestone before it", m.ID))
			default:
				last = t
			}
		}
		if !release.MatchString(m.Release) {
			out = append(out, fmt.Sprintf("the release of %s is %q; want a version such as v0.1.0", m.ID, m.Release))
		}
	}
	return out
}
