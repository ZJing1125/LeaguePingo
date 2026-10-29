package lcu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostGameSequenceAndCompletionRequireExplicitFields(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`null`, ""}, {`{"name":""}`, ""}, {`{"name":"missions-celebration","priority":10}`, "missions-celebration"}, {`{"name":"unknown-private-or-selection"}`, "other"},
	} {
		got, err := parsePostGameSequence([]byte(tc.raw))
		if err != nil || got != tc.want {
			t.Fatal("unsafe sequence normalization", err)
		}
	}
	for _, raw := range []string{`{}`, `{"name":null}`, `{"name":false}`, `"missions-celebration"`} {
		if _, err := parsePostGameSequence([]byte(raw)); err == nil {
			t.Fatal("unknown sequence treated as empty")
		}
	}
	for _, raw := range []string{`{"gameId":42,"fullTeamVote":false}`, `{"gameId":42,"fullTeamVote":true}`} {
		if id, err := parseHonorCompletion([]byte(raw)); err != nil || id != 42 {
			t.Fatal("explicit vote completion lost", err)
		}
	}
	for _, raw := range []string{`{}`, `null`, `{"fullTeamVote":true}`, `{"gameId":-1}`, `{"gameId":"42"}`} {
		if _, err := parseHonorCompletion([]byte(raw)); err == nil {
			t.Fatal("completion fabricated from missing/invalid game")
		}
	}
}

func TestPostGameContinueIsOneFixedBodylessEndpoint(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.Method+" "+r.URL.Path]++
		switch {
		case r.Method == "GET" && r.URL.Path == postGameSequencePath:
			fmt.Fprint(w, `{"name":"missions-celebration"}`)
		case r.Method == "GET" && r.URL.Path == honorCompletionPath:
			fmt.Fprint(w, `{"gameId":42,"fullTeamVote":false}`)
		case r.Method == "POST" && r.URL.Path == continuePostGamePath:
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 {
				t.Error("continue submitted a selection")
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Error("unexpected ceremony mutation")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := testEventClient(t, server.URL)
	if name, err := c.PostGameSequence(context.Background()); err != nil || name != "missions-celebration" {
		t.Fatal(err)
	}
	if id, err := c.HonorVoteCompletion(context.Background()); err != nil || id != 42 {
		t.Fatal(err)
	}
	if err := c.ContinuePostGameProgress(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls["POST "+continuePostGamePath] != 1 {
		t.Fatal("unreviewed sequence operations")
	}
}
