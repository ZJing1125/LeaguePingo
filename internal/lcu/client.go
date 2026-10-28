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
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"leaguepingo/internal/diagnostics"
)

var ErrNotFound = errors.New("League 客户端未连接")

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
		return Credentials{}, errors.New("客户端 lockfile 为空")
	}
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 || parts[0] != "LeagueClient" || parts[4] != "https" {
		return Credentials{}, errors.New("客户端 lockfile 格式无效")
	}
	if pid, err := strconv.Atoi(parts[1]); err != nil || pid <= 0 {
		return Credentials{}, errors.New("客户端进程 ID 无效")
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil || port < 1 || port > 65535 || parts[3] == "" {
		return Credentials{}, errors.New("客户端连接信息无效")
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
			lastErr = errors.New("客户端 lockfile 路径指向目录")
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
		return nil, fmt.Errorf("%w：%w", ErrNotFound, lastErr)
	}
	return nil, ErrNotFound
}

func lockfileReadError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return errors.New("没有权限读取客户端 lockfile")
	}
	return errors.New("无法读取客户端 lockfile")
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
			return Credentials{}, errors.New("客户端连接启动参数重复")
		}
		if !hasValue {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return Credentials{}, errors.New("客户端连接启动参数缺少值")
			}
			i++
			value = args[i]
		}
		values[key] = value
	}
	port, err := strconv.Atoi(values["--app-port"])
	token := values["--remoting-auth-token"]
	if err != nil || port < 1 || port > 65535 || token == "" || strings.ContainsAny(token, "\r\n\x00") {
		return Credentials{}, errors.New("客户端启动参数中没有有效的 LCU 端口或鉴权信息")
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
		return nil, errors.New("客户端查询参数编码失败")
	}
	return c.request(ctx, http.MethodPost, "/lol-summoner/v1/summoners/aliases", body)
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) (data []byte, resultErr error) {
	started, status := time.Now(), 0
	defer func() { diagnostics.Default.Record(path, time.Since(started), status, resultErr != nil) }()
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, errors.New("invalid LCU path")
	}
	endpoint := fmt.Sprintf("https://127.0.0.1:%d%s", c.Cred.Port, path)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("客户端请求构造失败")
	}
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("riot:"+c.Cred.Password)))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, safeRequestError(ctx, err, "客户端连接失败，请确认客户端仍在运行后重试")
	}
	defer response.Body.Close()
	status = response.StatusCode
	if response.StatusCode != http.StatusOK && !(method == http.MethodPost && (path == readyCheckPath+"/accept" || path == playAgainPath || path == startSearchPath) && response.StatusCode == http.StatusNoContent) {
		return nil, fmt.Errorf("LCU HTTP %d", response.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, safeRequestError(ctx, err, "客户端响应读取失败，请稍后重试")
	}
	return data, nil
}

// Preserve only safe cancellation classifications. The raw net/url error can
// contain a complete query URL and player identity, so never return or wrap it.
type requestError struct {
	message string
	cause   error
}

func (e *requestError) Error() string { return e.message }
func (e *requestError) Unwrap() error { return e.cause }

func safeRequestError(ctx context.Context, err error, fallback string) error {
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		return &requestError{message: "客户端请求已取消", cause: context.Canceled}
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return &requestError{message: "客户端响应超时，请稍后重试", cause: context.DeadlineExceeded}
	}
	return errors.New(fallback)
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
