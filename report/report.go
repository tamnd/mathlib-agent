// Package report writes the weekly report and the edition report of reports/ (spec 09.1, 05.6). A report reads only the records of the map: the drift records, the coverage, the edition log, the run records and the ledger. It writes Markdown with one paragraph on one line.
package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/agent"
	"github.com/tamnd/mathlib-agent/coverage"
	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/edition"
	"github.com/tamnd/mathlib-agent/rename"
)

// AlertDays is the age of the last sync after which the weekly report raises an alert (spec 05.3).
const AlertDays = 30

// MaxRenames is the number of renames that a report lists.
const MaxRenames = 20

// Input is the records that a report reads.
type Input struct {
	Drift    []drift.Record
	Coverage *coverage.Record
	Editions edition.Log
	Runs     []*agent.Run
	Ledger   []agent.Entry
	// Tool is the name and the version of the tool, for the first line.
	Tool string
}

// Load reads the records of the map.
func Load(mapDir, tool string) (Input, error) {
	in := Input{Tool: tool}
	var err error
	if in.Drift, err = rename.LoadDrift(mapDir); err != nil {
		return in, err
	}
	b, err := os.ReadFile(filepath.Join(mapDir, "records", "coverage.json")) // #nosec G304 -- a record of the map
	switch {
	case err == nil:
		var c coverage.Record
		if err := json.Unmarshal(b, &c); err != nil {
			return in, fmt.Errorf("records/coverage.json: %w", err)
		}
		in.Coverage = &c
	case !errors.Is(err, fs.ErrNotExist):
		return in, err
	}
	if in.Editions, err = edition.Load(edition.Path(mapDir)); err != nil {
		return in, err
	}
	if in.Runs, err = agent.LoadRuns(mapDir); err != nil {
		return in, err
	}
	in.Ledger, err = agent.LoadLedger(mapDir, in.Runs)
	return in, err
}

var weekRe = regexp.MustCompile(`^([0-9]{4})-W([0-9]{2})$`)

// Week gives the Monday and the Sunday of an ISO week such as 2027-W03.
func Week(w string) (time.Time, time.Time, error) {
	m := weekRe.FindStringSubmatch(w)
	if m == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("the week %q is not of the form 2027-W03", w)
	}
	y, _ := strconv.Atoi(m[1])
	n, _ := strconv.Atoi(m[2])
	// 4 January is always in week 1.
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, time.UTC)
	mon := jan4.AddDate(0, 0, -((int(jan4.Weekday())+6)%7)+7*(n-1))
	if yy, nn := mon.ISOWeek(); yy != y || nn != n {
		return time.Time{}, time.Time{}, fmt.Errorf("the year %d has no week %d", y, n)
	}
	return mon, mon.AddDate(0, 0, 6), nil
}

// WeekOf gives the ISO week of a date, such as 2027-W03.
func WeekOf(t time.Time) string {
	y, n := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, n)
}

func day(t time.Time) string { return t.Format(time.DateOnly) }

func short(c string) string {
	if len(c) > 10 {
		return c[:10]
	}
	return c
}

func in(d string, a, b time.Time) bool {
	return d >= day(a) && d <= day(b)
}

