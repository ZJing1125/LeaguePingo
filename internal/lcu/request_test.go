package lcu

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"leaguepingo/internal/apperrors"
)

type requestTransport func(*http.Request) (*http.Response, error)

func (fn requestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

type failingResponseBody struct{}

func (failingResponseBody) Read([]byte) (int, error) { return 0, errors.New("synthetic-private-body") }
func (failingResponseBody) Close() error             { return nil }

func TestRequestErrorsDoNotExposeIdentityURLOrTransportDetails(t *testing.T) {
	for _, tt := range []struct {
		name    string
		failure error
		want    string
		cause   error
	}{
		{"deadline", context.DeadlineExceeded, "超时", context.DeadlineExceeded},
		{"cancelled", context.Canceled, "取消", context.Canceled},
		{"network timeout", &net.DNSError{Err: "synthetic-private-transport", Name: "private-host", IsTimeout: true}, "超时", context.DeadlineExceeded},
		{"connection", errors.New("synthetic-private-transport"), "连接失败", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, method := range []string{"get", "alias"} {
				t.Run(method, func(t *testing.T) {
					c := NewClient(Credentials{Port: 12345, Password: "synthetic-secret"})
					calls := 0
					c.HTTP.Transport = requestTransport(func(*http.Request) (*http.Response, error) { calls++; return nil, tt.failure })
					var err error
					if method == "get" {
						_, err = c.Get(context.Background(), "/lol-summoner/v1/summoners?name=PrivatePlayer%23TEST")
					} else {
						_, err = c.LookupSummonerAlias(context.Background(), "PrivatePlayer", "TEST")
					}
					assertSafeRequestError(t, err, tt.want)
					if tt.cause != nil && !errors.Is(err, tt.cause) {
						t.Fatal("lost safe cancellation/timeout classification")
					}
					if calls != 1 {
						t.Fatal("request unexpectedly retried")
					}
				})
			}
		})
	}
}

func TestAliasRequestUsesJSONAndRetainsCancellation(t *testing.T) {
	c := NewClient(Credentials{Port: 12345, Password: "synthetic-secret"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.HTTP.Transport = requestTransport(func(r *http.Request) (*http.Response, error) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "riot" || password != "synthetic-secret" || r.Method != http.MethodPost || r.URL.Path != "/lol-summoner/v1/summoners/aliases" || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("incorrect alias request contract")
		}
		cancel()
		return nil, errors.New("synthetic-private-transport")
	})
	_, err := c.LookupSummonerAlias(ctx, "PrivatePlayer", "TEST")
	assertSafeRequestError(t, err, "取消")
	if !errors.Is(err, context.Canceled) {
		t.Fatal("request context must take precedence")
	}
}

func TestBodyAndConstructionErrorsAreRedacted(t *testing.T) {
	c := NewClient(Credentials{Port: 12345})
	c.HTTP.Transport = requestTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: failingResponseBody{}}, nil
	})
	_, err := c.Get(context.Background(), "/private-body")
	assertSafeRequestError(t, err, apperrors.Message(apperrors.InvalidResponse))
	_, err = c.Get(context.Background(), "/PrivatePlayer\n")
	assertSafeRequestError(t, err, apperrors.Message(apperrors.InvalidResponse))
	c.HTTP.Transport = requestTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("synthetic-private-body"))}, nil
	})
	_, err = c.Get(context.Background(), "/private-body")
	assertSafeRequestError(t, err, apperrors.Message(apperrors.ServiceBusy))
}

func assertSafeRequestError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("missing safe error class: %v", err)
	}
	for _, forbidden := range []string{"127.0.0.1", "12345", "PrivatePlayer", "private-body", "private-transport", "private-host", "synthetic-secret", "https://"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatal("request error exposed private details")
		}
	}
}
