package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/mathlib-agent/blueprint"
	"github.com/tamnd/mathlib-agent/coverage"
	"github.com/tamnd/mathlib-agent/depth"
	"github.com/tamnd/mathlib-agent/drift"
	"github.com/tamnd/mathlib-agent/index"
)

// The release of tamnd/mathlib that holds the index assets (spec 08.4, Q-06).
const (
	indexRepo    = "tamnd/mathlib"
	indexTag     = "index"
	mathlibRepo  = "leanprover-community/mathlib4"
	depthTopType = 0.02
	depthMinUses = 5
)

var httpClient = &http.Client{Timeout: 10 * time.Minute}

// allowedHosts are the hosts that mlib downloads from.
var allowedHosts = map[string]bool{
	"api.github.com":                       true,
	"github.com":                           true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
	"raw.githubusercontent.com":            true,
}

// get fetches a URL. It sends GITHUB_TOKEN when it is set, for the rate limit of the API. It refuses a host that is not in allowedHosts.
func get(ctx context.Context, url string, accept string) (io.ReadCloser, error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" || !allowedHosts[u.Hostname()] {
		return nil, fmt.Errorf("mlib does not download from %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // the host is checked above
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.Header.Set("User-Agent", "mlib")
	if t := os.Getenv("GITHUB_TOKEN"); t != "" && strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := httpClient.Do(req) //nolint:gosec // get accepts only the HTTPS hosts in allowedHosts
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

func download(ctx context.Context, url, path string) error {
	body, err := get(ctx, url, "")
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // the path comes from the -out flag of the user
		return err
	}
	f, err := os.Create(path) //nolint:gosec // the path comes from the -out flag of the user
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// pickAssets returns the index and modules assets of the commit, or of the newest date when commit is "latest". The names are index-<date>-<commit>.jsonl.gz and modules-<date>-<commit>.jsonl.gz. The proofs asset proofs-<date>-<commit>.jsonl.gz is optional: an index from before 2026-11-27 has none, and then proofs has no name.
func pickAssets(assets []asset, commit string) (idx, mods, proofs asset, err error) {
	var names []string
	byName := map[string]asset{}
	for _, a := range assets {
		byName[a.Name] = a
		if strings.HasPrefix(a.Name, "index-") && strings.HasSuffix(a.Name, ".jsonl.gz") {
			key := strings.TrimSuffix(strings.TrimPrefix(a.Name, "index-"), ".jsonl.gz")
			parts := strings.SplitN(key, "-", 4)
			if len(parts) != 4 {
				continue
			}
			if commit == "latest" || strings.HasPrefix(commit, parts[3]) || strings.HasPrefix(parts[3], commit) {
				names = append(names, key)
			}
		}
	}
	if len(names) == 0 {
		return idx, mods, proofs, fmt.Errorf("the release %s of %s has no index for %s", indexTag, indexRepo, commit)
	}
	sort.Strings(names)
	key := names[len(names)-1]
	idx = byName["index-"+key+".jsonl.gz"]
	mods, ok := byName["modules-"+key+".jsonl.gz"]
	if !ok {
		return idx, mods, proofs, fmt.Errorf("the release has index-%s.jsonl.gz but no modules-%s.jsonl.gz", key, key)
	}
	return idx, mods, byName["proofs-"+key+".jsonl.gz"], nil
}

func runIndex(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: give a subcommand: fetch or show", errUsage)
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("index "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	switch sub {
	case "fetch":
		commit := fs.String("commit", "latest", "the Mathlib commit, a prefix of it, or latest")
		out := fs.String("out", "index", "the directory to write index.jsonl.gz, modules.jsonl.gz and docs/ to")
		if err := fs.Parse(args); err != nil {
			return err
		}
		ctx := context.Background()
		body, err := get(ctx, "https://api.github.com/repos/"+indexRepo+"/releases/tags/"+indexTag, "application/vnd.github+json")
		if err != nil {
			return err
		}
		var rel struct {
			Assets []asset `json:"assets"`
		}
		err = json.NewDecoder(body).Decode(&rel)
		_ = body.Close()
		if err != nil {
			return err
		}
		ia, ma, pa, err := pickAssets(rel.Assets, *commit)
		if err != nil {
			return err
		}
		files := []struct{ asset, file string }{{ia.URL, "index.jsonl.gz"}, {ma.URL, "modules.jsonl.gz"}}
		// Remove a proofs file of an earlier fetch, so that the index and the proofs are always of one commit.
		if err := os.Remove(filepath.Join(*out, "proofs.jsonl.gz")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if pa.Name != "" {
			files = append(files, struct{ asset, file string }{pa.URL, "proofs.jsonl.gz"})
		}
		for _, a := range files {
			if err := download(ctx, a.asset, filepath.Join(*out, a.file)); err != nil {
				return err
			}
		}
		ix, err := index.Load(*out)
		if err != nil {
			return err
		}
		// The curated lists at the same commit, for the depth rules.
		for _, f := range depth.CuratedFiles {
			url := "https://raw.githubusercontent.com/" + mathlibRepo + "/" + ix.Header.Mathlib + "/" + f
			if err := download(ctx, url, filepath.Join(*out, f)); err != nil {
				return err
			}
		}
		fmt.Fprintf(stdout, "%s: %d declarations and %d modules of Mathlib %s, toolchain %s, from %s\n", *out, len(ix.Decls), len(ix.Modules), short(ix.Header.Mathlib), ix.Header.Toolchain, ia.Name)
		if !ix.Proofs {
			fmt.Fprintf(stdout, "%s has no proofs file, so the used-by counts of rule 2 of spec 04.4 have only the uses of the types\n", ia.Name)
		}
		return nil
	case "show":
		dir := fs.String("index", "index", "the directory of the index")
		if err := fs.Parse(args); err != nil {
			return err
		}
		ix, err := index.Load(*dir)
		if err != nil {
			return err
		}
		h := ix.Header
		fmt.Fprintf(stdout, "Mathlib %s, toolchain %s, written %s\n", h.Mathlib, h.Toolchain, h.Date)
		fmt.Fprintf(stdout, "%d declarations in %d modules\n", len(ix.Decls), len(ix.Modules))
		kinds := map[string]int{}
		aliases, errs := 0, 0
		for i := range ix.Decls {
			kinds[ix.Decls[i].Kind]++
			if ix.Decls[i].Deprecated != nil {
				aliases++
			}
			if ix.Decls[i].Error != "" {
				errs++
			}
		}
		writeCounts(stdout, kinds)
		fmt.Fprintf(stdout, "  deprecated aliases\t%d\n  records with an error\t%d\n", aliases, errs)
		return nil
	}
	return fmt.Errorf("%w: unknown subcommand %q", errUsage, sub)
}

func short(c string) string {
	if len(c) > 10 {
		return c[:10]
	}
	return c
}

func writeCounts(w io.Writer, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  %s\t%d\n", k, m[k])
	}
}

// loadDepths loads an index and computes the depths. The curated lists are in the docs/ folder of the index directory, as mlib index fetch writes them, or of a Mathlib checkout.
func loadDepths(dir, lists string) (*index.Index, map[string]depth.Result, depth.Lists, error) {
	ix, err := index.Load(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	if lists == "" {
		lists = dir
	}
	l, err := depth.LoadLists(lists)
	if err != nil {
		return nil, nil, nil, err
	}
	return ix, depth.Compute(ix, l, depthTopType, depthMinUses), l, nil
}

func runDepth(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("depth", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("index", "index", "the directory of the index")
	lists := fs.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ix, depths, _, err := loadDepths(*dir, *lists)
	if err != nil {
		return err
	}
	if fs.NArg() == 0 {
		counts := map[string]int{}
		for i := range ix.Decls {
			if ix.Decls[i].Deprecated == nil {
				counts[depths[ix.Decls[i].Name].Depth]++
			}
		}
		total := 0
		for _, n := range counts {
			total += n
		}
		for _, d := range depth.Depths {
			fmt.Fprintf(stdout, "%s\t%d\t%.1f%%\n", d, counts[d], 100*float64(counts[d])/float64(max(total, 1)))
		}
		return nil
	}
	for _, n := range fs.Args() {
		r, ok := depths[n]
		if !ok {
			fmt.Fprintf(stdout, "%s\tnot in the index\n", n)
			continue
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", n, r.Depth, r.Reason)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // the path comes from the -out flag of the user
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600) //nolint:gosec // the path comes from the -out flag of the user
}

func runCoverage(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("coverage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("index", "index", "the directory of the index")
	lists := fs.String("lists", "", "a Mathlib checkout with the curated lists in docs/; the default is the index directory")
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	write := fs.Bool("write", false, "write records/coverage.json and records/modules.json to the map")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "the date of the record")
	if err := fs.Parse(args); err != nil {
		return err
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return err
	}
	ix, depths, _, err := loadDepths(*dir, *lists)
	if err != nil {
		return err
	}
	rec, rows := coverage.Compute(ix, vm, depths, *date)
	writeCoverage(stdout, rec)
	if *write {
		if err := writeJSON(filepath.Join(*mapDir, "records", "coverage.json"), rec); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(*mapDir, "records", "modules.json"), rows); err != nil {
			return err
		}
	}
	if rec.Orphans > 0 {
		return fmt.Errorf("%d declarations have no home volume: run mlib volumes orphans", rec.Orphans)
	}
	return nil
}

func writeCoverage(w io.Writer, rec coverage.Record) {
	fmt.Fprintf(w, "Mathlib %s: %d declarations, %d deprecated aliases, %d with no home volume\n", short(rec.Mathlib), rec.Total.Decls, rec.Aliases, rec.Orphans)
	fmt.Fprintf(w, "%-4s %6s %7s %6s %6s %7s %7s %6s %7s\n", "vol", "mods", "decls", "E3", "E2", "E1", "E0", "C-home", "C-depth")
	for _, v := range append(rec.Volumes, rec.Total) {
		fmt.Fprintf(w, "%-4s %6d %7d %6d %6d %7d %7d %5.0f%% %6.0f%%\n", v.Volume, v.Modules, v.Decls, v.Depth[depth.E3], v.Depth[depth.E2], v.Depth[depth.E1], v.Depth[depth.E0], 100*v.CHome, 100*v.CDepth)
	}
}

func runSync(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	oldDir := fs.String("old", "", "the directory of the index of the last sync; empty for the first sync")
	newDir := fs.String("new", "index", "the directory of the new index")
	mapDir := fs.String("map", ".", "the checkout of tamnd/mathlib")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "the date of the drift record")
	if err := fs.Parse(args); err != nil {
		return err
	}
	vm, err := blueprint.LoadVolumes(filepath.Join(*mapDir, "blueprint", "volumes.yaml"))
	if err != nil {
		return err
	}
	nix, depths, nl, err := loadDepths(*newDir, "")
	if err != nil {
		return err
	}
	rec := drift.Record{From: "", To: nix.Header.Mathlib, Counts: map[string]int{}, Items: []drift.Item{}}
	if *oldDir != "" {
		oix, _, ol, err := loadDepths(*oldDir, "")
		if err != nil {
			return err
		}
		rec = drift.Compare(oix, nix, drift.Options{Depths: depths, Lists: [2]depth.Lists{ol, nl}, Added: true})
	} else {
		rec.Toolchain = [2]string{"", nix.Header.Toolchain}
		rec.Counts[drift.Added] = len(nix.Decls)
	}
	rec.Date = *date
	if rec.Items == nil {
		rec.Items = []drift.Item{}
	}
	if err := writeJSON(filepath.Join(*mapDir, "records", "drift", *date+".json"), rec); err != nil {
		return err
	}
	cov, rows := coverage.Compute(nix, vm, depths, *date)
	if err := writeJSON(filepath.Join(*mapDir, "records", "coverage.json"), cov); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(*mapDir, "records", "modules.json"), rows); err != nil {
		return err
	}
	from := "the first sync"
	if rec.From != "" {
		from = "from " + short(rec.From)
	}
	fmt.Fprintf(stdout, "sync to Mathlib %s, %s, %d drift items\n", short(rec.To), from, len(rec.Items))
	writeCounts(stdout, rec.Counts)
	if cov.Orphans > 0 {
		return fmt.Errorf("%d declarations have no home volume: run mlib volumes orphans", cov.Orphans)
	}
	return nil
}