// Weekly writes the weekly report of a week.
func Weekly(rec Input, week string) (string, error) {
	mon, sun, err := Week(week)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Weekly report %s\n\nThe week is from %s to %s. %s wrote this report from the records of tamnd/mathlib.\n", week, day(mon), day(sun), rec.Tool)

	b.WriteString("\n## The sync\n\n")
	var last *drift.Record
	var inWeek []drift.Record
	for i := range rec.Drift {
		r := &rec.Drift[i]
		if r.Date <= day(sun) && (last == nil || r.Date > last.Date) {
			last = r
		}
		if in(r.Date, mon, sun) {
			inWeek = append(inWeek, *r)
		}
	}
	if last == nil {
		b.WriteString("No sync ran up to the end of the week.\n")
	} else {
		t, _ := time.Parse(time.DateOnly, last.Date)
		age := int(sun.Sub(t).Hours() / 24)
		fmt.Fprintf(&b, "The last sync ran on %s, from %s to %s. At the end of the week it is %d days old. In the week, the number of syncs is %d.\n", last.Date, short(last.From), short(last.To), age, len(inWeek))
		if age > AlertDays {
			fmt.Fprintf(&b, "\n**Alert.** The last sync is more than %d days old. The edge must never fall more than 5 months behind master, because Mathlib deletes the deprecated aliases after 6 months (spec 05.3). The porter work goes to the top of the queue.\n", AlertDays)
		}
	}
	driftSection(&b, inWeek)

	if c := rec.Coverage; c != nil {
		fmt.Fprintf(&b, "\n## The coverage\n\nThe coverage of %s at Mathlib %s has %d declarations in %d modules. C-home is %.1f%% and C-depth is %.1f%% (spec 04.5).\n\n", c.Date, short(c.Mathlib), c.Total.Decls, c.Total.Modules, 100*c.Total.CHome, 100*c.Total.CDepth)
		b.WriteString("| Volume | Modules | Declarations | C-home | C-depth |\n|---|---|---|---|---|\n")
		for _, v := range c.Volumes {
			fmt.Fprintf(&b, "| %s | %d | %d | %.1f%% | %.1f%% |\n", v.Volume, v.Modules, v.Decls, 100*v.CHome, 100*v.CDepth)
		}
	}

	runsSection(&b, rec, mon, sun)

	var eds []string
	for _, e := range rec.Editions {
		for _, d := range []struct{ what, date string }{{"tag", e.TagDate}, {"opened", e.Opened}, {"released", e.Released}} {
			if d.date != "" && in(d.date, mon, sun) {
				eds = append(eds, fmt.Sprintf("- %s: %s on %s.", e.Edition, d.what, d.date))
			}
		}
	}
	b.WriteString("\n## The editions\n\n")
	if len(eds) == 0 {
		b.WriteString("No edition changed its state in the week.\n")
	} else {
		b.WriteString(strings.Join(eds, "\n") + "\n")
	}
	return b.String(), nil
}

// driftSection writes the counts of the drift classes and the renames of the records.
func driftSection(b *strings.Builder, rs []drift.Record) {
	b.WriteString("\n## The drift\n\n")
	counts := map[string]int{}
	for _, r := range rs {
		for k, v := range r.Counts {
			counts[k] += v
		}
	}
	if len(counts) == 0 {
		b.WriteString("The drift records have no items.\n")
		return
	}
	var ks []string
	for k := range counts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	b.WriteString("| Class | Items |\n|---|---|\n")
	for _, k := range ks {
		fmt.Fprintf(b, "| %s | %d |\n", k, counts[k])
	}
	t := rename.Table(rs, false)
	if len(t) == 0 {
		return
	}
	fmt.Fprintf(b, "\nThe renames and the moves (%d). `mlib rename` rewrites them in the handbook and the companion.\n\n", len(t))
	for i, e := range t {
		if i == MaxRenames {
			fmt.Fprintf(b, "- and %d more.\n", len(t)-MaxRenames)
			break
		}
		fmt.Fprintf(b, "- `%s` to `%s` (%s)\n", e.Old, e.New, e.Kind)
	}
}

