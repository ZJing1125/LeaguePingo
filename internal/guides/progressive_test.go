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
	if state.Base == nil || len(state.Base.Augments) == 0 || state.Base.AugmentsFetchedAt == nil || state.Base.BuildsFetchedAt != nil || state.Base.AugmentsStatus.State != "ready" || state.Base.BuildsStatus.State != "loading" {
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

func TestMayhemPartsFailIndependentlyAndKeepOnlyMatchingVersion(t *testing.T) {
	q := Query{Mode: "aram-mayhem", Champion: 3, Position: "mid", Patch: "16.21"}
	for _, failed := range []string{"augments", "build"} {
		t.Run(failed, func(t *testing.T) {
			o := NewOPgg()
			o.http = &http.Client{Transport: guideTransport(func(r *http.Request) (*http.Response, error) {
				tab, records := "augments", augmentRecords
				if strings.HasSuffix(r.URL.Path, "/build") {
					tab, records = "build", buildRecords
				}
				if tab == failed {
					return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(flightFixture(q, tab, records))))}, nil
			})}
			g, err := o.FetchProgressive(context.Background(), q, nil)
			if err != nil {
				t.Fatal(err)
			}
			if failed == "augments" && (g.AugmentsStatus.State != "failed" || g.BuildsStatus.State != "ready" || g.AugmentsFetchedAt != nil) {
				t.Fatal("augment failure overwrote build success")
			}
			if failed == "build" && (g.AugmentsStatus.State != "ready" || g.BuildsStatus.State != "failed" || g.BuildsFetchedAt != nil) {
				t.Fatal("build failure overwrote augment success")
			}
			prior := &Guide{Query: q, Version: g.Version, Builds: []BuildSection{{Label: "previous"}}, Augments: []Augment{{ID: 1}}}
			merged := mergeGuideParts(prior, g)
			if failed == "build" && merged.BuildsStatus.State != "retained" || failed == "augments" && merged.AugmentsStatus.State != "retained" {
				t.Fatal("same version content lost")
			}
			prior.Version = "different"
			if mergeGuideParts(prior, g) != g {
				t.Fatal("mixed different versions")
			}
		})
	}
}
func TestGuideCacheNormalizesOldPartsWithoutPersistingResponseID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, t.TempDir(), NewOPgg())
	q := Query{Mode: "aram-mayhem", Champion: 3}
	g := &Guide{Query: q, Version: "16.21", FetchedAt: time.Now(), SourceURL: "https://op.gg/zh-cn/lol/modes/aram-mayhem/galio/build", ResponseID: 42, Augments: []Augment{{ID: 1}}}
	key := cacheKey(q)
	s.save(key, g)
	loaded := s.cached(q)
	if loaded == nil || loaded.ResponseID != 0 || loaded.AugmentsStatus.State != "ready" || loaded.BuildsStatus.State != "failed" || loaded.AugmentsStatus.FetchedAt != nil {
		t.Fatal("old cache invented timestamps or kept trace ID")
	}
}
