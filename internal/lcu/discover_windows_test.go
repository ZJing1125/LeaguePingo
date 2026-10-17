//go:build windows

package lcu

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestProcessCommandLineArgs(t *testing.T) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	args, err := processCommandLineArgs(handle)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, os.Args) {
		t.Fatal("Windows command line arguments differ from os.Args (values redacted)")
	}
}

func TestDiscoverWithValidOverride(t *testing.T) {
	path := writeTestLockfile(t, "LeagueClient:1234:54321:test-only-token:https")
	t.Setenv("LEAGUEPINGO_LOCKFILE", path)
	client, err := Discover()
	if err != nil || client == nil || client.Cred.Port != 54321 {
		t.Fatalf("valid override not discovered: %v", err)
	}
}

// Opt-in only: one local phase GET, no player IDs, roster or history requests.
// Never print credentials or full startup arguments, even on failure.
func TestLiveClientPhase(t *testing.T) {
	if os.Getenv("LEAGUEPINGO_LIVE_LCU_TEST") != "1" {
		t.Skip("set LEAGUEPINGO_LIVE_LCU_TEST=1 for a running-client smoke test")
	}
	// Exercise automatic process discovery, not a developer's override file.
	t.Setenv("LEAGUEPINGO_LOCKFILE", "")
	client, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	defer client.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := client.Get(ctx, "/lol-gameflow/v1/gameflow-phase")
	if err != nil {
		t.Fatal(err)
	}
	var phase string
	if err := json.Unmarshal(data, &phase); err != nil {
		t.Fatal("LCU phase response is not a JSON string")
	}
	switch phase {
	case "None", "Lobby", "Matchmaking", "ReadyCheck", "ChampSelect", "GameStart", "InProgress", "Reconnect", "WaitingForStats", "PreEndOfGame", "EndOfGame", "TerminatedInError", "WatchInProgress":
		t.Logf("Live local LCU connection successful; phase=%s", phase)
	default:
		t.Fatal("LCU returned an unrecognized phase (response redacted)")
	}
}
