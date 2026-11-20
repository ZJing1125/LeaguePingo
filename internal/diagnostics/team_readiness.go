package diagnostics

import "time"

// Aggregate counts only, retained for the most recent selection and this run.
type TeamReadinessMetric struct {
	Started          uint64 `json:"started"`
	FirstRated       uint64 `json:"firstRated"`
	Exited           uint64 `json:"exited"`
	LastFirstRatedMS *int64 `json:"lastFirstRatedMs"`
	LastVisible      int    `json:"lastVisible"`
	LastKnown        int    `json:"lastKnown"`
	LastRated        int    `json:"lastRated"`
	LastInsufficient int    `json:"lastInsufficient"`
	LastReading      int    `json:"lastReading"`
}

func (r *Readiness) ObserveRatings(now time.Time, visible, known, rated, insufficient, reading int) {
	if !r.ratingActive {
		return
	}
	r.ratings.LastVisible, r.ratings.LastKnown, r.ratings.LastRated = visible, known, rated
	r.ratings.LastInsufficient, r.ratings.LastReading = insufficient, reading
	if rated > 0 && !r.ratingFirst {
		elapsed := now.Sub(r.start).Milliseconds()
		r.ratings.LastFirstRatedMS = &elapsed
		r.ratings.FirstRated++
		r.ratingFirst = true
	}
}
func (r *Readiness) Ratings() TeamReadinessMetric { return r.ratings }
