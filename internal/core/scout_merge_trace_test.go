package core

import (
	"testing"
	"time"
)

func TestRetainedScoutReportKeepsCurrentResponseAssociation(t *testing.T) {
	now := time.Now()
	old := &ScoutReport{ResponseID: 42, Read: 100, State: "partial", Complete30: true, FetchedAt: now}
	next := ScoutReport{ResponseID: 43, Read: 20, State: "loading", FetchedAt: now}
	merged := MergeScoutReports(old, next)
	if !merged.Retained || merged.ResponseID != 43 || old.ResponseID != 42 {
		t.Fatal("merged result acknowledged an earlier response")
	}
	next.ResponseID = 0
	if MergeScoutReports(old, next).ResponseID != 0 {
		t.Fatal("failed/untraced update revived an earlier measurement")
	}
}
