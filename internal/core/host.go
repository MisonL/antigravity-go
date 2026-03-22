// Package core provides the Core Host for managing antigravity_core lifecycle.
package core

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mison/antigravity-go/internal/index"
	"github.com/mison/antigravity-go/internal/rpc"
)

// Host manages the antigravity_core process lifecycle.
type Host struct {
	binPath string
	dataDir string
	indexer *index.Indexer

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr io.ReadCloser
	stdout io.ReadCloser

	mu        sync.RWMutex
	httpPort  int
	httpsPort int
	lspPort   int
	ready     bool
	logs      []string

	readyChan     chan struct{}
	portReadyChan chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	OnLog         func(string) // Callback for new log lines
	OnIndexStatus func(string) // Callback for indexing progress (start, complete, error)
}

// Config holds Host configuration.
type Config struct {
	BinPath string // Path to antigravity_core binary
	DataDir string // App data directory (--app_data_dir)
}

// NewHost creates a new Core Host instance.
func NewHost(cfg Config) *Host {
	ctx, cancel := context.WithCancel(context.Background())
	return &Host{
		binPath:       cfg.BinPath,
		dataDir:       cfg.DataDir,
		indexer:       index.NewIndexer("."), // Index current project root
		ctx:           ctx,
		cancel:        cancel,
		logs:          make([]string, 0, 1000),
		readyChan:     make(chan struct{}),
		portReadyChan: make(chan struct{}),
	}
}

func (h *Host) Indexer() *index.Indexer {
	return h.indexer
}

func (h *Host) startIndexing() {
	h.mu.Lock()
	onStatus := h.OnIndexStatus
	h.mu.Unlock()

	if onStatus != nil {
		onStatus("start")
	}

	fmt.Println("🔍 Indexing codebase...")
	if err := h.indexer.ScanProject(h.ctx); err != nil {
		fmt.Printf("⚠️ Indexing failed: %v\n", err)
		if onStatus != nil {
			onStatus("error: " + err.Error())
		}
	} else {
		summary := h.indexer.GetSummary()
		fmt.Printf("✅ Indexing complete: %s\n", summary)
		if onStatus != nil {
			onStatus("complete: " + summary)
		}
	}
}

// Start launches the antigravity_core process.
func (h *Host) Start() error {
	h.cmd = exec.CommandContext(h.ctx, h.binPath,
		"--enable_lsp",
		"--gemini_dir", h.dataDir, // Absolute path allowed here
		"--app_data_dir", ".", // Must be relative
		"--random_port=true", // Use random port to avoid conflicts
		"--logtostderr=true", // Force logs to stderr
	)

	// Re-implementing standard StdinPipe with explicit Close()
	var err error
	h.stdin, err = h.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	logPath := filepath.Join(h.dataDir, "core.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	h.cmd.Stdout = logFile
	h.cmd.Stderr = logFile

	if err := h.cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start core: %w", err)
	}

	// Start tailing
	go h.tailLogs(logPath)

	// Inject Protobuf metadata and CLOSE stdin to signal configuration is complete
	metadata := generateMetadata()
	if _, err := h.stdin.Write(metadata); err != nil {
		h.cmd.Process.Kill()
		return fmt.Errorf("write metadata: %w", err)
	}
	h.stdin.Close() // CRITICAL: Signal initialization end

	// Restore indexing
	go h.startIndexing()

	return nil
}

func generateMetadata() []byte {
	jsonBody := `{"version":"1.0.0"}`
	buf := new(bytes.Buffer)
	buf.WriteByte(0x0a)                // Tag: (1 << 3) | 2
	buf.WriteByte(byte(len(jsonBody))) // Length
	buf.WriteString(jsonBody)          // Value
	return buf.Bytes()
}

