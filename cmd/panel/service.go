//go:build windows

package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Service states reported to the panel UI.
const (
	stateStopped  = "stopped"
	stateStarting = "starting"
	stateRunning  = "running"
	stateExternal = "external" // a listener we did not start (e.g. the console server)
)

// service supervises the local metatube-server process.
type service struct {
	mu      sync.Mutex
	cfg     *Config
	cfgPath string
	cmd     *exec.Cmd
	state   string
	pid     int
	since   time.Time
	last    string
	logger  *os.File

	// stopping marks an intentional kill so the wait goroutine does not report
	// the resulting non-zero exit as a failure.
	stopping bool
}

func newService(cfg *Config, cfgPath string) *service {
	s := &service{cfg: cfg, cfgPath: cfgPath, state: stateStopped}
	if portOpen(cfg.ServicePort) {
		s.state = stateExternal
	}
	return s
}

// portOpen reports whether something is already listening on the loopback port.
func portOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *service) logPath() string {
	return filepath.Join(exeDir(), "metatube-service.log")
}

// Start launches the server and waits for its port to accept connections.
func (s *service) Start() error {
	s.mu.Lock()
	if s.state == stateRunning || s.state == stateStarting {
		s.mu.Unlock()
		return errors.New("服务已在运行")
	}
	exe, port := s.cfg.serviceExePath(), s.cfg.ServicePort
	s.mu.Unlock()

	if _, err := os.Stat(exe); err != nil {
		// 配置里的相对路径是相对**面板 exe 所在目录**拼的，手写时很容易多写一层
		// （例如写成 "metatube-win\metatube-server.exe"，结果拼成
		// metatube-win\metatube-win\metatube-server.exe）。面板旁边就有同名文件时
		// 直接用它，并把纠正后的路径写回配置，省得下次再踩。
		alt := filepath.Join(exeDir(), filepath.Base(exe))
		if _, altErr := os.Stat(alt); altErr != nil {
			return fmt.Errorf("找不到服务端程序：%s", exe)
		}
		log.Printf("service_exe 指向的 %s 不存在，改用同目录的 %s", exe, alt)
		exe = alt
		s.mu.Lock()
		s.cfg.ServiceExe = alt
		cfgPath := s.cfgPath
		s.mu.Unlock()
		if cfgPath != "" {
			if err := SaveConfig(cfgPath, s.cfg); err != nil {
				log.Printf("写回 panel.json 失败：%v", err)
			}
		}
	}
	if portOpen(port) {
		s.mu.Lock()
		s.state = stateExternal
		s.pid = 0
		s.mu.Unlock()
		return fmt.Errorf("端口 %d 已被占用，可能已有实例在跑", port)
	}

	cmd := exec.Command(exe, s.cfg.serviceArgs()...)
	cmd.Dir = filepath.Dir(exe)
	// The server expects data/ to exist; create it so a fresh copy has somewhere
	// to put the database.
	_ = os.MkdirAll(filepath.Join(cmd.Dir, "data"), 0o755)
	// Keep the server's own log so a failed start can be explained.
	if f, err := os.OpenFile(s.logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		cmd.Stdout, cmd.Stderr = f, f
		s.mu.Lock()
		s.logger = f
		s.mu.Unlock()
	}
	// No console window for the child; it is a background service.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动失败：%w", err)
	}

	s.mu.Lock()
	s.cmd = cmd
	s.state = stateStarting
	s.pid = cmd.Process.Pid
	s.since = time.Now()
	s.last = ""
	s.mu.Unlock()

	go func(c *exec.Cmd) {
		err := c.Wait()
		s.mu.Lock()
		if s.cmd == c {
			intentional := s.stopping
			s.cmd = nil
			s.state = stateStopped
			s.pid = 0
			s.stopping = false
			if err != nil && !intentional {
				s.last = "服务进程已退出：" + err.Error()
			}
			if s.logger != nil {
				_ = s.logger.Close()
				s.logger = nil
			}
		}
		s.mu.Unlock()
	}(cmd)

	// The server opens the port in a second or two; give it up to 25s before
	// concluding something went wrong.
	for i := 0; i < 50; i++ {
		if portOpen(port) {
			s.mu.Lock()
			s.state = stateRunning
			s.mu.Unlock()
			return nil
		}
		s.mu.Lock()
		gone := s.cmd == nil
		s.mu.Unlock()
		if gone {
			return fmt.Errorf("服务启动后立即退出，详见 %s", filepath.Base(s.logPath()))
		}
		time.Sleep(500 * time.Millisecond)
	}

	s.mu.Lock()
	s.state = stateRunning
	s.last = "启动超时：端口一直没起来"
	s.mu.Unlock()
	return errors.New("服务启动超时（25 秒）")
}

// Stop terminates the child process, never anything we did not start.
func (s *service) Stop() error {
	s.mu.Lock()
	cmd, port := s.cmd, s.cfg.ServicePort
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		if portOpen(port) {
			return errors.New("服务不是由面板启动的，无法结束；请手动关闭那个实例")
		}
		s.mu.Lock()
		s.state = stateStopped
		s.pid = 0
		s.mu.Unlock()
		return nil
	}

	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	_ = cmd.Process.Kill()
	for i := 0; i < 25; i++ {
		if !portOpen(port) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	s.mu.Lock()
	s.cmd = nil
	s.state = stateStopped
	s.pid = 0
	s.last = ""
	if s.logger != nil {
		_ = s.logger.Close()
		s.logger = nil
	}
	s.mu.Unlock()
	return nil
}

func (s *service) Restart() error {
	if err := s.Stop(); err != nil {
		return err
	}
	time.Sleep(400 * time.Millisecond)
	return s.Start()
}

// Status is what the panel UI polls.
func (s *service) Status() map[string]any {
	s.mu.Lock()
	state, pid, last := s.state, s.pid, s.last
	since := s.since
	s.mu.Unlock()

	// Reconcile with reality: the port is the source of truth.
	listening := portOpen(s.cfg.ServicePort)
	switch {
	case listening && state == stateStopped:
		state, pid = stateExternal, 0
	case !listening && (state == stateRunning || state == stateExternal):
		state, pid = stateStopped, 0
	}

	out := map[string]any{
		"state": state,
		"pid":   pid,
		"port":  s.cfg.ServicePort,
		"exe":   s.cfg.serviceExePath(),
		"error": last,
	}
	if !since.IsZero() && (state == stateRunning || state == stateStarting) {
		out["uptime_seconds"] = int64(time.Since(since).Seconds())
	}
	return out
}

// StopAsync is used on exit, where blocking is acceptable but errors are not.
func (s *service) StopOnExit() {
	_ = s.Stop()
}
