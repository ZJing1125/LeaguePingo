package apperrors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

func TestHTTPClassificationAndPresentation(t *testing.T) {
	for _, tt := range []struct {
		status int
		code   Code
	}{
		{401, NotLoggedIn}, {403, AccessDenied}, {404, DataUnavailable}, {408, RequestTimeout},
		{429, RateLimited}, {500, ServiceBusy}, {503, ServiceBusy}, {504, RequestTimeout}, {302, DataUnavailable},
	} {
		for _, service := range []Service{LocalClient, DataService, GuideService} {
			t.Run(fmt.Sprintf("%s/%d", service, tt.status), func(t *testing.T) {
				err := HTTP(service, tt.status)
				want := tt.code
				if service == GuideService && (tt.status == 401 || tt.status == 403) {
					want = GuideUnavailable
				}
				if CodeOf(err) != want || err.Error() != Message(want) {
					t.Fatalf("wrong classification: %v", err)
				}
				var ui Presentation
				if json.Unmarshal(Marshal(fmt.Errorf("private-player#DEMO: %w", err)), &ui) != nil || ui.Code != want || ui.Message != Message(want) {
					t.Fatalf("bad envelope: %+v", ui)
				}
				if strings.Contains(string(Marshal(err)), fmt.Sprint(tt.status)) || strings.Contains(err.Error(), "HTTP") {
					t.Fatal("status appeared in UI")
				}
			})
		}
	}
}

func TestPublicBoundaryRejectsArbitraryErrorTextAndPreservesSafeCauses(t *testing.T) {
	var syntax any
	parseErr := json.Unmarshal([]byte(`{"private-player#DEMO":`), &syntax)
	for _, tt := range []struct {
		err   error
		code  Code
		cause error
	}{
		{errors.New("Get https://host/player/private-player#DEMO?token=secret: HTTP 500"), Unexpected, nil},
		{parseErr, InvalidResponse, nil},
		{&net.DNSError{Name: "private-player", Err: "secret", IsTimeout: true}, RequestTimeout, context.DeadlineExceeded},
		{fmt.Errorf("secret: %w", context.Canceled), RequestCanceled, context.Canceled},
		{fmt.Errorf("private-player: %w", HTTP(DataService, 429)), RateLimited, nil},
	} {
		err := tt.err
		Present(&err)
		if CodeOf(err) != tt.code || err.Error() != Message(tt.code) {
			t.Fatalf("bad public error: %v", err)
		}
		if tt.cause != nil && !errors.Is(err, tt.cause) {
			t.Fatal("lost cancellation classification")
		}
		data := string(Marshal(err))
		for _, secret := range []string{"private-player", "secret", "https://", "HTTP", "500"} {
			if strings.Contains(data, secret) || strings.Contains(err.Error(), secret) {
				t.Fatal("leaked source text")
			}
		}
		if cause := errors.Unwrap(err); cause != nil && cause != context.Canceled && cause != context.DeadlineExceeded {
			t.Fatal("retained raw cause")
		}
	}
	var ok error
	Present(&ok)
	if ok != nil || CodeOf(nil) != "" || string(Marshal(nil)) != "null" {
		t.Fatal("nil must remain success")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := Request(ctx, errors.New("private-host"), LocalClient); CodeOf(got) != RequestCanceled || !errors.Is(got, context.Canceled) {
		t.Fatal("request context did not take precedence")
	}
}

func TestMessageCatalogIsClosedAndActionable(t *testing.T) {
	for code, text := range messages {
		if Message(code) != text || New(code).Error() != text || text == "" {
			t.Fatalf("unmapped code %q", code)
		}
		if !strings.ContainsAny(text, "请可需已") || strings.Contains(text, "HTTP") || strings.Contains(text, "http:") || strings.Contains(text, "https:") {
			t.Fatalf("not a safe actionable message: %s", code)
		}
	}
	if CodeOf(New(Code("private-player"))) != Unexpected || Message("private-player") != Message(Unexpected) {
		t.Fatal("unregistered code escaped vocabulary")
	}
}

func TestFailureDiagnosticsBoundedConcurrentAndIdentityFree(t *testing.T) {
	var c FailureCollector
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 500 {
				c.Record(DataService, fmt.Errorf("https://host/private-player?token=secret: %w", HTTP(DataService, 429)))
				_ = c.Snapshot()
			}
		})
	}
	wg.Wait()
	rows := c.Snapshot()
	if len(rows) != 1 || rows[0].Code != RateLimited || rows[0].Status != 429 || rows[0].Count != 4000 {
		t.Fatalf("bad aggregate %+v", rows)
	}
	rows[0].Count = 0
	if c.Snapshot()[0].Count != 4000 {
		t.Fatal("snapshot mutated collector")
	}
	for status := 100; status < 600; status++ {
		c.Record(Service("private-player"), HTTP(DataService, status))
	}
	if len(c.Snapshot()) > 128 {
		t.Fatal("diagnostic cardinality not bounded")
	}
	data, _ := json.Marshal(c.Snapshot())
	for _, secret := range []string{"private-player", "token", "secret", "https://"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("diagnostic leak")
		}
	}
}
