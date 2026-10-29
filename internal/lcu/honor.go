package lcu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"leaguepingo/internal/apperrors"
)

const honorBallotPath = "/lol-honor-v2/v1/ballot"
const submitHonorBallotPath = "/lol-honor/v1/ballot"
const honorCompletionPath = "/lol-honor-v2/v1/vote-completion"

func (c *Client) HonorVoteCompletion(ctx context.Context) (int64, error) {
	raw, err := c.Get(ctx, honorCompletionPath)
	if err != nil {
		return 0, err
	}
	return parseHonorCompletion(raw)
}

func parseHonorCompletion(raw []byte) (int64, error) {
	var v struct {
		GameID *int64 `json:"gameId"`
	}
	if len(raw) > 4096 || json.Unmarshal(raw, &v) != nil || v.GameID == nil || *v.GameID < 0 {
		return 0, apperrors.New(apperrors.InvalidResponse)
	}
	// fullTeamVote=false still represents this game's completed ballot.
	return *v.GameID, nil
}

// Only retain the game and whether the user has already selected any honors.
// Ballot identities and candidate lists never leave this transient parser.
func (c *Client) HonorBallot(ctx context.Context) (int64, bool, error) {
	raw, err := c.Get(ctx, honorBallotPath)
	if err != nil {
		return 0, false, err
	}
	return parseHonorBallot(raw)
}

func parseHonorBallot(raw []byte) (int64, bool, error) {
	invalid := apperrors.New(apperrors.InvalidResponse)
	if len(raw) > 64<<10 {
		return 0, false, invalid
	}
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return 0, false, nil
	}
	var v struct {
		GameID *int64          `json:"gameId"`
		Honors json.RawMessage `json:"honoredPlayers"`
	}
	if json.Unmarshal(raw, &v) != nil || v.GameID == nil || *v.GameID < 0 {
		return 0, false, invalid
	}
	if *v.GameID == 0 {
		return 0, false, nil
	}
	var honors []struct{}
	if len(v.Honors) == 0 || bytes.Equal(bytes.TrimSpace(v.Honors), []byte("null")) || json.Unmarshal(v.Honors, &honors) != nil || len(honors) > 10 {
		return 0, false, invalid
	}
	return *v.GameID, len(honors) > 0, nil
}

// Finalizes an empty ballot; never sends a recipient or an honor category.
// The controller must recheck an unselected ballot immediately before calling.
func (c *Client) CompleteHonorBallot(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, submitHonorBallotPath, nil)
	return err
}
