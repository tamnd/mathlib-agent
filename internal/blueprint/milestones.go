package blueprint

import (
	"fmt"
	"os"
	"regexp"
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
}

// LoadMilestones reads blueprint/milestones.yaml.
func LoadMilestones(path string) ([]Milestone, error) {
	b, err := os.ReadFile(path)
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

// CheckMilestones returns the faults of the milestones: ids out of order, a title that does not start with the id, a target that is not a date or "open", and a release that is not a version.
func CheckMilestones(ms []Milestone) []string {
	var out []string
	var last time.Time
	for i, m := range ms {
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
