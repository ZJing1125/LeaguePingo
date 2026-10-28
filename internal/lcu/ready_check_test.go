package lcu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestReadyCheckUsesOnlyFixedLocalEndpoints(t *testing.T) {
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == readyCheckPath:
			fmt.Fprint(w, `{"state":"InProgress","playerResponse":"None"}`)
		case r.Method == "POST" && r.URL.Path == readyCheckPath+"/accept":
			posts++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	c := NewClient(Credentials{Port: port, Password: "synthetic"})
	c.HTTP = server.Client()
	state, err := c.ReadyCheck(context.Background())
	if err != nil || !state.Pending() {
		t.Fatalf("pending state not read: %v", err)
	}
	if err := c.AcceptReadyCheck(context.Background()); err != nil || posts != 1 {
		t.Fatalf("accept endpoint failed: %v", err)
	}
}
