//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the panel's own settings, stored next to the executable (or in the
// user config directory when that folder is read-only, e.g. a share).
type Config struct {
	// Backend is the MetaTube server the panel connects to, e.g.
	// http://127.0.0.1:8080. All /v1 traffic is proxied there.
	Backend string `json:"backend"`
	// Token is injected as a bearer token on proxied requests. Empty disables
	// authentication for the connection.
	Token string `json:"token"`
	// ServiceExe is the server binary the panel supervises.
	ServiceExe string `json:"service_exe"`
	// ServicePort is passed to the service as --port.
	ServicePort int `json:"service_port"`
	// PanelPort is where the panel itself serves its page and control API.
	PanelPort int `json:"panel_port"`
	// AutoStart starts the service right after the panel opens.
	AutoStart bool `json:"auto_start"`
	// ServiceArgs are extra arguments for the server, appended after --port.
	// Empty means the launcher defaults (data/metatube.db, 60s timeout).
	ServiceArgs []string `json:"service_args,omitempty"`
}

// defaultPanelPort matches the port the shipped panel.json under
// C:/Script/metatube/metatube-win uses, so a fresh install behaves like
// the existing one. It sits inside this machine's dynamic port range, which
// means Hyper-V / WinNAT can reserve it after a reboot; start() then walks
// forward to the next bindable port and persists it.
const defaultPanelPort = 8081

func defaultConfig() *Config {
	return &Config{
		Backend:     "http://127.0.0.1:8080",
		ServicePort: 8080,
		PanelPort:   defaultPanelPort,
		AutoStart:   true,
	}
}

// exeDir is the directory holding the panel executable.
func exeDir() string {
	p, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(p)
}

// configPath picks panel.json next to the exe, falling back to the user config
// directory when the exe folder is not writable (or already has no config there).
func configPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	primary := filepath.Join(exeDir(), "panel.json")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	if dir, err := os.UserConfigDir(); err == nil {
		alt := filepath.Join(dir, "MetaTubePanel", "panel.json")
		if _, err := os.Stat(alt); err == nil {
			return alt
		}
	}
	return primary
}

func LoadConfig(explicit string) (*Config, string, error) {
	cfg := defaultConfig()
	path := configPath(explicit)

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, path, fmt.Errorf("解析 %s 失败：%w", path, err)
		}
	case !os.IsNotExist(err):
		return nil, path, err
	}

	cfg.normalize()
	return cfg, path, nil
}

func (c *Config) normalize() {
	if c.PanelPort <= 0 || c.PanelPort > 65535 {
		c.PanelPort = defaultPanelPort
	}
	if c.ServicePort <= 0 || c.ServicePort > 65535 {
		c.ServicePort = 8080
	}
	c.Backend = strings.TrimRight(strings.TrimSpace(c.Backend), "/")
	if c.Backend == "" {
		c.Backend = fmt.Sprintf("http://127.0.0.1:%d", c.ServicePort)
	}
}

// serviceExePath resolves the server binary to launch. Empty means "the
// metatube-server.exe next to the panel"; relative values are also taken
// relative to the panel's own directory. Keeping the raw ServiceExe untouched
// means panel.json stays portable (an empty value keeps working after the whole
// folder is moved).
func (c *Config) serviceExePath() string {
	if c.ServiceExe == "" {
		return filepath.Join(exeDir(), "metatube-server.exe")
	}
	if !filepath.IsAbs(c.ServiceExe) {
		return filepath.Join(exeDir(), c.ServiceExe)
	}
	return c.ServiceExe
}

// localBackend reports whether the configured backend is the service this panel
// supervises (loopback + matching port). Only then does starting it, or passing
// it the token, make sense.
func (c *Config) localBackend() bool {
	u, err := url.Parse(c.Backend)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1", "[::1]":
	default:
		return false
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	return port == strconv.Itoa(c.ServicePort)
}

// serviceArgs keeps the console defaults (data/metatube.db), so the database
// path does not change depending on who launched the server.
func (c *Config) serviceArgs() []string {
	args := []string{"--port", strconv.Itoa(c.ServicePort)}
	if len(c.ServiceArgs) > 0 {
		args = append(args, c.ServiceArgs...)
	} else {
		args = append(args, "--dsn", "data/metatube.db", "--request-timeout", "60s")
	}
	// The panel adds the token itself when proxying, so the local service should
	// require the same one.
	if c.Token != "" && c.localBackend() {
		args = append(args, "--token", c.Token)
	}
	return args
}

func SaveConfig(path string, c *Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
