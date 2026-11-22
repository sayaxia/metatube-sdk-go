//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	V "github.com/metatube-community/metatube-sdk-go/internal/version"
	"github.com/metatube-community/metatube-sdk-go/route/ui"
)

// panelServer serves the console page, the panel's control API, and proxies
// /v1 traffic to the configured backend. Serving the page ourselves is what
// makes "restart the service" possible: the UI must stay alive while the
// service is down.
type panelServer struct {
	mu      sync.Mutex
	cfg     *Config
	cfgPath string
	svc     *service
	http    *http.Server

	backend atomic.Value // *url.URL
	token   atomic.Value // string

	onFocus func()
	onExit  func()
}

func newPanelServer(cfg *Config, cfgPath string, svc *service) *panelServer {
	p := &panelServer{cfg: cfg, cfgPath: cfgPath, svc: svc}
	p.token.Store(cfg.Token)
	p.reloadBackend()
	return p
}

func (p *panelServer) reloadBackend() error {
	raw := p.cfg.Backend
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("后端地址无效：%s", raw)
	}
	p.backend.Store(u)
	return nil
}

func (p *panelServer) backendURL() *url.URL {
	if v := p.backend.Load(); v != nil {
		return v.(*url.URL)
	}
	return nil
}

// start binds the panel port and serves until Shutdown is called.
//
// A port can be unbindable on Windows even when nothing is listening on it:
// Hyper-V / WinNAT reserves whole ranges (see
// `netsh int ipv4 show excludedportrange protocol=tcp`) and binding inside one
// fails with WSAEACCES ("An attempt was made to access a socket in a way
// forbidden by its access permissions"). Those ranges are re-negotiated after
// reboots and can eat any port inside the dynamic range, so a port that worked
// yesterday may be taken today. Instead of refusing to start, walk forward to
// the next bindable port and persist it, so the next launch goes straight there.
func (p *panelServer) start() error {
	// Windows reservations come in contiguous blocks (100 ports is common on
	// machines where the dynamic range was narrowed to 1024-14999), so the
	// probe window has to be wide enough to step over a couple of them. A
	// refused bind returns immediately, so walking a few hundred ports costs
	// well under a millisecond.
	const probeLimit = 300

	want := p.cfg.PanelPort
	var lastErr error

	for port := want; port <= 65535 && port < want+probeLimit; port++ {
		// Never take the port the supervised service is configured to use.
		if port == p.cfg.ServicePort {
			continue
		}
		addr := net.JoinHostPort("127.0.0.1", fmt.Sprint(port))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			lastErr = err
			continue
		}
		if port != want {
			log.Printf("面板端口 %d 不可用（%v），改用 %d", want, lastErr, port)
			p.cfg.PanelPort = port
			if err := SaveConfig(p.cfgPath, p.cfg); err != nil {
				log.Printf("写回配置失败：%v", err)
			}
		}
		srv := &http.Server{Handler: p.routes(), ReadHeaderTimeout: 10 * time.Second}
		p.http = srv
		go func() { _ = srv.Serve(ln) }()
		return nil
	}

	return fmt.Errorf("面板端口 %d 及其后 %d 个端口都无法监听（最后一次错误：%v）", want, probeLimit, lastErr)
}

func (p *panelServer) url() string {
	return fmt.Sprintf("http://127.0.0.1:%d/", p.cfg.PanelPort)
}

func (p *panelServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleIndex)
	mux.HandleFunc("/panel/info", p.handleInfo)
	mux.HandleFunc("/panel/config", p.handleConfig)
	mux.HandleFunc("/panel/service/start", p.handleServiceAction("start"))
	mux.HandleFunc("/panel/service/stop", p.handleServiceAction("stop"))
	mux.HandleFunc("/panel/service/restart", p.handleServiceAction("restart"))
	mux.HandleFunc("/panel/exit", p.handleExit)
	mux.HandleFunc("/panel/focus", p.handleFocus)
	mux.Handle("/v1/", p.proxy())
	return mux
}

