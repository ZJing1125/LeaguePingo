package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"leaguepingo/internal/postgame"
	"leaguepingo/internal/readycheck"
)

type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type Config struct {
	AutoReturnEnabled bool   `json:"autoReturnEnabled"`
	AutoQueueEnabled  bool   `json:"autoQueueEnabled"`
	AutoQueueDelay    int    `json:"autoQueueDelay"`
	AutoAcceptEnabled bool   `json:"autoAcceptEnabled"`
	AutoAcceptDelay   int    `json:"autoAcceptDelay"`
	Schema            int    `json:"schema"`
	Shortcut          string `json:"shortcut"`
	Pinned            bool   `json:"pinned"`
	HoldEnabled       bool   `json:"holdEnabled"`
	HoldKey           string `json:"holdKey"`
	Main              Rect   `json:"main"`
	Overlay           Rect   `json:"overlay"`
	SidebarEnabled    bool   `json:"sidebarEnabled"`
	PreviewEnabled    bool   `json:"previewEnabled"`
	SidebarWidth      int    `json:"sidebarWidth"`
}
type Store struct {
	mu     sync.Mutex
	path   string
	config Config
}

func Default() Config {
	return Config{Schema: 1, AutoQueueDelay: 10, AutoAcceptDelay: 1, Shortcut: "Ctrl+Shift+P", Pinned: true, HoldEnabled: true, HoldKey: "Backquote", SidebarEnabled: true, PreviewEnabled: true, SidebarWidth: 320, Main: Rect{Width: 1280, Height: 800}, Overlay: Rect{Width: 1100, Height: 640}}
}
func Open(path string) (*Store, error) {
	s := &Store{path: path, config: Default()}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, errors.New("无法读取本地设置，已使用默认值")
	}
	c := Default()
	if json.Unmarshal(b, &c) != nil || c.Schema != 1 {
		return s, errors.New("本地设置格式不兼容，已使用默认值")
	}
	key, err := NormalizeShortcut(c.Shortcut)
	if err != nil {
		return s, errors.New("本地快捷键无效，已使用默认值")
	}
	if !readycheck.ValidDelay(c.AutoAcceptDelay) {
		c.AutoAcceptDelay = 1
		c.AutoAcceptEnabled = false
	}
	if !postgame.ValidDelay(c.AutoQueueDelay) {
		c.AutoQueueDelay = 10
		c.AutoReturnEnabled = false
		c.AutoQueueEnabled = false
	}
	c.Shortcut = key
	c.SidebarWidth = max(280, min(c.SidebarWidth, 420))
	if !ValidHoldKey(c.HoldKey) {
		c.HoldKey = "Backquote"
		c.HoldEnabled = false
	}
	s.config = c
	return s, nil
}
func (s *Store) Snapshot() Config { s.mu.Lock(); defer s.mu.Unlock(); return s.config }
func (s *Store) SetBounds(main bool, r Rect) {
	if r.Width < 100 || r.Height < 100 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if main {
		s.config.Main = r
	} else {
		s.config.Overlay = r
	}
}
func (s *Store) SetPinned(p bool)       { s.mu.Lock(); s.config.Pinned = p; s.mu.Unlock() }
func (s *Store) SetShortcut(key string) { s.mu.Lock(); s.config.Shortcut = key; s.mu.Unlock() }
func ValidHoldKey(key string) bool {
	return key == "Backquote" || key == "F8" || key == "F9" || key == "F10"
}
func (s *Store) SetHold(enabled bool, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.HoldEnabled, s.config.HoldKey = enabled, key
}
func (s *Store) SetCompanion(sidebar, preview bool, width int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.SidebarEnabled, s.config.PreviewEnabled, s.config.SidebarWidth = sidebar, preview, max(280, min(width, 420))
}
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return errors.New("无法编码本地设置")
	}
	if os.MkdirAll(filepath.Dir(s.path), 0700) != nil {
		return errors.New("无法创建本地设置目录")
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), "settings-*.tmp")
	if err != nil {
		return errors.New("无法创建设置临时文件")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.New("无法保存本地设置")
	}
	if os.Rename(name, s.path) != nil {
		return errors.New("无法替换本地设置文件")
	}
	return nil
}
func NormalizeShortcut(key string) (string, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	if !regexp.MustCompile(`^CTRL\+SHIFT\+[A-Z]$`).MatchString(key) {
		return "", errors.New("快捷键仅支持 Ctrl+Shift+A 到 Ctrl+Shift+Z")
	}
	return "Ctrl+Shift+" + key[len(key)-1:], nil
}

// Keep the entire window inside a connected monitor work area. Negative monitor
// coordinates are valid; disconnected-screen positions fall back to the first.
func Clamp(r Rect, areas []Rect, minWidth, minHeight int) Rect {
	if len(areas) == 0 {
		return Rect{X: 80, Y: 80, Width: max(minWidth, min(r.Width, 1600)), Height: max(minHeight, min(r.Height, 1000))}
	}
	area := areas[0]
	for _, a := range areas {
		if r.X >= a.X && r.X < a.X+a.Width && r.Y >= a.Y && r.Y < a.Y+a.Height {
			area = a
			break
		}
	}
	r.Width = min(area.Width, max(minWidth, min(r.Width, 3000)))
	r.Height = min(area.Height, max(minHeight, min(r.Height, 2000)))
	r.X = max(area.X, min(r.X, area.X+area.Width-r.Width))
	r.Y = max(area.Y, min(r.Y, area.Y+area.Height-r.Height))
	return r
}

func (s *Store) SetAutoAccept(enabled bool, delay int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.AutoAcceptEnabled, s.config.AutoAcceptDelay = enabled, delay
}

func (s *Store) SetPostGame(ret, queue bool, delay int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.AutoReturnEnabled, s.config.AutoQueueEnabled, s.config.AutoQueueDelay = ret, queue, delay
}
