package source

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"leaguepingo/internal/lcu"
)

func TestLiveBasicProfileAvatar(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_PROFILE_TEST") != "1" {
		t.Skip("explicit current-account profile check only")
	}
	c, err := lcu.Discover()
	if err != nil {
		t.Skip("client unavailable; avatar not verified")
	}
	defer c.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	raw, err := c.Get(ctx, "/lol-summoner/v1/current-summoner")
	if err != nil {
		t.Fatal("self identity unavailable (redacted)")
	}
	var self struct {
		PUUID string `json:"puuid"`
	}
	if json.Unmarshal(raw, &self) != nil || self.PUUID == "" {
		t.Fatal("invalid self identity (redacted)")
	}
	profileCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	p := (&TencentLCU{client: c}).BasicProfile(profileCtx, self.PUUID)
	if p.Level == nil || p.ProfileIconID == nil || p.ProfileIconData == "" {
		t.Fatalf("basic profile incomplete: level=%t iconID=%t image=%t; values redacted", p.Level != nil, p.ProfileIconID != nil, p.ProfileIconData != "")
	}
	t.Log("current-account level and verified avatar image available; no history/rank requests, identities/images not retained")
}
