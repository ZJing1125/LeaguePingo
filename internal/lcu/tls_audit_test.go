package lcu

// S9 is deliberately test-only until a real Tencent LCU verifies this candidate.
// Riot documents this root for Game Client API, not a guarantee for every LCU.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/pem"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Official public certificate, downloaded 2026-11-04; never a client credential.
// https://static.developer.riotgames.com/docs/lol/riotgames.pem
//
//go:embed testdata/riot-game-client-root.pem
var riotRootCandidate []byte

func candidateRoots(t *testing.T) *x509.CertPool {
	t.Helper()
	block, rest := pem.Decode(riotRootCandidate)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		t.Fatal("published Riot candidate must contain exactly one certificate")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal("published Riot candidate is not a certificate")
	}
	digest := sha256.Sum256(certificate.Raw)
	if hex.EncodeToString(digest[:]) != "ca8c9d325b4cdc464c6c94a585c85e91ec23d40ba5bf3ae2822b951a4a504ea3" {
		t.Fatal("published Riot candidate fingerprint changed; research before replacing trust")
	}
	if now := time.Now(); now.Before(certificate.NotBefore) || now.After(certificate.NotAfter) {
		t.Fatal("published Riot candidate is outside its validity period")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(riotRootCandidate) {
		t.Fatal("published Riot candidate could not be added to isolated roots")
	}
	return roots
}

func TestPublishedRiotRootCandidate(t *testing.T) {
	// Integrity/parse evidence only, not proof of an actual LCU certificate chain.
	candidateRoots(t)
}

type tlsAuditTransport struct {
	http.RoundTripper
	websocket chan bool
}

func (transport tlsAuditTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.RoundTripper.RoundTrip(request)
	if strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
		select {
		case transport.websocket <- err == nil && response != nil && response.StatusCode == http.StatusSwitchingProtocols:
		default:
		}
	}
	return response, err
}

func TestLiveLCUTLSCandidate(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_TLS_TEST") != "1" {
		t.Skip("NOT RUN: explicit read-only Tencent LCU TLS verification only")
	}
	compatible, err := Discover()
	if err != nil {
		t.Skip("NOT RUN: no local client; strict TLS remains unverified")
	}
	defer compatible.HTTP.CloseIdleConnections()
	transport := compatible.HTTP.Transport.(*http.Transport).Clone()
	// Standard chain, validity and 127.0.0.1 hostname verification all stay on.
	// No insecure flag, custom bypass, system roots or compatibility fallback.
	transport.TLSClientConfig = &tls.Config{RootCAs: candidateRoots(t)}
	defer transport.CloseIdleConnections()
	handshake := make(chan bool, 1)
	client := &Client{Cred: compatible.Cred, HTTP: &http.Client{
		Timeout:       compatible.HTTP.Timeout,
		Transport:     tlsAuditTransport{RoundTripper: transport, websocket: handshake},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if _, err := client.Get(ctx, "/lol-gameflow/v1/gameflow-phase"); err != nil {
		t.Fatal("NOT VERIFIED: strict Riot-candidate HTTPS; no fallback or writes attempted")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = client.WatchPhase(ctx, func(string) {})
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("TLS audit subscription did not stop within its cleanup bound")
		}
	}()
	select {
	case verified := <-handshake:
		if !verified {
			t.Fatal("NOT VERIFIED: strict Riot-candidate WSS; no fallback or writes attempted")
		}
	case <-ctx.Done():
		t.Fatal("NOT VERIFIED: strict Riot-candidate WSS timed out")
	}
	t.Log("VERIFIED: strict candidate HTTPS and WSS for this client only; no identity, credentials, payloads or writes exported")
}
