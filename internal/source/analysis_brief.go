package source

import (
	"context"

	"leaguepingo/internal/apperrors"
)

type AnalysisBriefSource interface {
	ChampSelectConversation(context.Context) (string, error)
	SendAnalysisBrief(context.Context, string, string, string) error
}

func (s *TencentLCU) SelectionIdentity() string { return s.SessionMetadata().SelectionKey }

func (s *TencentLCU) ChampSelectConversation(ctx context.Context) (string, error) {
	return s.client.ChampSelectConversation(ctx)
}
func (s *TencentLCU) SendAnalysisBrief(ctx context.Context, id, body, audience string) error {
	return s.client.SendAnalysisBrief(ctx, id, body, audience)
}

// SelectionVerifier makes a recheck possible when the client exposes a stable
// identity for the selection used to build the brief.
type SelectionVerifier interface {
	VerifySelection(context.Context, string, int64, int) error
}

func (s *TencentLCU) VerifySelection(ctx context.Context, key string, gameID int64, queue int) error {
	if key == "" && gameID == 0 {
		return nil
	}
	raw, err := s.client.Get(ctx, champSelectSession)
	if err != nil {
		return err
	}
	m := readSelectionMetadata(raw)
	if key != "" && m.Key != key || gameID != 0 && (key == "" || m.GameID != 0) && m.GameID != gameID ||
		m.QueueID > 0 && m.QueueID != queue {
		return apperrors.New(apperrors.StaleData)
	}
	return nil
}
