package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

// Session credentials never cross the Go boundary or leave memory. Only this
// client's current Tencent platform is supported, not arbitrary/cross-region URLs.
type sgpHistory struct {
	cooldown                serviceCooldown
	http                    *http.Client
	origin, platform, token string
	expires                 time.Time
	leagueToken             string
	leagueExpires           time.Time
	pages                   map[string]cachedSGPPage
	heads                   map[string]int64
}

type cachedSGPPage struct {
	page    core.HistoryPage
	expires time.Time
}

// Caller holds sgpMu. History and ranked stats share the verified origin and
// transport, but their authorization tokens are different and never swapped.
func (s *TencentLCU) sgpClientLocked() *sgpHistory {
	if s.sgp == nil {
		s.sgp = &sgpHistory{}
	}
	r := s.sgp
	if r.http == nil {
		r.http = &http.Client{
			Timeout:       5 * time.Second,
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return r
}

func (s *TencentLCU) ensureSGPOrigin(ctx context.Context, r *sgpHistory) error {
	if r.origin != "" && ((r.token != "" && time.Now().Before(r.expires)) || (r.leagueToken != "" && time.Now().Before(r.leagueExpires))) {
		_, err := trustedSGPOrigin(r.platform, r.origin)
		return err
	}
	var auth struct {
		Platform string `json:"currentPlatformId"`
	}
	var configured string
	for _, item := range []struct {
		path   string
		target any
	}{
		{"/lol-rso-auth/v1/authorization", &auth},
		{"/lol-platform-config/v1/namespaces/PlayerPreferences/ServiceEndpoint", &configured},
	} {
		raw, err := s.client.Get(ctx, item.path)
		if err != nil {
			return err
		}
		if json.Unmarshal(raw, item.target) != nil {
			return apperrors.New(apperrors.InvalidResponse)
		}
	}
	origin, err := trustedSGPOrigin(auth.Platform, configured)
	if err != nil {
		return err
	}
	if r.origin != origin || r.platform != auth.Platform {
		r.token, r.leagueToken = "", ""
		r.pages, r.heads = nil, nil
	}
	r.origin, r.platform = origin, auth.Platform
	return nil
}

var tencentPlatform = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,11}$`)

func trustedSGPOrigin(platform, configured string) (string, error) {
	if !tencentPlatform.MatchString(platform) {
		return "", apperrors.New(apperrors.Unsupported)
	}
	expected := "https://" + strings.ToLower(platform) + "-sgp.lol.qq.com:21019"
	if configured != expected && configured != expected+"/" {
		return "", apperrors.New(apperrors.Unsupported)
	}
	return expected, nil
}

func (s *TencentLCU) sgpHistoryPage(ctx context.Context, puuid string, offset int) (core.HistoryPage, error) {
	page, err := s.sgpSummaryPage(ctx, puuid, offset)
	page.HasMore = page.HasMore && page.NextOffset <= 980
	return page, err
}

func (s *TencentLCU) sgpSummaryPage(ctx context.Context, puuid string, offset int) (core.HistoryPage, error) {
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	r := s.sgpClientLocked()
	if err := r.cooldown.blocked(time.Now()); err != nil {
		return core.HistoryPage{}, err
	}
	if err := s.ensureHistorySession(ctx, r); err != nil {
		return core.HistoryPage{}, r.cooldown.failed(err, "", time.Now())
	}
	path := "/match-history-query/v1/products/lol/player/" + url.PathEscape(puuid) + "/SUMMARY?" + url.Values{"startIndex": {fmt.Sprint(offset)}, "count": {"20"}}.Encode()
	if cached, ok := r.pages[path]; offset > 0 && ok && time.Now().Before(cached.expires) {
		return cached.page, nil
	}
	raw, status, err := r.get(ctx, path)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		r.token = ""
	}
	if err != nil {
		return core.HistoryPage{}, err
	}
	result, err := core.ParseSGPHistory(raw, puuid, r.platform)
	if err != nil {
		return core.HistoryPage{}, err
	}
	if result.Meta.Returned > 20 {
		return core.HistoryPage{}, errors.New("战绩服务返回超出请求的页范围")
	}
	page := core.HistoryPage{Matches: result.Matches, Meta: result.Meta, Offset: offset, NextOffset: offset + result.Meta.Returned, HasMore: result.Meta.Returned == 20}
	if offset == 0 {
		var head int64
		if len(result.Matches) > 0 {
			head = result.Matches[0].GameID
		}
		r.rememberHead(puuid, head)
	}
	if r.pages == nil || len(r.pages) >= 512 {
		r.pages = map[string]cachedSGPPage{}
	}
	r.pages[path] = cachedSGPPage{page: page, expires: time.Now().Add(15 * time.Second)}
	return page, nil
}

func (s *TencentLCU) ensureHistorySession(ctx context.Context, r *sgpHistory) error {
	if r.token == "" || time.Now().After(r.expires) {
		if err := s.ensureSGPOrigin(ctx, r); err != nil {
			return err
		}
		var token struct {
			Access string `json:"accessToken"`
		}
		raw, err := s.client.Get(ctx, "/entitlements/v1/token")
		if err != nil {
			return err
		}
		if json.Unmarshal(raw, &token) != nil || token.Access == "" || len(token.Access) > 32768 || strings.ContainsAny(token.Access, "\r\n") {
			return apperrors.New(apperrors.NotLoggedIn)
		}
		r.token, r.expires = token.Access, time.Now().Add(2*time.Minute)
	}
	return nil
}

func (r *sgpHistory) rememberHead(puuid string, head int64) {
	if r.heads == nil || len(r.heads) >= 64 {
		r.heads = map[string]int64{}
		r.pages = nil
	}
	if previous, ok := r.heads[puuid]; ok && previous != head {
		prefix := "/match-history-query/v1/products/lol/player/" + url.PathEscape(puuid) + "/SUMMARY?"
		for key := range r.pages {
			if strings.HasPrefix(key, prefix) {
				delete(r.pages, key)
			}
		}
	}
	r.heads[puuid] = head
}

func (r *sgpHistory) get(ctx context.Context, path string) (raw []byte, status int, resultErr error) {
	return r.getAuthorized(ctx, path, r.token, "/sgp-history")
}

func (r *sgpHistory) getAuthorized(ctx context.Context, path, token, category string) (raw []byte, status int, resultErr error) {
	if err := r.cooldown.blocked(time.Now()); err != nil {
		return nil, apperrors.Status(err), err
	}
	started := time.Now()
	defer func() {
		diagnostics.Default.Record(category, time.Since(started), status, resultErr != nil)
		apperrors.Failures.Record(apperrors.DataService, resultErr)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.origin+path, nil)
	if err != nil {
		return nil, 0, apperrors.New(apperrors.InvalidResponse)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := r.http.Do(req)
	if err != nil {
		return nil, 0, r.cooldown.failed(apperrors.Request(ctx, err, apperrors.DataService), "", time.Now())
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, response.StatusCode, r.cooldown.failed(apperrors.HTTP(apperrors.DataService, response.StatusCode), response.Header.Get("Retry-After"), time.Now())
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		if ctx.Err() != nil {
			return nil, response.StatusCode, r.cooldown.failed(apperrors.Normalize(ctx.Err(), apperrors.InvalidResponse), "", time.Now())
		}
		if err != nil {
			return nil, response.StatusCode, r.cooldown.failed(apperrors.Normalize(err, apperrors.InvalidResponse), "", time.Now())
		}
		return nil, response.StatusCode, apperrors.New(apperrors.InvalidResponse)
	}
	r.cooldown = serviceCooldown{}
	return raw, response.StatusCode, nil
}
