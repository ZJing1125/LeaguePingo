package source

import "context"

type AnalysisBriefSource interface {
	ChampSelectConversation(context.Context) (string, error)
	SendAnalysisBrief(context.Context, string, string, string) error
}
type SelectionIdentitySource interface{ SelectionIdentity() string }

func (s *TencentLCU) SelectionIdentity() string {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	return s.selectChat
}
func (s *TencentLCU) ChampSelectConversation(ctx context.Context) (string, error) {
	return s.client.ChampSelectConversation(ctx)
}
func (s *TencentLCU) SendAnalysisBrief(ctx context.Context, id, body, audience string) error {
	return s.client.SendAnalysisBrief(ctx, id, body, audience)
}
