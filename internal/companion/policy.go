// Package companion contains window policy only. It never requests player data.
package companion

import (
	"fmt"
	"time"

	"leaguepingo/internal/core"
	"leaguepingo/internal/settings"
)

const PreviewDuration = 6 * time.Second
const EntryWindow = 45 * time.Second

// InGame includes the loading screen. Window/shortcut eligibility is separate
// from the engine's InProgress-only player-data preparation budget.
func InGame(s core.Snapshot) bool {
	return s.Connected && (s.Phase == "GameStart" || s.Phase == "InProgress")
}

func Pregame(s core.Snapshot) bool {
	if !s.Connected {
		return false
	}
	switch s.Phase {
	case "Lobby", "Matchmaking", "ReadyCheck", "ChampSelect":
		return true
	}
	return false
}

// Preview remembers actual matches, rather than connection generations. A
// reconnect or late roster/report must not renew a match's automatic display.
type Preview struct {
	key        string
	entered    time.Time
	used       map[string]bool
	order      []string
	phase      string
	lastID     int64
	suppressed bool
}

func (p *Preview) Observe(s core.Snapshot, now time.Time) {
	previous := p.phase
	previousID := p.lastID
	p.phase = s.Phase
	p.lastID = s.GameID
	// A champ-select session can already expose the final game ID. It must not
	// consume the future game's preview before loading/play has begun.
	if !s.Connected || (!InGame(s) && s.Phase != "Reconnect") {
		p.Consume()
		p.entered = time.Time{}
		p.suppressed = false
		return
	}
	if p.entered.IsZero() {
		p.entered = now
	}
	if s.Phase == "Reconnect" {
		p.suppressed = true
	}
	if s.GameID <= 0 {
		return
	}
	key := fmt.Sprintf("%s:%d", s.Region, s.GameID)
	if key != p.key {
		p.key = key
		if InGame(s) && (previous == "GameStart" || previous == "InProgress" || previous == "Reconnect") && previousID > 0 && previousID != s.GameID {
			p.entered = now
			p.suppressed = false
		}
	}
	if p.suppressed {
		p.Consume()
	}
}

func (p *Preview) Eligible(s core.Snapshot, now time.Time) bool {
	return InGame(s) && s.GameID > 0 && len(s.Participants) > 0 && !p.entered.IsZero() &&
		p.key == fmt.Sprintf("%s:%d", s.Region, s.GameID) && !p.used[p.key] && !now.Before(p.entered) && now.Sub(p.entered) <= EntryWindow
}

func (p *Preview) Consume() {
	// An explicit hold/manual action may arrive while the loading game's ID
	// is still missing. Remember it for this entry, so a late ID cannot pop up
	// an automatic preview after the player has already dismissed the window.
	if !p.entered.IsZero() {
		p.suppressed = true
	}
	if p.key == "" || p.used[p.key] {
		return
	}
	if p.used == nil {
		p.used = map[string]bool{}
	}
	p.used[p.key] = true
	p.order = append(p.order, p.key)
	if len(p.order) > 32 {
		delete(p.used, p.order[0])
		p.order = p.order[1:]
	}
}

type Placement struct {
	Bounds settings.Rect
	Side   string
	Rail   bool
}

// Place uses physical pixels, including negative monitor coordinates. The gap
// is decorative: reduce it before treating an otherwise fitting sidebar as
// overflow. It never resizes/moves the client.
func Place(client, work settings.Rect, width, railWidth, gap int, collapsed, expanded bool) Placement {
	width = min(width, work.Width)
	leftSpace := client.X - work.X
	rightSpace := work.X + work.Width - client.X - client.Width
	left, right := leftSpace >= width, rightSpace >= width
	rail := collapsed || (!left && !right && !expanded)
	w := width
	if rail {
		w = railWidth
	}
	x, side := client.X, "left"
	if left {
		x = client.X - w - min(gap, max(0, leftSpace-w))
	} else if right {
		x, side = client.X+client.Width+min(gap, max(0, rightSpace-w)), "right"
	}
	height := min(client.Height, work.Height)
	y := max(work.Y, min(client.Y, work.Y+work.Height-height))
	x = max(work.X, min(x, work.X+work.Width-w))
	return Placement{settings.Rect{X: x, Y: y, Width: w, Height: height}, side, rail}
}
