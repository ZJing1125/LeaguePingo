package lcu

import (
	"context"
	"crypto/tls"
	"encoding/base64"
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
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 || parts[0] != "LeagueClient" || parts[4] != "https" {
		return Credentials{}, errors.New("客户端 lockfile 格式无效")
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return Credentials{}, errors.New("客户端进程 ID 无效")
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil || port < 1 || port > 65535 || parts[3] == "" {
		return Credentials{}, errors.New("客户端连接信息无效")
	}
	return Credentials{Port: port, Password: parts[3]}, nil
}

func DiscoverFromPaths(paths []string) (*Client, error) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		contents, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			continue
		}
		cred, err := ParseLockfile(string(contents))
		if err != nil {
			continue
		}
		return NewClient(cred), nil
	}
	return nil, ErrNotFound
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
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, errors.New("invalid LCU path")
	}
	endpoint := fmt.Sprintf("https://127.0.0.1:%d%s", c.Cred.Port, path)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("riot:"+c.Cred.Password)))
	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LCU HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func HistoryPath(puuid string, count int) string {
	if count < 1 {
		count = 1
	}
	if count > 20 {
		count = 20
	}
	return fmt.Sprintf("/lol-match-history/v1/products/lol/%s/matches?begIndex=0&endIndex=%d", url.PathEscape(puuid), count-1)
}
