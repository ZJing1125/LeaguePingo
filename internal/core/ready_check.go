package core

// ReadyCheck contains only the local confirmation state, never player identity.
type ReadyCheck struct {
	State          string `json:"state"`
	PlayerResponse string `json:"playerResponse"`
}

func (r ReadyCheck) Pending() bool { return r.State == "InProgress" && r.PlayerResponse == "None" }
