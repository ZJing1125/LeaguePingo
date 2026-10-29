package lcu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"leaguepingo/internal/apperrors"
)

const postGameSequencePath = "/lol-pre-end-of-game/v1/currentSequenceEvent"
const continuePostGamePath = "/lol-pre-end-of-game/v1/complete/missions-celebration"

func (c *Client) PostGameSequence(ctx context.Context) (string, error) {
	raw, err := c.Get(ctx, postGameSequencePath)
	if err != nil {
		return "", err
	}
	return parsePostGameSequence(raw)
}

func parsePostGameSequence(raw []byte) (string, error) {
	if len(raw) > 4096 {
		return "", apperrors.New(apperrors.InvalidResponse)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var v struct {
		Name *string `json:"name"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Name == nil {
		return "", apperrors.New(apperrors.InvalidResponse)
	}
	if *v.Name == "missions-celebration" || *v.Name == "" {
		return *v.Name, nil
	}
	return "other", nil // Never retain/export arbitrary event names.
}

func (c *Client) ContinuePostGameProgress(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, continuePostGamePath, nil)
	return err
}
