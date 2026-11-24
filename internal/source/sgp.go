package source

import (
	"container/list"
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
	http                    *http.Client
	origin, platform, token string
	expires                 time.Time
	leagueToken             string
	leagueExpires           time.Time
	pages                   sgpPageCache
	heads                   map[string]cachedSGPHead
}

// One gate per SGP feature: a stalled history scan no longer shares a lock or a
// cooldown with rank, match detail or grouping.
type sgpChannels struct {
	history, rank, detail, group serviceGate
}

type cachedSGPPage struct {
	key     string
	page    core.HistoryPage
	expires time.Time
}

type cachedSGPHead struct {
	gameID int64
	usedAt time.Time
}

const sgpPageCacheCapacity = 512
const sgpHeadCacheCapacity = 64
const sgpPageCacheTTL = 15 * time.Second

type sgpPageCache struct {
	items map[string]*list.Element
	order list.List
}

func (c *sgpPageCache) get(key string, now time.Time) (core.HistoryPage, bool) {
	if c.items == nil {
		return core.HistoryPage{}, false
	}
	element, ok := c.items[key]
	if !ok {
		return core.HistoryPage{}, false
	}
	entry := element.Value.(cachedSGPPage)
	if !now.Before(entry.expires) {
		c.remove(element)
		return core.HistoryPage{}, false
	}
	c.order.MoveToFront(element)
	return entry.page, true
}

func (c *sgpPageCache) put(key string, page core.HistoryPage, now time.Time) {
	if c.items == nil {
		c.items = make(map[string]*list.Element, sgpPageCacheCapacity)
	}
	for candidate, element := range c.items {
		if !now.Before(element.Value.(cachedSGPPage).expires) {
			delete(c.items, candidate)
			c.order.Remove(element)
		}
	}
	entry := cachedSGPPage{key: key, page: page, expires: now.Add(sgpPageCacheTTL)}
	if element, ok := c.items[key]; ok {
		element.Value = entry
		c.order.MoveToFront(element)
		return
	}
	c.items[key] = c.order.PushFront(entry)
	for len(c.items) > sgpPageCacheCapacity {
		c.remove(c.order.Back())
	}
}

func (c *sgpPageCache) removePrefix(prefix string) {
	for key, element := range c.items {
		if strings.HasPrefix(key, prefix) {
			delete(c.items, key)
			c.order.Remove(element)
		}
	}
}

func (c *sgpPageCache) remove(element *list.Element) {
	if element == nil {
		return
	}
	delete(c.items, element.Value.(cachedSGPPage).key)
	c.order.Remove(element)
}

func (c *sgpPageCache) clear() {
	c.items = nil
	c.order.Init()
}

func (c *sgpPageCache) len() int { return len(c.items) }

// Caller holds sgpMu. History and ranked stats share the verified origin and
// transport, but their authorization tokens are different and never swapped.
func (s *TencentLCU) sgpClientLocked() *sgpHistory {
	if s.sgp == nil {
		s.sgp = &sgpHistory{}
	}
	r := s.sgp
	if r.http == nil {
		r.http = &http.Client{
			// The safety net sits above the adaptive page ceiling (8s); every
			// caller still passes its own shorter budget.
			Timeout:       9 * time.Second,
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return r
}

func (s *TencentLCU) fetchSGPOrigin(ctx context.Context) (string, string, error) {
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
			return "", "", err
		}
		if json.Unmarshal(raw, item.target) != nil {
			return "", "", apperrors.New(apperrors.InvalidResponse)
		}
	}
	origin, err := trustedSGPOrigin(auth.Platform, configured)
	if err != nil {
		return "", "", err
	}
	return origin, auth.Platform, nil
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
	if offset != 0 {
		return s.loadSGPSummaryPage(ctx, puuid, offset)
	}
	s.sgpMu.Lock()
	epoch := s.sgpEpoch
	s.sgpMu.Unlock()
	key := fmt.Sprintf("%d:%s", epoch, puuid)
	return s.pageReads.read(ctx, key, func(request context.Context) (core.HistoryPage, error) {
		s.sgpMu.Lock()
		current := epoch == s.sgpEpoch
		s.sgpMu.Unlock()
		if !current {
			return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
		}
		return s.loadSGPSummaryPage(request, puuid, offset)
	})
}

