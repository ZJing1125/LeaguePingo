package guides

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMayhemAugmentsUsableWhileBuildIsBlocked(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buildStarted, release := make(chan struct{}), make(chan struct{})
	o := NewOPgg()
	o.http = &http.Client{Transport: guideTransport(func(r *http.Request) (*http.Response, error) {
		tab, records := "augments", augmentRecords
		if strings.HasSuffix(r.URL.Path, "/build") {
			close(buildStarted)
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			tab, records = "build", buildRecords
		}
		q := Query{Mode: "aram-mayhem", Champion: 3, Position: "mid", Patch: "16.21"}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(flightFixture(q, tab, records))))}, nil
	})}
	s := New(ctx, t.TempDir(), o)
	snap := guideSnapshot()
	snap.QueueID = 2400
	snap.MapID = 12
	snap.GameMode = "ARAM"
	snap.Participants = snap.Participants[:1]
	s.Observe(snap)
	select {
	case <-buildStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("build not started")
	}
	state := s.Snapshot()
	if state.Base == nil || len(state.Base.Augments) == 0 || state.Base.AugmentsFetchedAt == nil || state.Base.BuildsFetchedAt != nil {
		t.Fatal("augments withheld behind build")
	}
	ids := []int{state.Base.Augments[0].ID, state.Base.Augments[1].ID, state.Base.Augments[2].ID}
	if _, err := s.Candidates(state.Scope, 3, ids); err != nil {
		t.Fatal("available augment candidates blocked", err)
	}
	close(release)
	waitGuide(t, s, "ready")
	state = s.Snapshot()
	if len(state.Candidates) != 3 || len(state.Base.Builds) == 0 || state.Base.BuildsFetchedAt == nil {
		t.Fatal("build arrival lost augment selection")
	}
}
