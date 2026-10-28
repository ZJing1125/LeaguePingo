package core

// LobbyControl is a minimal, in-memory view for local matchmaking actions.
// Keys are hashes; no names, member IDs, or raw lobby responses reach the UI.
type LobbyControl struct {
	QueueID    int
	PartyKey   string
	MembersKey string
	Leader     bool
	Ready      bool
}
