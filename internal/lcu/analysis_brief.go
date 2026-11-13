package lcu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"leaguepingo/internal/apperrors"
)

const conversationsPath = "/lol-chat/v1/conversations"

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
func (c *Client) SendAnalysisBrief(ctx context.Context, id, body, audience string) error {
	if !validConversationID(id) || !utf8.ValidString(body) || utf8.RuneCountInString(body) > 450 || len(body) > 1800 || body == "" || audience != "team" && audience != "self" {
		return apperrors.New(apperrors.InvalidResponse)
	}
	current, err := c.ChampSelectConversation(ctx)
	if err != nil {
		return err
	}
	if current != id {
		return apperrors.New(apperrors.StaleData)
	}
	kind := "chat"
	if audience == "self" {
		kind = "celebration"
	}
	raw, _ := json.Marshal(map[string]string{"body": body, "type": kind})
	_, err = c.request(ctx, http.MethodPost, conversationsPath+"/"+url.PathEscape(id)+"/messages", raw)
	return err
}
