package engine

import (
	"context"
	"errors"
	"fmt"
	"leaguepingo/internal/core"
	"strings"
	"testing"
)

type playerDataClient struct {
	*fakeClient
	profile   core.Profile
	onProfile func()
}

func (c *playerDataClient) Lobby(context.Context) ([]core.Participant, error) {
	c.record("lobby")
	return c.roster, nil
}
func (c *playerDataClient) Profile(context.Context, string) core.Profile {
	c.record("profile")
	if c.onProfile != nil {
		c.onProfile()
	}
	return c.profile
}
func dataProfile() core.Profile {
	level, icon := 100, 123
	return core.Profile{Level: &level, ProfileIconID: &icon, ProfileIconData: "synthetic-avatar", PrivacyState: "public", RankState: "ready", Ranks: []core.Rank{{Queue: "RANKED_SOLO_5x5", Tier: "GOLD"}}}
}
func TestLobbyAutomaticallyPreparesAvatarAndRanksEvenWithWarmHistory(t *testing.T) {
	c := &playerDataClient{fakeClient: &fakeClient{phase: "Lobby", roster: fullTestRoster()[:1]}, profile: dataProfile()}
	e := New(nil, nil)
	e.client = c
	p := c.roster[0]
	core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 42}}})
	e.cachePlayerLocked(p)
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	p = e.Snapshot().Participants[0]
	if p.Profile == nil || p.Profile.RankState != "ready" || p.Profile.ProfileIconData == "" || p.History[0].GameID != 42 {
		t.Fatal("lobby did not prepare full data independently of warm history")
	}
	for i := 0; i < 3; i++ {
		e.poll(context.Background())
		waitPreparationStopped(t, e)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, call := range c.calls {
		if call == "profile" {
			n++
		}
		if call == "history" {
			t.Fatal("warm history refetched")
		}
	}
	if n != 1 {
		t.Fatal("profile repeated on polling", n)
	}
}

func TestInitialMatchPreparesFullDataOnlyOnceAndHistoryFirst(t *testing.T) {
	c := &playerDataClient{fakeClient: &fakeClient{phase: "InProgress", gameID: 42, roster: fullTestRoster()}, profile: dataProfile()}
	e := New(func() (Client, error) { return c, nil }, nil)
	e.initialMatch.parent = context.Background()
	e.poll(context.Background())
	waitInitialMatch(t, e)
	for _, p := range e.Snapshot().Participants {
		if p.Profile == nil || p.Profile.RankState != "ready" || !historyCached(p) {
			t.Fatal("initial batch missed full player data")
		}
	}
	for i := 0; i < 3; i++ {
		e.poll(context.Background())
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	profiles, histories := 0, 0
	for _, call := range c.calls {
		if call == "history" {
			histories++
			if profiles > 0 {
				t.Fatal("optional data delayed first-pass history")
			}
		}
		if call == "profile" {
			profiles++
		}
	}
	if profiles != 10 || histories != 10 {
		t.Fatal("unbounded or repeated batch", profiles, histories)
	}
}

func TestAutomaticProfileFailureIsBoundedAndLatePhaseReplyIgnored(t *testing.T) {
	for _, leave := range []bool{false, true} {
		c := &playerDataClient{fakeClient: &fakeClient{phase: "Lobby", roster: fullTestRoster()[:1]}, profile: core.Profile{RankState: "failed", RankMessage: "资料暂不可用"}}
		e := New(nil, nil)
		e.client = c
		if leave {
			c.onProfile = func() { e.mu.Lock(); defer e.mu.Unlock(); e.replaceLocked("idle", "None", 0, nil, "") }
		}
		e.poll(context.Background())
		waitPreparationStopped(t, e)
		if leave {
			if len(e.Snapshot().Participants) != 0 || e.cache[c.roster[0].Key].Profile != nil {
				t.Fatal("late profile crossed phase")
			}
		} else {
			for i := 0; i < 3; i++ {
				e.poll(context.Background())
				waitPreparationStopped(t, e)
			}
			c.mu.Lock()
			count := 0
			for _, call := range c.calls {
				if call == "profile" {
					count++
				}
			}
			c.mu.Unlock()
			if count != 1 {
				t.Fatal("failed profile polled repeatedly", count)
			}
		}
	}
}

func TestUnifiedRefreshKeepsIndependentSuccessfulData(t *testing.T) {
	for _, historyFails := range []bool{false, true} {
		c := &playerDataClient{fakeClient: &fakeClient{phase: "Lobby"}, profile: dataProfile()}
		e := New(nil, nil)
		e.client = c
		p := fullTestRoster()[0]
		core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 42}}})
		previous := mergeProfile(dataProfile(), nil)
		p.Profile = &previous
		p.PrivacyState = previous.PrivacyState
		e.cachePlayerLocked(p)
		e.snapshot.Participants = []core.Participant{p}
		c.historyFn = func(context.Context, string) (core.HistoryResult, error) {
			if historyFails {
				return core.HistoryResult{}, errors.New("synthetic failure")
			}
			return core.HistoryResult{Matches: []core.Match{{GameID: 77}}}, nil
		}
		if !historyFails {
			c.profile = core.Profile{RankState: "failed", RankMessage: "段位查询超时，可重试"}
		}
		got, err := e.RefreshData(context.Background(), p.Key, 0)
		if err != nil || got.Profile == nil || got.Profile.RankState != "ready" {
			t.Fatal("unified refresh discarded usable data", err)
		}
		if historyFails {
			if got.History[0].GameID != 42 || !strings.Contains(got.Error, "战绩更新失败") || got.Profile.Fields["rank"].State != "ready" {
				t.Fatal("history failure prevented successful profile update")
			}
		} else if got.History[0].GameID != 77 || got.Profile.Fields["rank"].State != "retained" || got.Profile.Fields["rank"].FetchedAt != previous.Fields["rank"].FetchedAt {
			t.Fatal("profile failure lost new history or old rank age")
		}
	}
}