func (s *TencentLCU) loadSGPSummaryPage(ctx context.Context, puuid string, offset int) (core.HistoryPage, error) {
	return s.loadSGPQueuePage(ctx, puuid, offset, 0, false)
}
func (s *TencentLCU) loadSGPQueuePage(ctx context.Context, puuid string, offset, queue int, requireVerified bool) (core.HistoryPage, error) {
	gate := &s.channels.history
	if err := gate.blocked(time.Now()); err != nil {
		return core.HistoryPage{}, err
	}
	version := gate.version()
	params := url.Values{"startIndex": {fmt.Sprint(offset)}, "count": {"20"}}
	headKey := puuid
	if queue > 0 {
		params.Set("tag", fmt.Sprintf("q_%d", queue))
		headKey = fmt.Sprintf("%s/q_%d", puuid, queue)
	}
	path := "/match-history-query/v1/products/lol/player/" + url.PathEscape(puuid) + "/SUMMARY?" + params.Encode()
	session, err := s.sgpSession(ctx, false, gate)
	if err != nil {
		return core.HistoryPage{}, gate.failedFor(version, err, "", time.Now())
	}
	s.sgpMu.Lock()
	if !s.sessionValidLocked(session) {
		s.sgpMu.Unlock()
		return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
	}
	if requireVerified && !s.verifiedQueues[session.platform][queue] {
		s.sgpMu.Unlock()
		return core.HistoryPage{}, apperrors.New(apperrors.Unsupported)
	}
	r := session.client
	headBefore, hadHead := r.heads[headKey]
	if cached, ok := r.pages.get(path, time.Now()); offset > 0 && ok {
		s.sgpMu.Unlock()
		return cached, nil
	}
	s.sgpMu.Unlock()

	// The request itself runs outside sgpMu: a slow history page must not block
	// the rank, detail or grouping channels.
	raw, err := s.getSGPAuthorized(ctx, session, path, "/sgp-history", gate)
	if err != nil {
		return core.HistoryPage{}, err
	}
	started := time.Now()
	result, err := core.ParseSGPHistory(raw, puuid, session.platform)
	diagnostics.Default.Record("/compute/history-parse", time.Since(started), 200, err != nil)
	diagnostics.ResponseStage(ctx, "parse", time.Since(started))
	if err != nil {
		return core.HistoryPage{}, s.queueContractFailure(session, queue, err)
	}
	if result.Meta.Returned > 20 {
		return core.HistoryPage{}, s.queueContractFailure(session, queue, errors.New("战绩服务返回超出请求的页范围"))
	}
	page := core.HistoryPage{QueueID: queue, Matches: result.Matches, Meta: result.Meta, Offset: offset, NextOffset: offset + result.Meta.Returned, HasMore: result.Meta.Returned == 20}
	s.sgpMu.Lock()
	defer s.sgpMu.Unlock()
	if ctx.Err() != nil || !s.sessionValidLocked(session) {
		return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
	}
	if headAfter, hasHead := r.heads[headKey]; offset > 0 && (hadHead != hasHead || headBefore.gameID != headAfter.gameID) {
		return core.HistoryPage{}, apperrors.New(apperrors.StaleData)
	}
	if queue > 0 && !validQueuePage(r, path, page, queue) {
		delete(s.verifiedQueues[session.platform], queue)
		return core.HistoryPage{}, apperrors.New(apperrors.Unsupported)
	}
	if offset == 0 {
		var head int64
		if len(result.Matches) > 0 {
			head = result.Matches[0].GameID
		}
		r.rememberHead(headKey, head)
	}
	r.pages.put(path, page, time.Now())
	return page, nil
}

