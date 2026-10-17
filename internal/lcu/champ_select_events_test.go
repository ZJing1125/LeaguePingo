package lcu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func selectionFrame(champion, timer int) string {
	return fmt.Sprintf(`[8,"%s",{"uri":"/lol-champ-select/v1/session","eventType":"Update","data":{"gameId":42,"myTeam":[{"cellId":0,"puuid":"synthetic","championId":%d}],"theirTeam":[],"timer":%d}}]`, champSelectTopic, champion, timer)
}

func TestSelectionSignatureIgnoresTimerButDetectsHeroClearAndDelete(t *testing.T) {
	a := champSelectSignature([]byte(selectionFrame(23, 10)))
	if a == "" || a != champSelectSignature([]byte(selectionFrame(23, 9))) || a == champSelectSignature([]byte(selectionFrame(24, 9))) || a == champSelectSignature([]byte(selectionFrame(0, 9))) {
		t.Fatal("selection change detection failed")
	}
	intent := strings.Replace(selectionFrame(0, 10), `"championId":0`, `"championId":0,"championPickIntent":23`, 1)
	if champSelectSignature([]byte(intent)) == champSelectSignature([]byte(selectionFrame(0, 10))) {
		t.Fatal("pick intent change ignored")
	}
	for _, raw := range []string{`null`, `{}`, `[]`, `[8]`, strings.ReplaceAll(selectionFrame(23, 10), champSelectTopic, "OnJsonApiEvent"), strings.ReplaceAll(selectionFrame(23, 10), "Update", "Unknown"), strings.ReplaceAll(selectionFrame(23, 10), "/lol-champ-select/v1/session", "/other")} {
		if champSelectSignature([]byte(raw)) != "" {
			t.Fatal("unrelated event accepted")
		}
	}
	if champSelectSignature([]byte(`[8,"`+champSelectTopic+`",{"uri":"/lol-champ-select/v1/session","eventType":"Delete","data":null}]`)) != "deleted" {
		t.Fatal("delete not detected")
	}
}

func TestSelectionWatchFiltersDeduplicatesAndCloses(t *testing.T) {
	serverDone := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, subscription, err := conn.Read(ctx)
		if err != nil || string(subscription) != `[5,"`+champSelectTopic+`"]` {
			t.Error("wrong selection subscription")
			return
		}
		for _, frame := range []string{selectionFrame(23, 10), selectionFrame(23, 9), selectionFrame(24, 8), selectionFrame(0, 7)} {
			if conn.Write(ctx, websocket.MessageText, []byte(frame)) != nil {
				return
			}
		}
		_, _, _ = conn.Read(ctx)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifications := make(chan struct{}, 10)
	done := make(chan error, 1)
	go func() {
		done <- testEventClient(t, server.URL).WatchChampSelect(ctx, func() { notifications <- struct{}{} })
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-notifications:
		case <-time.After(2 * time.Second):
			t.Fatal("missing selection notification")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("selection stream did not stop")
	}
	if len(notifications) != 0 {
		t.Fatal("timer-only event was emitted")
	}
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("server not closed")
	}
}

func TestSelectionWatchRejectsRedirects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Error("followed selection redirect")
		}
		http.Redirect(w, r, "/synthetic-private", http.StatusFound)
	}))
	defer server.Close()
	err := testEventClient(t, server.URL).WatchChampSelect(context.Background(), func() { t.Error("unexpected callback") })
	if err == nil || strings.Contains(err.Error(), "synthetic") || strings.Contains(err.Error(), server.URL) {
		t.Fatal("selection redirect or error handling unsafe")
	}
}
