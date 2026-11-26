package blueprint

import "testing"

func TestCheckMilestones(t *testing.T) {
	good := []Milestone{
		{ID: "M0", Title: "M0. Start", Target: "2026-12-20", Release: "v0.1.0"},
		{ID: "M1", Title: "M1. Next", Target: "2027-02-28", Release: "v0.2.0"},
		{ID: "M2", Title: "M2. Later", Target: "open", Release: "v2.x"},
	}
	if p := CheckMilestones(good); len(p) != 0 {
		t.Errorf("problems for good milestones: %v", p)
	}
	bad := []Milestone{
		{ID: "M1", Title: "Start", Target: "soon", Release: "1.0"},
		{ID: "M1", Title: "M1. Next", Target: "2020-01-01", Release: "v0.2.0"},
	}
	if p := CheckMilestones(bad); len(p) != 4 {
		t.Errorf("problems = %v; want 4", p)
	}
}
