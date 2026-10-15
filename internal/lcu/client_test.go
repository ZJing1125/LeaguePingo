package lcu

import "testing"

func TestParseLockfile(t *testing.T) {
	got, err := ParseLockfile("LeagueClient:1234:54321:secret:https")
	if err != nil || got.Port != 54321 || got.Password != "secret" {
		t.Fatalf("parsed credentials=%+v err=%v", got, err)
	}
	if _, err := ParseLockfile("bad"); err == nil {
		t.Fatal("invalid lockfile accepted")
	}
}
