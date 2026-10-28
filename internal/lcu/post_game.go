package lcu

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"leaguepingo/internal/core"
)

const playAgainPath = "/lol-lobby/v2/play-again"
const startSearchPath = "/lol-lobby/v2/lobby/matchmaking/search"

func (c *Client) PlayAgain(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, playAgainPath, nil)
	return err
}
func (c *Client) StartSearch(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, startSearchPath, nil)
	return err
}
func (c *Client) LobbyControl(ctx context.Context) (core.LobbyControl, error) {
	raw, err := c.Get(ctx, "/lol-lobby/v2/lobby")
	if err != nil {
		return core.LobbyControl{}, err
	}
	return parseLobbyControl(raw)
}
func parseLobbyControl(raw []byte) (core.LobbyControl, error) {
	type member struct {
		PUUID  string `json:"puuid"`
		ID     int64  `json:"summonerId"`
		Leader *bool  `json:"isLeader"`
	}
	var v struct {
		PartyID    string `json:"partyId"`
		CanStart   *bool  `json:"canStartActivity"`
		GameConfig struct {
			QueueID int   `json:"queueId"`
			Custom  *bool `json:"isCustom"`
		} `json:"gameConfig"`
		Local       *member  `json:"localMember"`
		Members     []member `json:"members"`
		Invitations []struct {
			State string `json:"state"`
		} `json:"invitations"`
	}
	invalid := errors.New("房间状态不完整或不支持自动排队")
	if json.Unmarshal(raw, &v) != nil || v.PartyID == "" || v.CanStart == nil || v.GameConfig.QueueID <= 0 || v.GameConfig.Custom == nil || *v.GameConfig.Custom || v.Local == nil || v.Local.Leader == nil || len(v.Members) == 0 {
		return core.LobbyControl{}, invalid
	}
	key := func(m member) string {
		if m.PUUID != "" {
			return "p:" + m.PUUID
		}
		if m.ID > 0 {
			return fmt.Sprintf("s:%d", m.ID)
		}
		return ""
	}
	self := key(*v.Local)
	found := false
	members := make([]string, 0, len(v.Members))
	for _, m := range v.Members {
		id := key(m)
		if id == "" {
			return core.LobbyControl{}, invalid
		}
		members = append(members, id)
		if id == self && m.Leader != nil && *m.Leader == *v.Local.Leader {
			found = true
		}
	}
	if self == "" || !found {
		return core.LobbyControl{}, invalid
	}
	ready := *v.CanStart
	for _, inv := range v.Invitations {
		if inv.State == "Pending" {
			ready = false
		}
	}
	sort.Strings(members)
	return core.LobbyControl{QueueID: v.GameConfig.QueueID, PartyKey: fmt.Sprintf("%x", sha256.Sum256([]byte(v.PartyID))), MembersKey: fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(members, "\x00")))), Leader: *v.Local.Leader, Ready: ready}, nil
}
