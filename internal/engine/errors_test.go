package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

func TestConnectionSnapshotDoesNotExposeUnexpectedSourceText(t *testing.T) {
	raw := errors.New("Get https://private-host/player/private-player#DEMO?token=secret: HTTP 503")
	f := &fakeClient{phase: "InProgress", phaseErr: raw}
	e := New(func() (Client, error) { return nil, raw }, nil)
	e.poll(context.Background())
	if e.Snapshot().Message != apperrors.Message(apperrors.ClientUnavailable) {
		t.Fatal("discovery bypassed fixed message")
	}
	e.client = f
	e.snapshot = core.Snapshot{Connected: true, Frozen: true, Stage: "in-game", Phase: "InProgress", Participants: []core.Participant{{Key: "retained"}}}
	e.poll(context.Background())
	s := e.Snapshot()
	if s.Connected || len(s.Participants) != 1 || !s.Frozen || !strings.Contains(s.Message, apperrors.Message(apperrors.ClientUnavailable)) {
		t.Fatal("failure lost retained snapshot")
	}
	for _, secret := range []string{"private-host", "private-player", "token", "secret", "HTTP", "503", "https://"} {
		if strings.Contains(s.Message, secret) {
			t.Fatal("source text escaped into snapshot")
		}
	}
}

func TestFirstHistoryFailureShowsClassifiedHintWithVerifiedIdentity(t *testing.T) {
	f := &fakeClient{historyFn: func(context.Context, string) (core.HistoryResult, error) {
		return core.HistoryResult{}, apperrors.HTTP(apperrors.DataService, 429)
	}}
	e := New(nil, nil)
	e.client = f
	e.snapshot.Connected = true
	p, err := e.Lookup(context.Background(), "Synthetic#DEMO")
	if err != nil || p.IdentityState != core.IdentityKnown || p.HistoryState != core.HistoryFailed ||
		!strings.Contains(p.Error, apperrors.Message(apperrors.RateLimited)) || strings.Contains(p.Error, "429") {
		t.Fatalf("first failure lost verified identity or rate-limit hint: %v / %q", err, p.Error)
	}
}
