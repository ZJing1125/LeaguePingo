package updatecheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSemanticVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		candidate, current string
		newer              bool
	}{
		{"0.2.1", "0.2.0", true},
		{"0.2.0", "0.2.0-dev", true},
		{"0.2.0-beta.2", "0.2.0-beta.1", true},
		{"0.2.0-beta.1", "0.2.0", false},
		{"0.1.9", "0.2.0-dev", false},
		{"0.2.0-beta.01", "0.2.0-beta.1", false},
		{"invalid", "0.1.0", false},
	} {
		if got := newer(tc.candidate, tc.current); got != tc.newer {
			t.Fatalf("newer(%q, %q)=%v", tc.candidate, tc.current, got)
		}
	}
}

func TestCheckerAcceptsOnlyBoundedStaticManifest(t *testing.T) {
	body := `{"schemaVersion":1,"version":"0.3.0","url":"https://github.com/ZJing1125/LeaguePingo/releases/tag/v0.3.0","publishedAt":"2026-11-01T00:00:00Z"}`
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/main/update.json" || req.Header.Get("Accept") != "application/json" {
			t.Fatal("unexpected update request")
		}
		return response(http.StatusOK, body), nil
	})}
	checker := New("0.2.0", "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/main/update.json", client)
	checker.now = func() time.Time { return time.Unix(42, 0) }
	state := checker.Check(context.Background())
	if state.Status != "available" || state.LatestVersion != "0.3.0" || state.CheckedAt.Unix() != 42 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if checker.Snapshot() != state {
		t.Fatal("snapshot mismatch")
	}
}

func TestCheckerFailsClosedWithoutRetainingNetworkError(t *testing.T) {
	for _, body := range []string{
		`{"schemaVersion":1,"version":"0.3.0","url":"http://example.com/update.exe"}`,
		`{"schemaVersion":1,"version":"0.3.0","url":"https://evil.example/update.exe"}`,
		`{"schemaVersion":2,"version":"0.3.0","url":"https://github.com/ZJing1125/LeaguePingo/releases/latest"}`,
		`{"schemaVersion":1,"version":"bad","url":"https://github.com/ZJing1125/LeaguePingo/releases/latest"}`,
		`{"schemaVersion":1,"version":"0.3.0","url":"https://github.com:444/ZJing1125/LeaguePingo/releases/latest"}`,
		`{"schemaVersion":1,"version":"0.3.0","url":"https://github.com/ZJing1125/LeaguePingo/releases/latest?download=1"}`,
	} {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(200, body), nil })}
		state := New("0.2.0", "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/main/update.json", client).Check(context.Background())
		if state.Status != "unavailable" || state.URL != "" || state.LatestVersion != "" {
			t.Fatalf("unsafe manifest leaked into state: %+v", state)
		}
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("SECRET network path")
	})}
	state := New("0.2.0", "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/main/update.json", client).Check(context.Background())
	if strings.Contains(fmt.Sprintf("%+v", state), "SECRET network path") || state.Status != "unavailable" {
		t.Fatal("raw failure retained")
	}
}

func TestRedirectPolicyRejectsOtherHosts(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://evil.example/update.json", nil)
	if safeRedirect(req, []*http.Request{{URL: req.URL}}) == nil {
		t.Fatal("cross-host redirect accepted")
	}
	if validateSource("https://raw.githubusercontent.com/other/repo/main/update.json") == nil {
		t.Fatal("unexpected manifest source accepted")
	}
	if validateSource(manifestSource+"?ref=dev") == nil {
		t.Fatal("modified manifest source accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}