// runsSection writes the runs by role and status and the spend by role.
func runsSection(b *strings.Builder, rec Input, from, to time.Time) {
	b.WriteString("\n## The runs and the spend\n\n")
	type row struct{ planned, ran, failed, passed int }
	rows := map[string]*row{}
	for _, r := range rec.Runs {
		if !in(r.Date, from, to) {
			continue
		}
		x := rows[r.Role]
		if x == nil {
			x = &row{}
			rows[r.Role] = x
		}
		switch r.Status {
		case agent.Planned:
			x.planned++
		case agent.Ran:
			x.ran++
		case agent.Failed:
			x.failed++
		case agent.Passed:
			x.passed++
		}
	}
	spend := map[string]float64{}
	total, tokens := 0.0, 0
	for _, e := range rec.Ledger {
		if in(e.Date, from, to) {
			spend[e.Role] += e.Cost
			total += e.Cost
			tokens += e.TokensIn + e.TokensOut
		}
	}
	if len(rows) == 0 && len(spend) == 0 {
		b.WriteString("No role ran.\n")
		return
	}
	b.WriteString("| Role | Planned | Ran | Failed | Passed | Spend |\n|---|---|---|---|---|---|\n")
	for _, r := range agent.Roles {
		x, s := rows[r.Name], spend[r.Name]
		if x == nil && s == 0 {
			continue
		}
		if x == nil {
			x = &row{}
		}
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %.2f |\n", r.Name, x.planned, x.ran, x.failed, x.passed, s)
	}
	fmt.Fprintf(b, "\nThe spend is %.2f dollars for %d tokens.\n", total, tokens)
}

// Edition writes the report of an edition (spec 05.6).
func Edition(rec Input, tag string) (string, error) {
	e, ok := rec.Editions.Find(tag)
	if !ok {
		return "", fmt.Errorf("records/editions.json has no edition %s", tag)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Edition %s\n\n%s wrote this report from the records of tamnd/mathlib.\n\n", e.Edition, rec.Tool)
	b.WriteString("| Field | Value |\n|---|---|\n")
	for _, f := range [][2]string{
		{"State", e.State}, {"Mathlib", e.Mathlib}, {"Lean", e.Lean}, {"Tag date", e.TagDate}, {"Opened", e.Opened}, {"Released", e.Released},
		{"Handbook version", e.HandbookVersion}, {"mathlib-lean", e.MathlibLean},
	} {
		if f[1] != "" {
			fmt.Fprintf(&b, "| %s | %s |\n", f[0], f[1])
		}
	}
	if e.Released != "" {
		days := e.DaysFromTag
		if days == 0 {
			days, _ = edition.DaysBetween(e.TagDate, e.Released)
		}
		verdict := "within"
		if days > 14 {
			verdict = "later than"
		}
		fmt.Fprintf(&b, "\nThe edition came out %d days after the tag, %s the 14 days of spec 05.6.\n", days, verdict)
	}
	if e.Coverage != nil {
		fmt.Fprintf(&b, "\n## The coverage\n\nC-home is %.1f%%, C-depth is %.1f%%, C-fresh is %.1f%% and C-review is %.1f%%.\n", 100*e.Coverage.Home, 100*e.Coverage.Depth, 100*e.Coverage.Fresh, 100*e.Coverage.Review)
	}
	if len(e.Volumes) > 0 {
		b.WriteString("\n## The volumes\n\n| Volume | State |\n|---|---|\n")
		var vs []string
		for v := range e.Volumes {
			vs = append(vs, v)
		}
		sort.Strings(vs)
		for _, v := range vs {
			fmt.Fprintf(&b, "| %s | %s |\n", v, e.Volumes[v])
		}
	}
	var prev *edition.Record
	for i := range rec.Editions {
		if rec.Editions[i].Edition == tag {
			break
		}
		prev = &rec.Editions[i]
	}
	if prev == nil {
		b.WriteString("\n## The drift since the last edition\n\nThis is the first edition, so there is no drift since the last one.\n")
		return b.String(), nil
	}
	chain, err := rename.Chain(rec.Drift, prev.Mathlib, e.Mathlib)
	if err != nil {
		fmt.Fprintf(&b, "\n## The drift since the last edition\n\nThe drift records do not lead from %s (%s) to %s. %s.\n", prev.Edition, short(prev.Mathlib), short(e.Mathlib), err)
		return b.String(), nil
	}
	fmt.Fprintf(&b, "\nThe drift since %s comes from %d drift records.\n", prev.Edition, len(chain))
	driftSection(&b, chain)
	return b.String(), nil
}

// Path gives the place of a report in the map.
func Path(mapDir, kind, name string) string {
	return filepath.Join(mapDir, "reports", kind, name+".md")
}
