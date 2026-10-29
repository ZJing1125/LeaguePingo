package lcu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHonorBallotOnlyReturnsGameAndExistingSelections(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		id       int64
		selected bool
	}{
		{`null`, 0, false}, {`{"gameId":0}`, 0, false},
		{`{"gameId":42,"honoredPlayers":[],"eligibleAllies":[{"puuid":"secret"}]}`, 42, false},
		{`{"gameId":42,"honoredPlayers":[{"recipientPuuid":"secret"}]}`, 42, true},
	} {
		id, selected, err := parseHonorBallot([]byte(tc.raw))
		if err != nil || id != tc.id || selected != tc.selected {
			t.Fatal("wrong minimal ballot", id, selected, err)
		}
	}
	for _, raw := range []string{``, `{}`, `{"gameId":-1}`, `{"gameId":42}`, `{"gameId":42,"honoredPlayers":null}`, `{"gameId":42,"honoredPlayers":"invalid"}`, `"failed_to_contact_honor_server"`, strings.Repeat("x", 64<<10)} {
		if _, _, err := parseHonorBallot([]byte(raw)); err == nil {
			t.Fatal("unknown/malformed ballot treated as unselected")
		}
	}
}

func TestHonorUsesFixedLocalPathsAndNeverSubmitsAPlayer(t *testing.T) {
	reads, submits := 0, 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == honorBallotPath:
			reads++
			fmt.Fprint(w, `{"gameId":42,"honoredPlayers":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == submitHonorBallotPath:
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 || r.Header.Get("Content-Type") != "" {
				t.Error("honor submission included a body/recipient")
			}
			submits++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Error("unexpected honor endpoint")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := testEventClient(t, server.URL)
	if id, selected, err := client.HonorBallot(context.Background()); err != nil || id != 42 || selected {
		t.Fatal("ballot read failed", err)
	}
	if err := client.CompleteHonorBallot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || submits != 1 {
		t.Fatal("unexpected honor reads/writes", reads, submits)
	}
}

func TestHonor204DoesNotRelaxOtherRequests(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	client := testEventClient(t, server.URL)
	if _, _, err := client.HonorBallot(context.Background()); err == nil {
		t.Fatal("empty GET accepted")
	}
	if _, err := client.request(context.Background(), http.MethodPost, "/other", nil); err == nil {
		t.Fatal("unrelated mutation accepts 204")
	}
}
