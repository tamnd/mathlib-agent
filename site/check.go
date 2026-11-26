package site

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// The limits of a Worker with static assets on the free plan.
const (
	MaxFiles    = 20000
	MaxFileSize = 25 << 20
)

// Required are the pages that every build must have (spec 13).
var Required = []string{"index.html", "404.html", "decl/index.html", "module/index.html", "area/index.html", "coverage/index.html", "drift/index.html", "sitemap.xml", "robots.txt", "llms.txt"}

// Problem is one fault that Check found.
type Problem struct {
	File string
	Text string
}

func (p Problem) String() string { return p.File + ": " + p.Text }

// CheckReport is the result of Check.
type CheckReport struct {
	Files    int
	Pages    int
	Links    int
	Problems []Problem
}

var (
	linkRe  = regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	titleRe = regexp.MustCompile(`<title>([^<]+)</title>`)
)

// Check reads a built site in dir. It checks the required pages, the file count and size limits, the title of each page, the lookup shards, and each internal link. A link to decl/<name>/ is resolved by the 404 page, so the check does not follow it.
func Check(dir, base string) (CheckReport, error) {
	if base == "" {
		base = "/"
	}
	var r CheckReport
	root, err := os.OpenRoot(dir)
	if err != nil {
		return r, err
	}
	defer func() { _ = root.Close() }()
	fsys := root.FS()
	exists := func(rel string) bool {
		_, err := fs.Stat(fsys, rel)
		return err == nil
	}
	for _, p := range Required {
		if !exists(p) {
			r.Problems = append(r.Problems, Problem{p, "the page is missing"})
		}
	}
	for i := 0; i < Shards; i++ {
		p := fmt.Sprintf("decl/shard/%02x.json", i)
		if !exists(p) {
			r.Problems = append(r.Problems, Problem{p, "the shard is missing"})
		}
	}
	err = fs.WalkDir(fsys, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		r.Files++
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxFileSize {
			r.Problems = append(r.Problems, Problem{rel, fmt.Sprintf("the file has %d bytes, more than the limit of %d", info.Size(), MaxFileSize)})
		}
		if !strings.HasSuffix(rel, ".html") {
			return nil
		}
		r.Pages++
		data, err := fs.ReadFile(fsys, rel)
		if err != nil {
			return err
		}
		if m := titleRe.FindSubmatch(data); m == nil || len(bytes.TrimSpace(m[1])) == 0 {
			r.Problems = append(r.Problems, Problem{rel, "the page has no title"})
		}
		for _, m := range linkRe.FindAllSubmatch(data, -1) {
			link := string(m[1])
			if !strings.HasPrefix(link, base) || strings.HasPrefix(link, "//") {
				continue
			}
			r.Links++
			u, err := url.Parse(link)
			if err != nil {
				r.Problems = append(r.Problems, Problem{rel, "a link does not parse: " + link})
				continue
			}
			target := strings.TrimPrefix(u.Path, base)
			if strings.HasPrefix(target, "decl/") && target != "decl/" {
				continue
			}
			if target == "" || strings.HasSuffix(target, "/") {
				target += "index.html"
			}
			if !fs.ValidPath(target) || !exists(target) {
				r.Problems = append(r.Problems, Problem{rel, "a link has no target: " + link})
			}
		}
		return nil
	})
	if err != nil {
		return r, err
	}
	if r.Files > MaxFiles {
		r.Problems = append(r.Problems, Problem{".", fmt.Sprintf("the site has %d files, more than the limit of %d", r.Files, MaxFiles)})
	}
	return r, nil
}
