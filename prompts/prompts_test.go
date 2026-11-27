package prompts

import (
	"strings"
	"testing"
)

func TestEveryRoleHasAPrompt(t *testing.T) {
	for _, r := range []string{"planner", "informalizer", "family", "writer", "exampler", "exerciser", "checker", "refresher", "porter"} {
		p, err := Get(r)
		if err != nil {
			t.Errorf("%s: %v", r, err)
			continue
		}
		if p.Role != r || !strings.HasPrefix(p.Version, r+"-") || p.Body == "" {
			t.Errorf("%s: %+v", r, p)
		}
	}
	if _, err := Get("nobody"); err == nil {
		t.Error("Get gives a prompt for an unknown role")
	}
	for _, bad := range []string{"no front matter", "---\nrole: x\n", "---\nrole: x\n---\nbody"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) has no error", bad)
		}
	}
}
