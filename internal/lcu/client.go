package lcu

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/diagnostics"
)

var ErrNotFound = apperrors.New(apperrors.ClientNotRunning)

type Credentials struct {
	Port     int
	Password string
	Region   string
}

type Client struct {
	HTTP *http.Client
	Cred Credentials
}

func ParseLockfile(raw string) (Credentials, error) {
	if strings.TrimSpace(raw) == "" {
		return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 || parts[0] != "LeagueClient" || parts[4] != "https" {
		return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	if pid, err := strconv.Atoi(parts[1]); err != nil || pid <= 0 {
		return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil || port < 1 || port > 65535 || parts[3] == "" {
		return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	return Credentials{Port: port, Password: parts[3]}, nil
}

func DiscoverFromPaths(paths []string) (*Client, error) {
	var lastErr error
	for _, path := range paths {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				lastErr = lockfileReadError(err)
			}
			continue
		}
		if info.IsDir() {
			lastErr = apperrors.New(apperrors.ClientConfigurationInvalid)
			continue
		}
		contents, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			lastErr = lockfileReadError(err)
			continue
		}
		cred, err := ParseLockfile(string(contents))
		if err != nil {
			lastErr = err
			continue
		}
		return NewClient(cred), nil
	}
	if lastErr != nil {
		return nil, errors.Join(lastErr, ErrNotFound)
	}
	return nil, ErrNotFound
}

func lockfileReadError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return apperrors.New(apperrors.ClientPermissionDenied)
	}
	return apperrors.New(apperrors.ClientConfigurationInvalid)
}

// ParseConnectionArgs accepts only LeagueClient/LeagueClientUx's LCU flags.
// riotclient-app-port and riotclient-auth-token belong to a different service.
// Never include the input arguments or token in returned errors.
func ParseConnectionArgs(args []string) (Credentials, error) {
	values := make(map[string]string)
	for i := 0; i < len(args); i++ {
		key, value, hasValue := strings.Cut(args[i], "=")
		if key != "--app-port" && key != "--remoting-auth-token" && key != "--region" {
			continue
		}
		if _, exists := values[key]; exists {
			return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
		}
		if !hasValue {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
			}
			i++
			value = args[i]
		}
		values[key] = value
	}
	port, err := strconv.Atoi(values["--app-port"])
	token := values["--remoting-auth-token"]
	if err != nil || port < 1 || port > 65535 || token == "" || strings.ContainsAny(token, "\r\n\x00") {
		return Credentials{}, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	return Credentials{Port: port, Password: token, Region: values["--region"]}, nil
}

func NewClient(cred Credentials) *Client {
	return &Client{Cred: cred, HTTP: &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:             nil,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // LCU uses a local self-signed certificate.
			DisableKeepAlives: false,
		},
	}}
}

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.request(ctx, http.MethodGet, path, nil)
}

// LookupSummonerAlias is a read-only identity lookup despite its POST method.
// Keep the endpoint fixed: this is not a general client mutation API.
func (c *Client) LookupSummonerAlias(ctx context.Context, name, tag string) ([]byte, error) {
	body, err := json.Marshal([]struct {
		GameName string `json:"gameName"`
		TagLine  string `json:"tagLine"`
	}{{GameName: name, TagLine: tag}})
	if err != nil {
		return nil, apperrors.New(apperrors.ClientConfigurationInvalid)
	}
	return c.request(ctx, http.MethodPost, "/lol-summoner/v1/summoners/aliases", body)
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) (data []byte, resultErr error) {
	started, status := time.Now(), 0
	defer func() {
		diagnostics.Default.Record(path, time.Since(started), status, resultErr != nil)
		apperrors.Failures.Record(apperrors.LocalClient, resultErr)
	}()
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, apperrors.New(apperrors.InvalidResponse)
	}
	endpoint := fmt.Sprintf("https://127.0.0.1:%d%s", c.Cred.Port, path)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.New(apperrors.InvalidResponse)
	}
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("riot:"+c.Cred.Password)))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, apperrors.Request(ctx, err, apperrors.LocalClient)
	}
	defer response.Body.Close()
	status = response.StatusCode
	if response.StatusCode != http.StatusOK && !(method == http.MethodPost && (path == readyCheckPath+"/accept" || path == playAgainPath || path == startSearchPath || path == submitHonorBallotPath) && response.StatusCode == http.StatusNoContent) {
		return nil, apperrors.HTTP(apperrors.LocalClient, response.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, apperrors.Normalize(err, apperrors.InvalidResponse)
	}
	return data, nil
}

func HistoryPath(puuid string, count int) string {
	return HistoryPagePath(puuid, 0, count)
}

func HistoryPagePath(puuid string, offset, count int) string {
	if count < 1 {
		count = 1
	}
	if count > 20 {
		count = 20
	}
	return fmt.Sprintf("/lol-match-history/v1/products/lol/%s/matches?begIndex=%d&endIndex=%d", url.PathEscape(puuid), offset, offset+count-1)
}
