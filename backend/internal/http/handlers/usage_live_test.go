package handlers

import (
	"context"
	"testing"
)

func TestUsageLiveTracksRequestsAndErrors(t *testing.T) {
	u := &UsageLive{}
	u.Track("model", "Provider", "", true, false)
	result, err := u.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	active := result["activeRequests"].([]map[string]any)
	if len(active) != 1 || active[0]["account"] != "Free / No Auth" || active[0]["count"] != 1 {
		t.Fatalf("unexpected active: %v", active)
	}
	u.Track("model", "Provider", "", false, true)
	result, err = u.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result["activeRequests"].([]map[string]any)) != 0 || result["errorProvider"] != "provider" {
		t.Fatal("request not completed")
	}
	u.Track("model", "Provider", "", false, false)
	result, _ = u.Snapshot(context.Background())
	if len(result["activeRequests"].([]map[string]any)) != 0 {
		t.Fatal("negative counter")
	}
}