// tailLogs reads from the provided log file and parses port information.
func (h *Host) tailLogs(path string) {
	// Open the file for reading
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening log file for tailing: %v\n", err)
		return
	}
	defer file.Close()

	// Start from the beginning to catch all logs
	// file.Seek(0, io.SeekStart) is the default after Open

	// Create a buffered reader for efficient reading
	reader := bufio.NewReader(file)

	// Regex patterns for port detection
	httpsRe := regexp.MustCompile(`Language server listening on (?:fixed|random) port at (\d+) for HTTPS$`)
	httpRe := regexp.MustCompile(`Language server listening on (?:fixed|random) port at (\d+) for HTTP$`)
	lspRe := regexp.MustCompile(`LSP.*listening.*:(\d+)`)
	readyRe := regexp.MustCompile(`initialized server successfully`)

	for {
		select {
		case <-h.ctx.Done():
			return // Stop tailing if context is cancelled
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// Wait for a short period before trying to read again
					time.Sleep(100 * time.Millisecond)
					continue
				}
				fmt.Printf("Error reading from log file: %v\n", err)
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			h.mu.Lock()
			h.logs = append(h.logs, line)
			// Keep only last 1000 lines
			if len(h.logs) > 1000 {
				h.logs = h.logs[len(h.logs)-1000:]
			}
			onLog := h.OnLog
			h.mu.Unlock()

			// Trigger callback
			if onLog != nil {
				onLog(line)
			}

			// Parse port info
			if m := httpsRe.FindStringSubmatch(line); len(m) > 1 {
				if port, err := strconv.Atoi(m[1]); err == nil {
					h.mu.Lock()
					h.httpsPort = port
					h.mu.Unlock()
				}
			}
			if m := httpRe.FindStringSubmatch(line); len(m) > 1 {
				if port, err := strconv.Atoi(m[1]); err == nil {
					h.mu.Lock()
					h.httpPort = port
					h.mu.Unlock()
					select {
					case <-h.portReadyChan: // Check if already closed
					default:
						close(h.portReadyChan)
					}
				}
			}
			if m := lspRe.FindStringSubmatch(line); len(m) > 1 {
				if port, err := strconv.Atoi(m[1]); err == nil {
					h.mu.Lock()
					h.lspPort = port
					h.mu.Unlock()
				}
			}
			if readyRe.MatchString(line) {
				h.mu.Lock()
				h.ready = true
				h.mu.Unlock()
				select {
				case <-h.readyChan: // Check if already closed
				default:
					close(h.readyChan)
				}
			}
		}
	}
}

// WaitForPort blocks until the HTTP/HTTPS port is detected
func (h *Host) WaitForPort(timeout time.Duration) error {
	select {
	case <-h.portReadyChan:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for port")
	}
}

// WaitReady blocks until the core service is responsive.
func (h *Host) WaitReady(timeout time.Duration) error {
	// First, wait for the port to be discovered from logs
	if err := h.WaitForPort(timeout); err != nil {
		return err
	}

	h.mu.RLock()
	port := h.httpPort
	h.mu.RUnlock()

	// Active Probe: Try to connect to the port and send Heartbeat
	deadline := time.Now().Add(timeout)
	client := rpc.NewClient(port)

	for time.Now().Before(deadline) {
		// Step 1: Check TCP connectivity
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()

			// Step 2: RPC Heartbeat (Application Layer Readiness)
			if err := client.Heartbeat(); err == nil {
				h.mu.Lock()
				h.ready = true
				h.mu.Unlock()
				return nil
			}
		}

		// Wait before next probe
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout probing core RPC heartbeat at port %d", port)
}

// HTTPPort returns the HTTP port for Connect RPC.
func (h *Host) HTTPPort() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.httpPort
}

// HTTPSPort returns the HTTPS port.
func (h *Host) HTTPSPort() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.httpsPort
}

// LSPPort returns the LSP TCP port.
func (h *Host) LSPPort() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lspPort
}

// IsReady returns whether the core is ready.
func (h *Host) IsReady() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.ready
}

// Logs returns recent log lines.
func (h *Host) Logs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]string, len(h.logs))
	copy(result, h.logs)
	return result
}

// SetOnLog sets the log callback thread-safely.
func (h *Host) SetOnLog(cb func(string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.OnLog = cb
}

// Stop terminates the core process.
func (h *Host) Stop() error {
	h.cancel()
	if h.cmd != nil && h.cmd.Process != nil {
		return h.cmd.Process.Kill()
	}
	return nil
}

// Status returns a summary of the host status.
func (h *Host) Status() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Ready: %v\n", h.ready))
	sb.WriteString(fmt.Sprintf("HTTP Port: %d\n", h.httpPort))
	sb.WriteString(fmt.Sprintf("HTTPS Port: %d\n", h.httpsPort))
	sb.WriteString(fmt.Sprintf("Log Lines: %d\n", len(h.logs)))
	return sb.String()
}