func (p *panelServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "没有这个地址")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(ui.Index)
}

func (p *panelServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"panel":     true,
		"version":   V.BuildString(),
		"backend":   p.cfg.Backend,
		"has_token": p.cfg.Token != "",
		"panel_url": p.url(),
		"config":    p.cfgPath,
		"service":   p.svc.Status(),
	})
}

// handleConfig persists the backend settings and applies them immediately.
func (p *panelServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"backend":   p.cfg.Backend,
			"has_token": p.cfg.Token != "",
			"service": map[string]any{
				"exe":  p.cfg.serviceExePath(),
				"port": p.cfg.ServicePort,
			},
		})
	case http.MethodPost:
		var body struct {
			Backend     string `json:"backend"`
			Token       string `json:"token"`
			TokenSet    bool   `json:"token_set"`
			ServicePort int    `json:"service_port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "请求内容无法解析")
			return
		}

		prevBackend, prevPort := p.cfg.Backend, p.cfg.ServicePort
		p.mu.Lock()
		if b := strings.TrimRight(strings.TrimSpace(body.Backend), "/"); b != "" {
			p.cfg.Backend = b
		}
		if body.TokenSet {
			p.cfg.Token = body.Token
		}
		if body.ServicePort > 0 && body.ServicePort <= 65535 {
			p.cfg.ServicePort = body.ServicePort
		}
		p.cfg.normalize()
		p.mu.Unlock()

		if err := p.reloadBackend(); err != nil {
			p.mu.Lock()
			p.cfg.Backend = prevBackend
			p.mu.Unlock()
			_ = p.reloadBackend()
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		p.token.Store(p.cfg.Token)

		if prevPort != p.cfg.ServicePort {
			// A running service on the old port no longer matches the config.
			_ = p.svc.Stop()
		}
		if err := SaveConfig(p.cfgPath, p.cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "保存配置失败："+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"backend":   p.cfg.Backend,
			"has_token": p.cfg.Token != "",
			"service":   p.svc.Status(),
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET / POST")
	}
}

func (p *panelServer) handleServiceAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
			return
		}
		var err error
		switch action {
		case "start":
			err = p.svc.Start()
		case "stop":
			err = p.svc.Stop()
		case "restart":
			err = p.svc.Restart()
		}
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"service": p.svc.Status()})
	}
}

func (p *panelServer) handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "只支持 POST")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	if p.onExit != nil {
		go p.onExit()
	}
}

func (p *panelServer) handleFocus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	if p.onFocus != nil {
		p.onFocus()
	}
}

// proxy forwards /v1 to the backend, injecting the configured token so the page
// never has to hold it.
func (p *panelServer) proxy() http.Handler {
	transport := &http.Transport{
		// Never route loopback/LAN traffic through an outbound proxy.
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	}

	rp := &httputil.ReverseProxy{
		Transport: transport,
		// Searches can legitimately take a minute; do not cut them off, and
		// stream image responses through without buffering the whole body.
		FlushInterval: 100 * time.Millisecond,
		Director: func(req *http.Request) {
			target := p.backendURL()
			if target == nil {
				return
			}
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = joinPath(target.Path, req.URL.Path)
			req.Host = target.Host
			if tok, _ := p.token.Load().(string); tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			target := p.backendURL()
			host := "后端"
			if target != nil {
				host = target.Host
			}
			writeError(w, http.StatusBadGateway, "连不上 "+host+"："+err.Error())
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.backendURL() == nil {
			writeError(w, http.StatusServiceUnavailable, "还没有设置后端地址，请点右上角设置")
			return
		}
		rp.ServeHTTP(w, r)
	})
}

func joinPath(base, path string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return path
	}
	return base + path
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError answers in the same envelope the service uses, so the page can
// display panel errors and service errors the same way.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}
