package lcu

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLockfile(t *testing.T) {
	got, err := ParseLockfile("LeagueClient:1234:54321:secret:https")
	if err != nil || got.Port != 54321 || got.Password != "secret" {
		t.Fatalf("parsed credentials=%+v err=%v", got, err)
	}
	if _, err := ParseLockfile("bad"); err == nil {
		t.Fatal("invalid lockfile accepted")
	}
}

func writeTestLockfile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lockfile")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEmptyLockfileExplainsDiscoveryFailure(t *testing.T) {
	client, err := DiscoverFromPaths([]string{writeTestLockfile(t, "")})
	if client != nil || !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "lockfile 为空") {
		t.Fatalf("expected explicit empty-lockfile error: %v", err)
	}
}

func TestDiscoverSkipsInvalidPaths(t *testing.T) {
	empty := writeTestLockfile(t, "")
	valid := writeTestLockfile(t, "LeagueClient:1234:54321:test-only-token:https")
	client, err := DiscoverFromPaths([]string{"", filepath.Join(t.TempDir(), "missing"), empty, valid})
	if err != nil || client == nil || client.Cred.Port != 54321 {
		t.Fatalf("valid fallback path not discovered: %v", err)
	}
}

func TestParseConnectionArgs(t *testing.T) {
	for _, args := range [][]string{
		{"LeagueClientUx.exe", "--app-port=54321", "--remoting-auth-token=test-only-token", "--region=HN1"},
		{"LeagueClientUx.exe", "--app-port", "54321", "--remoting-auth-token", "test-only-token", "--region", "HN1"},
		{"--riotclient-app-port=12345", "--riotclient-auth-token=unrelated", "--app-port=54321", "--remoting-auth-token=test-only-token", "--region=HN1"},
	} {
		cred, err := ParseConnectionArgs(args)
		if err != nil || cred.Port != 54321 || cred.Password != "test-only-token" || cred.Region != "HN1" {
			t.Fatalf("LCU flags not parsed correctly: %v", err)
		}
	}
}

func TestParseConnectionArgsRejectsInvalidInputWithoutLeakingSecrets(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--riotclient-app-port=12345", "--riotclient-auth-token=sensitive-test-token"},
		{"--app-port=0", "--remoting-auth-token=sensitive-test-token"},
		{"--app-port=65536", "--remoting-auth-token=sensitive-test-token"},
		{"--app-port=sensitive-test-token", "--remoting-auth-token=sensitive-test-token"},
		{"--app-port=54321", "--remoting-auth-token="},
		{"--app-port=54321", "--remoting-auth-token"},
		{"--app-port", "--remoting-auth-token=sensitive-test-token"},
		{"--app-port=54321", "--app-port=54322", "--remoting-auth-token=sensitive-test-token"},
		{"--app-port=54321", "--remoting-auth-token=sensitive-test-token\n"},
	} {
		_, err := ParseConnectionArgs(args)
		if err == nil {
			t.Fatal("invalid arguments accepted")
		}
		if strings.Contains(err.Error(), "sensitive-test-token") {
			t.Fatal("secret leaked in parse error")
		}
	}
}

func TestParseLockfileRejectsInvalidInputWithoutLeakingSecrets(t *testing.T) {
	for _, raw := range []string{"", " \n", "LeagueClient:0:54321:sensitive-test-token:https", "LeagueClient:1:65536:sensitive-test-token:https", "LeagueClient:1:54321:sensitive-test-token:http"} {
		_, err := ParseLockfile(raw)
		if err == nil || strings.Contains(err.Error(), "sensitive-test-token") {
			t.Fatal("invalid lockfile accepted or secret leaked")
		}
	}
}