func TestPreparationCapsProfilesAndSkipsUnknownIdentities(t *testing.T) {
	c := &playerDataClient{fakeClient: &fakeClient{phase: "Lobby"}, profile: dataProfile()}
	e := New(nil, nil)
	e.client = c
	for i := 0; i < 13; i++ {
		p := fullTestRoster()[0]
		p.Key = fmt.Sprintf("bounded-%d", i)
		p.PUUID = p.Key
		if i == 12 {
			p.PUUID = ""
			p.IdentityState = core.IdentityUnavailable
		}
		core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 42}}})
		c.roster = append(c.roster, p)
		if i < 12 {
			e.cachePlayerLocked(p)
		}
	}
	e.poll(context.Background())
	waitPreparationStopped(t, e)
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, call := range c.calls {
		if call == "profile" {
			count++
		}
	}
	if count != 10 {
		t.Fatal("automatic profile limit was not respected", count)
	}
	if len(e.Snapshot().Participants) != 13 {
		t.Fatal("unknown or extra members were removed")
	}
}

func TestHistoryReplyRetainsProfileCompletedWhileWaiting(t *testing.T) {
	c := &playerDataClient{fakeClient: &fakeClient{phase: "Lobby"}, profile: dataProfile()}
	e := New(nil, nil)
	e.client = c
	p := fullTestRoster()[0]
	core.ApplyHistory(&p, core.HistoryResult{Matches: []core.Match{{GameID: 42}}})
	e.cachePlayerLocked(p)
	e.snapshot.Participants = []core.Participant{p}
	completed := mergeProfile(dataProfile(), nil)
	c.historyFn = func(context.Context, string) (core.HistoryResult, error) {
		e.mu.Lock()
		current := e.cache[p.PUUID]
		current.Profile = &completed
		current.PrivacyState = completed.PrivacyState
		e.cache[p.PUUID] = current
		e.mu.Unlock()
		return core.HistoryResult{Matches: []core.Match{{GameID: 77}}}, nil
	}
	got, err := e.RefreshHistory(context.Background(), p.Key, 0)
	if err != nil || got.Profile == nil || !got.Profile.UpdatedAt.Equal(*completed.UpdatedAt) || got.History[0].GameID != 77 || e.Snapshot().Participants[0].Profile == nil || got.PrivacyState != "public" {
		t.Fatal("history reply overwrote independently completed profile", err)
	}
}
