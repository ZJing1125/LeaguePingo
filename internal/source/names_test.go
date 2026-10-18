package source

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"leaguepingo/internal/lcu"
)

func TestNameReadsVerifiedDisplayNameAndRejectsDifferentPlayer(t *testing.T) {
	for _, test := range []struct{ name, body, want string }{
		{"display name", `{"puuid":"self","displayName":"Visible","tagLine":"TEST"}`, "Visible#TEST"},
		{"modern name", `{"puuid":"self","gameName":"Current","tagLine":"TEST","displayName":"Old"}`, "Current#TEST"},
		{"different player", `{"puuid":"other","gameName":"Other","tagLine":"TEST"}`, ""},
		{"missing identity", `{"gameName":"Other","tagLine":"TEST"}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := lcu.NewClient(lcu.Credentials{Port: 12345, Password: "synthetic"})
			c.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			name, err := (&TencentLCU{client: c}).Name(context.Background(), "self")
			if name != test.want || (test.want == "" && err == nil) || (test.want != "" && err != nil) {
				t.Fatal("name response was not identity checked", err)
			}
		})
	}
}
