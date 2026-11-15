//go:build windows

package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Must happen before anything creates a window, otherwise Windows scales the
	// whole panel bitmap on a HiDPI screen and the text goes soft.
	enableDPIAwareness()
	setupLogging()
	log.Println("面板启动")

	cfgPath := flag.String("config", "", "配置文件路径（默认 exe 同目录 panel.json）")
	panelPort := flag.Int("panel-port", 0, "面板端口，覆盖配置")
	servicePort := flag.Int("service-port", 0, "后端服务端口，覆盖配置")
	noService := flag.Bool("no-service", false, "不自动启动后端服务")
	noWindow := flag.Bool("no-window", false, "不开窗口，只跑面板服务（调试用）")
	flag.Parse()

	cfg, path, err := LoadConfig(*cfgPath)
	if err != nil {
		fail("配置读取失败", err.Error())
	}
	if *panelPort > 0 {
		cfg.PanelPort = *panelPort
	}
	if *servicePort > 0 && *servicePort != cfg.ServicePort {
		// start.bat passes PORT; keep the backend in step with it.
		cfg.ServicePort = *servicePort
		if strings.HasPrefix(cfg.Backend, "http://127.0.0.1:") || strings.HasPrefix(cfg.Backend, "http://localhost:") {
			cfg.Backend = "http://127.0.0.1:" + strconv.Itoa(*servicePort)
		}
	}
	cfg.normalize()

	// Materialise the defaults so the settings are discoverable in a text editor.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		if err := SaveConfig(path, cfg); err != nil {
			log.Printf("写入默认配置失败：%v", err)
		}
	}

	// 已经有一个实例在跑：把它的窗口叫到前面，本进程直接退出。
	if infoURL := "http://127.0.0.1:" + strconv.Itoa(cfg.PanelPort) + "/panel/info"; httpAlive(infoURL) {
		log.Println("已有实例在运行，唤起窗口")
		postNoBody("http://127.0.0.1:" + strconv.Itoa(cfg.PanelPort) + "/panel/focus")
		return
	}

	svc := newService(cfg)
	srv := newPanelServer(cfg, path, svc)
	if err := srv.start(); err != nil {
		fail("面板启动失败", err.Error())
	}
	log.Printf("面板地址 %s（配置 %s）", srv.url(), path)

	if *noWindow {
		if cfg.AutoStart && cfg.localBackend() && !*noService {
			go func() {
				if err := svc.Start(); err != nil {
					log.Printf("启动服务失败：%v", err)
				}
			}()
		}
		log.Println("无窗口模式，Ctrl+C 结束")
		select {}
	}

	app, err := newPanelApp(cfg, srv, svc)
	if err != nil {
		fail("窗口创建失败", err.Error())
	}
	srv.onFocus = app.focusFromHTTP
	srv.onExit = app.quit

	if cfg.AutoStart && cfg.localBackend() && !*noService {
		go func() {
			if err := svc.Start(); err != nil {
				log.Printf("自动启动服务：%v", err)
			}
		}()
	}

	app.wv.Navigate(srv.url())
	app.wv.Run()

	// 走到这里说明窗口循环已结束（退出菜单或 /panel/exit）。
	log.Println("面板退出，收尾中")
	svc.StopOnExit()
	app.removeTray()
	app.destroyIcons()
	log.Println("已退出")
}

// setupLogging writes to a file because a GUI build has no console.
func setupLogging() {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "MetaTubePanel")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "panel.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.SetOutput(io.Discard)
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags)
}

func httpAlive(url string) bool {
	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func postNoBody(url string) {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Post(url, "application/json", nil)
	if err == nil {
		_ = resp.Body.Close()
	}
}
