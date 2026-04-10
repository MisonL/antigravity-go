package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mison/antigravity-go/internal/rpc"
)

func (h *Host) SetOnRestart(cb func(RestartInfo) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.OnRestart = cb
}

func (h *Host) RestartCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.restartCount
}

func (h *Host) startProcess() error {
	h.mu.Lock()
	h.resetRuntimeLocked()
	h.generation++
	generation := h.generation
	h.mu.Unlock()

	cmd := exec.CommandContext(h.ctx, h.binPath,
		"--enable_lsp",
		"--gemini_dir", h.dataDir,
		"--app_data_dir", ".",
		"--http_server_port=0",
		"--logtostderr=true",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	logPath := filepath.Join(h.dataDir, "core.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start core: %w", err)
	}
	if err := logFile.Close(); err != nil {
		return fmt.Errorf("close log file handle: %w", err)
	}

	done := make(chan struct{})
	go h.tailLogs(logPath, done)
	go h.waitForProcessExit(cmd, generation, done)

	h.mu.Lock()
	h.cmd = cmd
	h.stdin = stdin
	h.processDone = done
	h.mu.Unlock()

	metadata := generateMetadata()
	if _, err := stdin.Write(metadata); err != nil {
		_ = stopProcess(cmd, done)
		return fmt.Errorf("write metadata: %w", err)
	}
	if err := stdin.Close(); err != nil {
		_ = stopProcess(cmd, done)
		return fmt.Errorf("close metadata pipe: %w", err)
	}

	h.emitHostLog("host: core process started")
	return nil
}

func (h *Host) startWatchdog() {
	h.watchdogOnce.Do(func() {
		go h.watchdogLoop()
	})
}

func (h *Host) watchdogLoop() {
	ticker := time.NewTicker(h.watchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			if h.shouldRestart() {
				if err := h.restartProcess(); err != nil {
					h.emitHostLog(fmt.Sprintf("host: watchdog restart failed: %v", err))
				}
			}
		}
	}
}

func (h *Host) shouldRestart() bool {
	h.mu.RLock()
	if h.restarting {
		h.mu.RUnlock()
		return false
	}
	cmd := h.cmd
	port := h.httpPort
	exited := h.processExited
	failures := h.heartbeatFailures
	limit := h.healthBudget
	h.mu.RUnlock()

	if cmd == nil || exited {
		return true
	}
	if port == 0 {
		return false
	}

	client := rpcClientFactory(port)
	if err := client.Heartbeat(); err != nil {
		h.mu.Lock()
		h.heartbeatFailures++
		failures = h.heartbeatFailures
		h.ready = false
		h.mu.Unlock()
		h.emitHostLog(fmt.Sprintf("host: heartbeat failed (%d/%d): %v", failures, limit, err))
		return failures >= limit
	}

	h.mu.Lock()
	h.heartbeatFailures = 0
	h.ready = true
	h.mu.Unlock()
	return false
}

func (h *Host) restartProcess() error {
	h.restartMu.Lock()
	defer h.restartMu.Unlock()

	if h.ctx.Err() != nil {
		return h.ctx.Err()
	}

	h.mu.Lock()
	if h.restarting {
		h.mu.Unlock()
		return nil
	}
	h.restarting = true
	current := h.cmd
	h.ready = false
	h.mu.Unlock()

	h.emitHostLog("host: starting self-heal restart")

	if current != nil {
		if err := h.stopCurrentProcess(); err != nil {
			h.emitHostLog(fmt.Sprintf("host: graceful stop failed, forcing exit: %v", err))
		}
	}

	if err := h.startProcess(); err != nil {
		h.mu.Lock()
		h.restarting = false
		h.mu.Unlock()
		return err
	}

	if err := h.WaitReady(h.restartReadyTimeout); err != nil {
		h.mu.Lock()
		h.restarting = false
		h.mu.Unlock()
		return err
	}

	h.mu.Lock()
	h.heartbeatFailures = 0
	h.restartCount++
	info := RestartInfo{
		HTTPPort:     h.httpPort,
		RestartCount: h.restartCount,
	}
	callback := h.OnRestart
	h.restarting = false
	h.mu.Unlock()

	if callback != nil {
		if err := callback(info); err != nil {
			h.emitHostLog(fmt.Sprintf("host: restart callback failed: %v", err))
		}
	}

	h.emitHostLog(fmt.Sprintf("host: self-heal restart complete on port %d", info.HTTPPort))
	return nil
}

func (h *Host) stopCurrentProcess() error {
	h.mu.RLock()
	cmd := h.cmd
	done := h.processDone
	h.mu.RUnlock()
	return stopProcess(cmd, done)
}

func (h *Host) waitForProcessExit(cmd *exec.Cmd, generation uint64, done chan struct{}) {
	err := cmd.Wait()
	close(done)

	h.mu.Lock()
	if h.generation != generation {
		h.mu.Unlock()
		return
	}
	h.processExited = true
	h.ready = false
	onLog := h.OnLog
	if err != nil && h.ctx.Err() == nil {
		line := "host: core process exited: " + err.Error()
		h.logs = append(h.logs, line)
		if len(h.logs) > 1000 {
			h.logs = h.logs[len(h.logs)-1000:]
		}
		h.mu.Unlock()
		if onLog != nil {
			onLog(line)
		}
		return
	}
	h.mu.Unlock()
}

func (h *Host) resetRuntimeLocked() {
	h.cmd = nil
	h.stdin = nil
	h.stderr = nil
	h.stdout = nil
	h.processDone = nil
	h.httpPort = 0
	h.httpsPort = 0
	h.lspPort = 0
	h.ready = false
	h.processExited = false
	h.heartbeatFailures = 0
	h.readyChan = make(chan struct{})
	h.portReadyChan = make(chan struct{})
}

func (h *Host) emitHostLog(line string) {
	h.mu.Lock()
	h.logs = append(h.logs, line)
	if len(h.logs) > 1000 {
		h.logs = h.logs[len(h.logs)-1000:]
	}
	onLog := h.OnLog
	h.mu.Unlock()

	if onLog != nil {
		onLog(line)
	}
}

func stopProcess(cmd *exec.Cmd, done <-chan struct{}) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && err.Error() != "os: process already finished" {
		if killErr := cmd.Process.Kill(); killErr != nil && killErr.Error() != "os: process already finished" {
			return killErr
		}
		return nil
	}

	if done == nil {
		done = make(chan struct{})
	}

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		if err := cmd.Process.Kill(); err != nil && err.Error() != "os: process already finished" {
			return err
		}
		return nil
	}
}

var rpcClientFactory = func(port int) heartbeatClient {
	return newHeartbeatClient(port)
}

type heartbeatClient interface {
	Heartbeat() error
}

func newHeartbeatClient(port int) heartbeatClient {
	return rpc.NewClient(port)
}
