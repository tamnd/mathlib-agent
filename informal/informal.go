// Package informal reads and checks the E1 records of informal/<vol>.jsonl (spec 09.3). An E1 record is one sentence in words for one declaration, written for the type hash of the declaration at one commit. When the hash changes, the record is stale and goes back to the queue of the informalizer.
package informal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tamnd/mathlib-agent/index"
)

// The results of a check. An empty result means that the check has not run.
const (
	Pass = "pass"
	Fail = "fail"
)

// Checks are the three checks of spec 14.3.
type Checks struct {
	Roundtrip string `json:"roundtrip,omitempty"`
	Critic    string `json:"critic,omitempty"`
	Human     string `json:"human,omitempty"`
}

// Record is one line of informal/<vol>.jsonl.
type Record struct {
	Name     string `json:"name"`
	TypeHash string `json:"typeHash"`
	Text     string `json:"text"`
	By       string `json:"by"`
	Run      string `json:"run,omitempty"`
	Checks   Checks `json:"checks"`
	Since    string `json:"since"`
	// Formerly lists the old names of the declaration, after mlib rename (spec 05.4).
	Formerly []string `json:"formerly,omitempty"`
}

// Path is the place of the records of a volume in a checkout of tamnd/mathlib.
func Path(mapDir, vol string) string {
	return filepath.Join(mapDir, "informal", vol+".jsonl")
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{8,64}$`)

// Validate checks the form of one record.
func (r Record) Validate() error {
	switch {
	case r.Name == "":
		return errors.New("the record has no name")
	case !hashRe.MatchString(r.TypeHash):
		return fmt.Errorf("%s: the type hash %q is not hex", r.Name, r.TypeHash)
	case strings.TrimSpace(r.Text) == "":
		return fmt.Errorf("%s: the record has no text", r.Name)
	case strings.Contains(r.Text, "\n"):
		return fmt.Errorf("%s: the text is one sentence on one line", r.Name)
	case r.By == "":
		return fmt.Errorf("%s: the record does not say who wrote it", r.Name)
	case r.Since == "":
		return fmt.Errorf("%s: the record has no commit in since", r.Name)
	}
	for _, c := range []string{r.Checks.Roundtrip, r.Checks.Critic, r.Checks.Human} {
		if c != "" && c != Pass && c != Fail {
			return fmt.Errorf("%s: the check result %q is not pass or fail", r.Name, c)
		}
	}
	return nil
}

// Load reads the records of a volume. A missing file gives no records. A name two times is a fault.
func Load(path string) ([]Record, error) {
	f, err := os.Open(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []Record
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if seen[r.Name] {
			return nil, fmt.Errorf("%s:%d: %s has two records", path, n, r.Name)
		}
		seen[r.Name] = true
		out = append(out, r)
	}
	return out, sc.Err()
}

// Save writes the records in name order, one JSON line each.
func Save(path string, rs []Record) error {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Name < rs[j].Name })
	var b strings.Builder
	for _, r := range rs {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// Stale is a record that does not match the index.
type Stale struct {
	Record Record
	// Reason is "gone" when the name is not in the index, or "type" when the type hash changed.
	Reason  string
	NewHash string
}

// FindStale gives the records whose name is gone from the index or whose type hash changed.
func FindStale(ix *index.Index, rs []Record) []Stale {
	var out []Stale
	for _, r := range rs {
		d, ok := ix.Lookup(r.Name)
		switch {
		case !ok:
			out = append(out, Stale{Record: r, Reason: "gone"})
		case d.TypeHash != r.TypeHash:
			out = append(out, Stale{Record: r, Reason: "type", NewHash: d.TypeHash})
		}
	}
	return out
}

// Status is the state of the E1 records of one volume.
type Status struct {
	Volume string `json:"volume"`
	// Needed is the number of declarations of the volume at depth E1.
	Needed int `json:"needed"`
	// Fresh records match the type hash of the index. Stale records do not. Missing declarations have no record.
	Fresh   int `json:"fresh"`
	Stale   int `json:"stale"`
	Missing int `json:"missing"`
	// The counts of each check over the fresh records.
	Roundtrip map[string]int `json:"roundtrip"`
	Critic    map[string]int `json:"critic"`
	Human     map[string]int `json:"human"`
}

// Share gives the part of the needed declarations that have a fresh record.
func (s Status) Share() float64 {
	if s.Needed == 0 {
		return 1
	}
	return float64(s.Fresh) / float64(s.Needed)
}

// Compute gives the status of a volume. need is the set of the names at depth E1 whose home is the volume.
func Compute(vol string, ix *index.Index, need map[string]bool, rs []Record) Status {
	s := Status{Volume: vol, Needed: len(need), Roundtrip: map[string]int{}, Critic: map[string]int{}, Human: map[string]int{}}
	have := map[string]bool{}
	for _, r := range rs {
		d, ok := ix.Lookup(r.Name)
		if !ok || d.TypeHash != r.TypeHash {
			s.Stale++
			continue
		}
		if !need[r.Name] {
			continue
		}
		have[r.Name] = true
		s.Fresh++
		count(s.Roundtrip, r.Checks.Roundtrip)
		count(s.Critic, r.Checks.Critic)
		count(s.Human, r.Checks.Human)
	}
	s.Missing = len(need) - len(have)
	return s
}

func count(m map[string]int, v string) {
	if v == "" {
		v = "none"
	}
	m[v]++
}

// Sample gives n records chosen at random with the seed, in name order, for the human sample of spec 14.3.
func Sample(rs []Record, n int, seed uint64) []Record {
	cp := append([]Record{}, rs...)
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) // #nosec G404 -- the sample needs to be repeatable, not secret
	r.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	if n < len(cp) {
		cp = cp[:n]
	}
	sort.Slice(cp, func(i, j int) bool { return cp[i].Name < cp[j].Name })
	return cp
}
