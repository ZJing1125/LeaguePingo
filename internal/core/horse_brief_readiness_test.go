package core

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHorseBriefReadinessAndFivePhysicalSeats(t *testing.T) {
	s := Snapshot{QueueID: 420, SelfKey: "p0"}
	for i := range 5 {
		cell := i
		s.Participants = append(s.Participants, Participant{Key: fmt.Sprint("p", i), Team: "ally", CellID: &cell, IdentityState: IdentityUnavailable})
	}
	if _, ready, _ := HorseBriefLines(s, true); ready {
		t.Fatal("anonymous placeholders consumed the early-send opportunity")
	}
	for i := range s.Participants {
		a := BuildPlayerAnalysis(horseHistory(15, 420, 60), 420, time.Now(), false)
		s.Participants[i].IdentityState = IdentityKnown
		s.Participants[i].Scout = &ScoutReport{State: "loading", Queues: []ScoutQueue{{QueueID: 420, Analysis: a}}}
	}
	if _, ready, rated := HorseBriefLines(s, true); !ready || rated != 5 {
		t.Fatal("completed 15-game sample waited for deeper history")
	}
	s.Participants[0].Scout.Queues[0].Analysis.Version = 1
	if _, ready, _ := HorseBriefLines(s, true); ready {
		t.Fatal("legacy score consumed new model's preparation window")
	}
	s.Participants[0].Scout.Queues[0].Analysis.Version = 3
	saved := s.Participants[0].Scout
	s.Participants[0].Scout = nil
	if _, ready, _ := HorseBriefLines(s, false); ready {
		t.Fatal("display preference skipped self's unfinished team score")
	}
	s.Participants[0].Scout = saved
	s.Participants = s.Participants[:3]
	lines, ready, rated := HorseBriefLines(s, true)
	if len(lines) != 6 || ready || rated != 3 || !strings.Contains(lines[5], "5楼(待定) 资料待取") {
		t.Fatal("missing slots changed the team denominator", lines)
	}
	lines, _, rated = HorseBriefLines(s, false)
	if len(lines) != 5 || rated != 2 || strings.Contains(AnalysisBriefBody(lines), "1楼") {
		t.Fatal("include-self preference was lost")
	}
}

func TestHorseBriefTerminalFetchStates(t *testing.T) {
	for _, tc := range []struct {
		status       FetchStatus
		reason, text string
	}{
		{StatusPending, "", "资料待取"},
		{StatusNoMatchInQueue, "", "无本模式战绩"},
		{StatusTimeoutOrError, "RequestTimeout", "网络超时/资料待取"},
		{StatusTimeoutOrError, "ServiceBusy", "资料暂不可用"},
		{StatusSuccess, "", "字段不足"},
	} {
		text, rated := horseBriefText(&ScoutReport{Reason: tc.reason}, &ScoutQueue{}, tc.status, nil)
		if text != tc.text || rated {
			t.Fatalf("%s: %s", tc.status, text)
		}
	}
}
