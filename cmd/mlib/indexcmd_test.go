package main

import "testing"

func TestPickAssets(t *testing.T) {
	assets := []asset{
		{Name: "index-2026-11-26-e54979a532.jsonl.gz", URL: "i1"},
		{Name: "modules-2026-11-26-e54979a532.jsonl.gz", URL: "m1"},
		{Name: "index-2026-11-27-4472534b60.jsonl.gz", URL: "i2"},
		{Name: "modules-2026-11-27-4472534b60.jsonl.gz", URL: "m2"},
		{Name: "proofs-2026-11-27-4472534b60.jsonl.gz", URL: "p2"},
	}
	i, m, p, err := pickAssets(assets, "latest")
	if err != nil || i.URL != "i2" || m.URL != "m2" || p.URL != "p2" {
		t.Errorf("latest: %v %v %v %v", i, m, p, err)
	}
	// The older index has no proofs file.
	i, m, p, err = pickAssets(assets, "e54979a532247ae4eb9d5578a8fd40a1bc332101")
	if err != nil || i.URL != "i1" || m.URL != "m1" || p.Name != "" {
		t.Errorf("e54979a532: %v %v %v %v", i, m, p, err)
	}
	if _, _, _, err := pickAssets(assets, "0000000000"); err == nil {
		t.Error("no error for a commit with no index")
	}
}
