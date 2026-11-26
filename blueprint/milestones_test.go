package blueprint

import "testing"

func TestCheckMilestones(t *testing.T) {
	good := []Milestone{
		{ID: "M0", Title: "M0. Start", Target: "2026-12-20", Release: "v0.1.0", Closed: "2026-11-26"},
		{ID: "M1", Title: "M1. Next", Target: "2027-02-28", Release: "v0.2.0", Volumes: []string{"V00", "V01"}},
		{ID: "M2", Title: "M2. Later", Target: "open", Release: "v2.x"},
	}
	if p := CheckMilestones(good); len(p) != 0 {
		t.Errorf("problems for good milestones: %v", p)
	}
	bad := []Milestone{
		{ID: "M1", Title: "Start", Target: "soon", Release: "1.0"},
		{ID: "M1", Title: "M1. Next", Target: "2020-01-01", Release: "v0.2.0", Closed: "soon", Volumes: []string{"V1"}},
	}
	// The second milestone has a bad closed date, is closed after an open milestone and has a bad volume id.
	if p := CheckMilestones(bad); len(p) != 7 {
		t.Errorf("problems = %v; want 7", p)
	}
}

func TestWeight(t *testing.T) {
	ms := []Milestone{
		{ID: "M0", Closed: "2026-11-26"},
		{ID: "M1", Volumes: []string{"V00", "V01"}},
		{ID: "M2", Volumes: []string{"V02"}},
	}
	if c := Current(ms); c == nil || c.ID != "M1" {
		t.Fatalf("Current = %v; want M1", c)
	}
	for vol, want := range map[string]float64{"V01": 1, "V02": 0.5, "V30": 0.2} {
		if got := Weight(ms, "M1", vol); got != want {
			t.Errorf("Weight(%s) = %v; want %v", vol, got, want)
		}
	}
	if Find(ms, "M9") != nil || Find(ms, "M2") == nil {
		t.Error("Find gives the wrong milestone")
	}
}