func (r *sgpHistory) rememberHead(puuid string, head int64) {
	now := time.Now()
	if r.heads == nil {
		r.heads = make(map[string]cachedSGPHead, sgpHeadCacheCapacity)
	}
	if previous, ok := r.heads[puuid]; !ok || previous.gameID != head {
		identity := strings.SplitN(puuid, "/q_", 2)[0]
		prefix := "/match-history-query/v1/products/lol/player/" + url.PathEscape(identity) + "/SUMMARY?"
		if puuid == identity {
			for key := range r.heads {
				if strings.HasPrefix(key, identity+"/q_") {
					delete(r.heads, key)
				}
			}
			r.pages.removePrefix(prefix)
		} else {
			tag := "&tag=q_" + strings.SplitN(puuid, "/q_", 2)[1]
			for key, element := range r.pages.items {
				if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, tag) {
					r.pages.order.Remove(element)
					delete(r.pages.items, key)
				}
			}
		}
	}
	if _, exists := r.heads[puuid]; !exists && len(r.heads) >= sgpHeadCacheCapacity {
		var oldestKey string
		var oldest time.Time
		for key, candidate := range r.heads {
			if oldestKey == "" || candidate.usedAt.Before(oldest) {
				oldestKey, oldest = key, candidate.usedAt
			}
		}
		delete(r.heads, oldestKey)
		r.pages.removePrefix("/match-history-query/v1/products/lol/player/" + url.PathEscape(strings.SplitN(oldestKey, "/q_", 2)[0]) + "/SUMMARY?")
	}
	r.heads[puuid] = cachedSGPHead{gameID: head, usedAt: now}
}

func (r *sgpHistory) getAuthorized(ctx context.Context, origin, path, token, category string, gate *serviceGate) (raw []byte, status int, resultErr error) {
	return r.getAuthorizedChecked(ctx, origin, path, token, category, gate, nil)
}

func (r *sgpHistory) getAuthorizedChecked(ctx context.Context, origin, path, token, category string, gate *serviceGate, check func() error) (raw []byte, status int, resultErr error) {
	queued := time.Now()
	limit := sgpChannelConcurrency
	if category == "/sgp-history" {
		limit = sgpHistoryConcurrency
	}
	if err := gate.acquireLimit(ctx, limit); err != nil {
		return nil, apperrors.Status(err), err
	}
	diagnostics.ResponseStage(ctx, "queue", time.Since(queued))
	defer gate.release()
	version := gate.version()
	if check != nil {
		if err := check(); err != nil {
			return nil, 0, err
		}
	}
	started := time.Now()
	defer func() {
		diagnostics.Default.Record(category, time.Since(started), status, resultErr != nil)
		apperrors.Failures.Record(apperrors.DataService, resultErr)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+path, nil)
	if err != nil {
		return nil, 0, apperrors.New(apperrors.InvalidResponse)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := r.http.Do(req)
	if err != nil {
		return nil, 0, gate.failedFor(version, apperrors.Request(ctx, err, apperrors.DataService), "", time.Now())
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, response.StatusCode, gate.failedFor(version, apperrors.HTTP(apperrors.DataService, response.StatusCode), response.Header.Get("Retry-After"), time.Now())
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		if ctx.Err() != nil {
			return nil, response.StatusCode, gate.failedFor(version, apperrors.Normalize(ctx.Err(), apperrors.InvalidResponse), "", time.Now())
		}
		if err != nil {
			return nil, response.StatusCode, gate.failedFor(version, apperrors.Normalize(err, apperrors.InvalidResponse), "", time.Now())
		}
		return nil, response.StatusCode, apperrors.New(apperrors.InvalidResponse)
	}
	gate.clearFor(version)
	return raw, response.StatusCode, nil
}
