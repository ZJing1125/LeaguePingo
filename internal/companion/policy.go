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
	key     string
	entered time.Time
	used    map[string]bool
	order   []string
	phase   string
	lastID  int64
}

func (p *Preview) Observe(s core.Snapshot, now time.Time) {
	previous := p.phase
	previousID := p.lastID
	p.phase = s.Phase
	p.lastID = s.GameID
	// A champ-select session can already expose the final game ID. It must not
	// consume the future game's preview before loading/play has begun.
	if !s.Connected || (s.Phase != "GameStart" && s.Phase != "InProgress" && s.Phase != "Reconnect") {
		p.Consume()
		p.entered = time.Time{}
		return
	}
	if s.Phase == "GameStart" {
		p.entered = time.Time{}
	}
	if s.Phase == "InProgress" && previous != "InProgress" {
		p.entered = now
	}
	if s.GameID <= 0 {
		if s.Phase == "Reconnect" {
			p.Consume()
		}
		return
	}
	key := fmt.Sprintf("%s:%d", s.Region, s.GameID)
	if key != p.key {
		p.key = key
		if p.entered.IsZero() || (s.Phase == "InProgress" && previous == "InProgress" && previousID > 0 && previousID != s.GameID) {
			p.entered = now
		}
	}
	if s.Phase == "Reconnect" || previous == "Reconnect" {
		p.Consume()
	}
}

func (p *Preview) Eligible(s core.Snapshot, now time.Time) bool {
	return s.Connected && s.Phase == "InProgress" && s.GameID > 0 && len(s.Participants) > 0 &&
		p.key == fmt.Sprintf("%s:%d", s.Region, s.GameID) && !p.used[p.key] && now.Sub(p.entered) <= EntryWindow
}

func (p *Preview) Consume() {
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

// Place uses physical pixels, including negative monitor coordinates. It never
// resizes/moves the client. Overflow opens inward from a left-hand rail.
func Place(client, work settings.Rect, width, railWidth, gap int, collapsed, expanded bool) Placement {
	width = min(width, work.Width)
	left := client.X-gap-width >= work.X
	right := client.X+client.Width+gap+width <= work.X+work.Width
	x, side := client.X-gap-width, "left"
	if !left && right {
		x, side = client.X+client.Width+gap, "right"
	}
	rail := collapsed || (!left && !right && !expanded)
	w := width
	if rail {
		w = railWidth
		if left {
			x = client.X - gap - w
		} else if right {
			x = client.X + client.Width + gap
		} else {
			x = client.X
		}
	} else if !left && !right {
		x = client.X
	}
	height := min(client.Height, work.Height)
	y := max(work.Y, min(client.Y, work.Y+work.Height-height))
	x = max(work.X, min(x, work.X+work.Width-w))
	return Placement{settings.Rect{X: x, Y: y, Width: w, Height: height}, side, rail}
}
