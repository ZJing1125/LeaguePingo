// Package updatecheck reads the public, static release manifest without
// downloading or installing software.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxManifestBytes = 32 << 10
const manifestSource = "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/main/update.json"

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
	URL           string `json:"url"`
	PublishedAt   string `json:"publishedAt,omitempty"`
}

type State struct {
	Status         string    `json:"status"`
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion,omitempty"`
	URL            string    `json:"url,omitempty"`
	CheckedAt      time.Time `json:"checkedAt,omitempty"`
}

type Checker struct {
	mu       sync.RWMutex
	current  string
	source   string
	client   *http.Client
	now      func() time.Time
	state    State
	checking bool
}

func New(current, source string, client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: safeRedirect}
	}
	return &Checker{current: current, source: source, client: client, now: time.Now,
		state: State{Status: "idle", CurrentVersion: current}}
}

func (c *Checker) Snapshot() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Check coalesces concurrent callers. A failed check stays silent to users and
// is represented only by the fixed unavailable status; raw network errors are
// not retained.
func (c *Checker) Check(ctx context.Context) State {
	c.mu.Lock()
	if c.checking {
		state := c.state
		c.mu.Unlock()
		return state
	}
	c.checking = true
	c.state = State{Status: "checking", CurrentVersion: c.current}
	c.mu.Unlock()

	state := State{Status: "unavailable", CurrentVersion: c.current, CheckedAt: c.now().UTC()}
	if manifest, err := fetch(ctx, c.client, c.source); err == nil {
		state.LatestVersion, state.URL = manifest.Version, manifest.URL
		state.Status = "current"
		if newer(manifest.Version, c.current) {
			state.Status = "available"
		}
	}
	c.mu.Lock()
	c.state, c.checking = state, false
	c.mu.Unlock()
	return state
}

func fetch(ctx context.Context, client *http.Client, source string) (Manifest, error) {
	if err := validateSource(source); err != nil {
		return Manifest{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return Manifest{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LeaguePingo-UpdateCheck/1")
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("manifest status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxManifestBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > maxManifestBytes {
		return Manifest{}, errors.New("invalid manifest size")
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Manifest{}, errors.New("multiple manifest values")
	}
	if manifest.SchemaVersion != 1 || parseVersion(manifest.Version) == nil {
		return Manifest{}, errors.New("invalid manifest")
	}
	download, err := url.Parse(manifest.URL)
	if err != nil || download.Scheme != "https" || !strings.EqualFold(download.Host, "github.com") ||
		download.User != nil || download.RawQuery != "" || download.Fragment != "" ||
		!strings.HasPrefix(download.EscapedPath(), "/ZJing1125/LeaguePingo/releases/") {
		return Manifest{}, errors.New("invalid release url")
	}
	return manifest, nil
}

func validateSource(raw string) error {
	if raw != manifestSource {
		return errors.New("invalid manifest source")
	}
	return nil
}

func safeRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 || req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), "raw.githubusercontent.com") {
		return errors.New("update redirect refused")
	}
	return nil
}

type version struct {
	parts [3]uint64
	pre   []string
}

func parseVersion(raw string) *version {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
	pieces := strings.SplitN(raw, "+", 2)
	core := strings.SplitN(pieces[0], "-", 2)
	numbers := strings.Split(core[0], ".")
	if len(numbers) != 3 {
		return nil
	}
	v := &version{}
	for i, value := range numbers {
		if value == "" || len(value) > 1 && value[0] == '0' {
			return nil
		}
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil
		}
		v.parts[i] = n
	}
	if len(core) == 2 {
		if core[1] == "" {
			return nil
		}
		v.pre = strings.Split(core[1], ".")
		for _, value := range v.pre {
			if value == "" {
				return nil
			}
			for _, r := range value {
				if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '-') {
					return nil
				}
			}
			if len(value) > 1 && value[0] == '0' {
				if _, err := strconv.ParseUint(value, 10, 64); err == nil {
					return nil
				}
			}
		}
	}
	return v
}

func newer(candidate, current string) bool {
	a, b := parseVersion(candidate), parseVersion(current)
	if a == nil || b == nil {
		return false
	}
	for i := range a.parts {
		if a.parts[i] != b.parts[i] {
			return a.parts[i] > b.parts[i]
		}
	}
	if len(a.pre) == 0 || len(b.pre) == 0 {
		return len(a.pre) == 0 && len(b.pre) > 0
	}
	for i := 0; i < max(len(a.pre), len(b.pre)); i++ {
		if i >= len(a.pre) || i >= len(b.pre) {
			return i >= len(b.pre)
		}
		if a.pre[i] == b.pre[i] {
			continue
		}
		an, ae := strconv.ParseUint(a.pre[i], 10, 64)
		bn, be := strconv.ParseUint(b.pre[i], 10, 64)
		switch {
		case ae == nil && be == nil:
			return an > bn
		case ae == nil:
			return false
		case be == nil:
			return true
		default:
			return a.pre[i] > b.pre[i]
		}
	}
	return false
}
