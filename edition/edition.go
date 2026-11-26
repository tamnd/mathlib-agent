// Package edition reads and writes records/editions.json, the edition log of spec 05.7. An edition is a build of the handbook at one stable Mathlib tag v4.X.0.
package edition

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// The states of an edition (spec 05.6).
const (
	Open     = "open"
	Released = "released"
	Skipped  = "skipped"
)

// TagRe is the form of a tag that opens an edition. A Lean patch tag v4.X.1 does not open an edition (spec 05.6).
var TagRe = regexp.MustCompile(`^v4\.[0-9]+\.0$`)

// Coverage is the coverage of the handbook at the release of an edition (spec 04.5).
type Coverage struct {
	Home   float64 `json:"C-home"`
	Depth  float64 `json:"C-depth"`
	Fresh  float64 `json:"C-fresh"`
	Review float64 `json:"C-review"`
}

// Record is one edition. The fields are those of spec 05.7. The fields that the release sets are empty while the edition is open.
type Record struct {
	Edition         string            `json:"edition"`
	Mathlib         string            `json:"mathlib"`
	Lean            string            `json:"lean"`
	TagDate         string            `json:"tag_date"`
	Opened          string            `json:"opened"`
	Released        string            `json:"released,omitempty"`
	DaysFromTag     int               `json:"days_from_tag,omitempty"`
	HandbookVersion string            `json:"handbook_version,omitempty"`
	MathlibMap      string            `json:"mathlib_map,omitempty"`
	MathlibLean     string            `json:"mathlib_lean,omitempty"`
	Volumes         map[string]string `json:"volumes,omitempty"`
	Coverage        *Coverage         `json:"coverage,omitempty"`
	DriftSinceLast  map[string]int    `json:"drift_since_last,omitempty"`
	State           string            `json:"state"`
	Notes           string            `json:"notes,omitempty"`
}

// Log is the list of editions, the oldest first.
type Log []Record

// Path is the place of the log in a checkout of tamnd/mathlib.
func Path(mapDir string) string {
	return filepath.Join(mapDir, "records", "editions.json")
}

// Load reads the log. A missing file gives an empty log.
func Load(path string) (Log, error) {
	b, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var l Log
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := l.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return l, nil
}

// Save writes the log.
func (l Log) Save(path string) error {
	if l == nil {
		l = Log{}
	}
	b, err := json.MarshalIndent(l, "", " ")
	if err != nil {
		return err
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// Validate checks the log: the form of each tag and date, the state, no tag two times, and at most one open edition.
func (l Log) Validate() error {
	seen := map[string]bool{}
	open := 0
	for i, r := range l {
		if !TagRe.MatchString(r.Edition) {
			return fmt.Errorf("edition %d: the tag %q is not of the form v4.X.0", i+1, r.Edition)
		}
		if seen[r.Edition] {
			return fmt.Errorf("the edition %s is in the log two times", r.Edition)
		}
		seen[r.Edition] = true
		if len(r.Mathlib) != 40 {
			return fmt.Errorf("%s: the Mathlib commit must have 40 hex digits", r.Edition)
		}
		for _, d := range []struct{ name, value string }{{"tag_date", r.TagDate}, {"opened", r.Opened}, {"released", r.Released}} {
			if d.value == "" && d.name == "released" {
				continue
			}
			if _, err := time.Parse(time.DateOnly, d.value); err != nil {
				return fmt.Errorf("%s: the date %s %q is not of the form YYYY-MM-DD", r.Edition, d.name, d.value)
			}
		}
		switch r.State {
		case Open:
			open++
			if r.Released != "" {
				return fmt.Errorf("%s: an open edition has no release date", r.Edition)
			}
		case Released:
			if r.Released == "" {
				return fmt.Errorf("%s: a released edition must have a release date", r.Edition)
			}
		case Skipped:
		default:
			return fmt.Errorf("%s: the state %q is not open, released or skipped", r.Edition, r.State)
		}
	}
	if open > 1 {
		return fmt.Errorf("the log has %d open editions; at most one edition is open", open)
	}
	return nil
}

// Find returns the record of a tag.
func (l Log) Find(tag string) (Record, bool) {
	for _, r := range l {
		if r.Edition == tag {
			return r, true
		}
	}
	return Record{}, false
}

// Add opens a new edition. If an edition is open, it closes as skipped (spec 05.6). It returns the tag of the skipped edition, or "".
func (l Log) Add(r Record) (Log, string, error) {
	if _, ok := l.Find(r.Edition); ok {
		return l, "", fmt.Errorf("the edition %s is in the log already", r.Edition)
	}
	r.State = Open
	skipped := ""
	out := make(Log, 0, len(l)+1)
	for _, e := range l {
		if e.State == Open {
			e.State = Skipped
			skipped = e.Edition
		}
		out = append(out, e)
	}
	out = append(out, r)
	return out, skipped, out.Validate()
}

// DaysBetween gives the number of days from the date a to the date b.
func DaysBetween(a, b string) (int, error) {
	ta, err := time.Parse(time.DateOnly, a)
	if err != nil {
		return 0, err
	}
	tb, err := time.Parse(time.DateOnly, b)
	if err != nil {
		return 0, err
	}
	return int(tb.Sub(ta).Hours() / 24), nil
}
