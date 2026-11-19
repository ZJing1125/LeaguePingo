package lcu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

const (
	conversationsPath = "/lol-chat/v1/conversations"
	chatSessionPath   = "/lol-chat/v1/session"
)

func (c *Client) requireChatReady(ctx context.Context) error {
	raw, err := c.Get(ctx, chatSessionPath)
	if apperrors.Status(err) == http.StatusNotFound || apperrors.Status(err) == http.StatusMethodNotAllowed {
		return apperrors.New(apperrors.Unsupported)
	}
	if err != nil {
		return err
	}
	var session struct {
		State string `json:"sessionState"`
	}
	if json.Unmarshal(raw, &session) != nil || session.State == "" {
		return apperrors.New(apperrors.InvalidResponse)
	}
	switch session.State {
	case "loaded", "connected":
		return nil
	case "initializing", "connecting", "disconnected":
		diagnostics.Default.Record("/chat-session/not-ready", 0, 0, false)
		return apperrors.New(apperrors.DataUnavailable)
	default:
		return apperrors.New(apperrors.StaleData)
	}
}

func validConversationID(id string) bool {
	if len(id) == 0 || len(id) > 256 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_@.+:-", c)) {
			return false
		}
	}
	return true
}

func chatMessagePath(path string) bool {
	if !strings.HasPrefix(path, conversationsPath+"/") || !strings.HasSuffix(path, "/messages") {
		return false
	}
	id, err := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(path, conversationsPath+"/"), "/messages"))
	return err == nil && validConversationID(id)
}

func (c *Client) ChampSelectConversation(ctx context.Context) (string, error) {
	raw, err := c.Get(ctx, "/lol-gameflow/v1/gameflow-phase")
	var phase string
	if err != nil {
		return "", err
	}
	if json.Unmarshal(raw, &phase) != nil || phase != "ChampSelect" {
		return "", apperrors.New(apperrors.StaleData)
	}
	if err := c.requireChatReady(ctx); err != nil {
		return "", err
	}
	raw, err = c.Get(ctx, conversationsPath)
	if err != nil {
		return "", err
	}
	var rooms []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &rooms) != nil || len(rooms) > 64 {
		return "", apperrors.New(apperrors.InvalidResponse)
	}
	id := ""
	for _, room := range rooms {
		if !strings.EqualFold(room.Type, "championSelect") {
			continue
		}
		if id != "" || !validConversationID(room.ID) {
			return "", apperrors.New(apperrors.InvalidResponse)
		}
		id = room.ID
	}
	if id == "" {
		return "", apperrors.New(apperrors.DataUnavailable)
	}
	return id, nil
}

// A single fixed mutation for the current champ-select conversation. No generic
// chat API is exposed to the desktop bridge, and no failed POST is retried here.
func (c *Client) SendAnalysisBrief(ctx context.Context, id, body, audience string) (bool, error) {
	if !validConversationID(id) || strings.TrimSpace(body) == "" || !utf8.ValidString(body) ||
		len(body) > core.AnalysisBriefMaxBytes || strings.ContainsFunc(body, invalidBriefRune) ||
		audience != "team" && audience != "self" {
		return false, apperrors.New(apperrors.InvalidResponse)
	}
	current, err := c.ChampSelectConversation(ctx)
	if err != nil {
		return false, err
	}
	if current != id || ctx.Err() != nil {
		return false, apperrors.New(apperrors.StaleData)
	}
	kind := "chat"
	if audience == "self" {
		kind = "celebration"
	}
	raw, _ := json.Marshal(map[string]string{"body": body, "type": kind})
	_, err = c.request(ctx, http.MethodPost, conversationsPath+"/"+url.PathEscape(id)+"/messages", raw)
	return true, err
}

func invalidBriefRune(r rune) bool {
	return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
}
