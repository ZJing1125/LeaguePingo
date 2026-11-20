package diagnostics

import (
	"testing"
	"time"
)

func TestSelectionRatingsTrackFirstAndExitCoverage(t *testing.T) {
	var r Readiness
	start := time.Now()
	r.Phase("ChampSelect", start)
	r.First(start.Add(time.Millisecond), start)
	r.ObserveRatings(start.Add(3*time.Second), 5, 4, 3, 0, 1)
	r.ObserveRatings(start.Add(5*time.Second), 5, 5, 4, 1, 0)
	r.Phase("GameStart", start.Add(6*time.Second))
	m := r.Ratings()
	if m.FirstRated != 1 || m.Exited != 1 || *m.LastFirstRatedMS != 3000 || m.LastVisible != 5 || m.LastKnown != 5 || m.LastRated != 4 || m.LastInsufficient != 1 || m.LastReading != 0 {
		t.Fatalf("incorrect rating coverage %+v", m)
	}
	r.Phase("ChampSelect", start.Add(time.Minute))
	if r.Ratings().LastFirstRatedMS != nil || r.Ratings().LastRated != 0 {
		t.Fatal("previous match reused as current coverage")
	}
}
