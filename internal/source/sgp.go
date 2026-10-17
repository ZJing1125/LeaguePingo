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

	"leaguepingo/internal/core"
	"leaguepingo/internal/diagnostics"
)

// Session credentials never cross the Go boundary or leave memory. Only this
// client's current Tencent platform is supported, not arbitrary/cross-region URLs.
type sgpHistory struct {
	http                    *http.Client
	origin, platform, token string
	expires                 time.Time
}

var tencentPlatform = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,11}$`)

func trustedSGPOrigin(platform, configured string) (string, error) {
	if !tencentPlatform.MatchString(platform) {
		return "", errors.New("当前大区无法核验")
	}
	expected := "https://" + strings.ToLower(platform) + "-sgp.lol.qq.com:21019"
	if configured != expected && configured != expected+"/" {
		return "", errors.New("客户端未提供可信的当前大区战绩服务")
	}
	return expected, nil
}

func (s *TencentLCU) sgpHistoryPage(ctx context.Context, puuid string, offset int) (core.HistoryPage, error) {
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	if s.sgp == nil {
		s.sgp = &sgpHistory{}
	}
	r := s.sgp
	if r.http == nil {
		r.http = &http.Client{
			Timeout:       5 * time.Second,
			Transport:     &http.Transport{Proxy: nil}, // Normal certificate verification; no inherited proxy.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	if r.token == "" || time.Now().After(r.expires) {
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
			if err != nil || json.Unmarshal(raw, item.target) != nil {
				return core.HistoryPage{}, errors.New("无法核验当前大区战绩服务")
			}
		}
		origin, err := trustedSGPOrigin(auth.Platform, configured)
		if err != nil {
			return core.HistoryPage{}, err
		}
		var token struct {
			Access string `json:"accessToken"`
		}
		raw, err := s.client.Get(ctx, "/entitlements/v1/token")
		if err != nil || json.Unmarshal(raw, &token) != nil || token.Access == "" || len(token.Access) > 32768 || strings.ContainsAny(token.Access, "\r\n") {
			return core.HistoryPage{}, errors.New("当前会话暂未提供战绩授权")
		}
		r.origin, r.platform, r.token, r.expires = origin, auth.Platform, token.Access, time.Now().Add(2*time.Minute)
	}
	path := "/match-history-query/v1/products/lol/player/" + url.PathEscape(puuid) + "/SUMMARY?" + url.Values{"startIndex": {fmt.Sprint(offset)}, "count": {"20"}}.Encode()
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
	return core.HistoryPage{Matches: result.Matches, Meta: result.Meta, Offset: offset, NextOffset: offset + result.Meta.Returned, HasMore: result.Meta.Returned == 20 && offset+result.Meta.Returned <= 980}, nil
}

func (r *sgpHistory) get(ctx context.Context, path string) (raw []byte, status int, resultErr error) {
	started := time.Now()
	defer func() { diagnostics.Default.Record("/sgp-history", time.Since(started), status, resultErr != nil) }()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.origin+path, nil)
	if err != nil {
		return nil, 0, errors.New("战绩请求构造失败")
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	response, err := r.http.Do(req)
	if err != nil {
		return nil, 0, errors.New("完整战绩服务连接失败或超时")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, response.StatusCode, fmt.Errorf("完整战绩服务 HTTP %d", response.StatusCode)
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, response.StatusCode, errors.New("战绩响应读取失败或超出限制")
	}
	return raw, response.StatusCode, nil
}
