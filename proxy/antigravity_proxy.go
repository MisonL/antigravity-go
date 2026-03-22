package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// AntigravityProxy 是完整的 Antigravity 代理服务器
type AntigravityProxy struct {
	cmd              *exec.Cmd
	stdin            io.WriteCloser
	stdout           io.Reader
	stderr           io.Reader
	extensionPort    int
	httpPort         int
	extensionServer  *MockExtensionServer
	mu               sync.RWMutex
	nextID           int
	responses        map[interface{}]chan []byte
	lspReady         bool
	lspReadyChan     chan bool
}

// MockExtensionServer 模拟 VSCode 扩展服务器
type MockExtensionServer struct {
	listener net.Listener
	port     int
}

// NewAntigravityProxy 创建新的代理实例
func NewAntigravityProxy(extensionPort, httpPort int) *AntigravityProxy {
	return &AntigravityProxy{
		extensionPort: extensionPort,
		httpPort:      httpPort,
		responses:     make(map[interface{}]chan []byte),
		lspReadyChan:  make(chan bool, 1),
	}
}

// Start 启动代理服务器
func (p *AntigravityProxy) Start() error {
	// 1. 启动模拟扩展服务器
	log.Printf("Starting mock extension server on port %d", p.extensionPort)
	p.extensionServer = &MockExtensionServer{port: p.extensionPort}
	if err := p.extensionServer.Start(); err != nil {
		return fmt.Errorf("failed to start extension server: %w", err)
	}

	// 等待扩展服务器启动
	time.Sleep(500 * time.Millisecond)

	// 2. 启动 antigravity_core
	log.Println("Starting antigravity_core...")
	if err := p.startAntigravityCore(); err != nil {
		return fmt.Errorf("failed to start antigravity_core: %w", err)
	}

	// 3. 等待 LSP 准备就绪
	log.Println("Waiting for LSP to be ready...")
	select {
	case <-p.lspReadyChan:
		log.Println("✅ LSP is ready!")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for LSP to be ready")
	}

	// 4. 启动 HTTP 服务器
	log.Printf("Starting HTTP server on port %d", p.httpPort)
	p.startHTTPServer()

	return nil
}

// startAntigravityCore 启动 antigravity_core 进程
func (p *AntigravityProxy) startAntigravityCore() error {
	metadata := p.generateMetadata()

	p.cmd = exec.Command("../antigravity_core",
		"--enable_lsp",
		"--app_data_dir", "test_data",
		"--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com",
		"--api_server_url", "http://127.0.0.1:50001",
		fmt.Sprintf("--extension_server_port=%d", p.extensionPort),
	)

	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start antigravity_core: %w", err)
	}

	// 发送 metadata
	_, err = p.stdin.Write(metadata)
	if err != nil {
		p.cmd.Process.Kill()
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	p.stdin.Close()

	// 启动 stderr 读取器
	go p.readStderr()

	// 启动 stdout 读取器 (LSP 响应)
	go p.readStdout()

	return nil
}

// readStderr 读取 stderr 输出
func (p *AntigravityProxy) readStderr() {
	scanner := bufio.NewScanner(p.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[Antigravity] %s", line)

		// 检查 LSP 是否准备就绪
		if bytes.Contains([]byte(line), []byte("initialized server successfully")) {
			p.mu.Lock()
			if !p.lspReady {
				p.lspReady = true
				p.lspReadyChan <- true
			}
			p.mu.Unlock()
		}
	}
}

// readStdout 读取 stdout 输出 (LSP 响应)
func (p *AntigravityProxy) readStdout() {
	scanner := bufio.NewScanner(p.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()

		// 解析 JSON-RPC 响应
		var response map[string]interface{}
		if err := json.Unmarshal(line, &response); err != nil {
			log.Printf("Failed to parse LSP response: %v", err)
			continue
		}

		// 获取 ID
		id, ok := response["id"]
		if !ok {
			continue
		}

		// 发送响应到对应的 channel
		p.mu.RLock()
		if ch, exists := p.responses[id]; exists {
			ch <- line
		}
		p.mu.RUnlock()
	}
}

// generateMetadata 生成 Protobuf metadata 消息
func (p *AntigravityProxy) generateMetadata() []byte {
	buf := new(bytes.Buffer)

	// Tag: field 1, wire type 2 (length-delimited)
	buf.WriteByte(0x0a) // (1 << 3) | 2

	// 简单的 JSON 对象
	value := []byte("{}")
	buf.WriteByte(byte(len(value)))
	buf.Write(value)

	return buf.Bytes()
}

// startHTTPServer 启动 HTTP 服务器
func (p *AntigravityProxy) startHTTPServer() {
	http.HandleFunc("/", p.handleHTTP)
	http.HandleFunc("/health", p.handleHealth)
	http.HandleFunc("/lsp/", p.handleLSP)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", p.httpPort), nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

// handleHTTP 处理 HTTP 请求
func (p *AntigravityProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    "Antigravity Proxy",
		"version": "1.0.0",
		"status":  "running",
	})
}

// handleHealth 处理健康检查
func (p *AntigravityProxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	p.mu.RLock()
	ready := p.lspReady
	p.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"lsp_ready": ready,
	})
}

// handleLSP 处理 LSP 请求
func (p *AntigravityProxy) handleLSP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 解析请求
	var request map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	method, _ := request["method"].(string)
	log.Printf("LSP Request: %s", method)

	// 发送 LSP 请求
	response, err := p.sendLSPRequest(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(response)
}

// sendLSPRequest 发送 LSP 请求
func (p *AntigravityProxy) sendLSPRequest(request map[string]interface{}) ([]byte, error) {
	p.mu.Lock()
	id := p.nextID
	p.nextID++
	p.mu.Unlock()

	// 设置 ID
	request["id"] = id
	request["jsonrpc"] = "2.0"

	// 创建响应 channel
	responseCh := make(chan []byte, 1)
	p.mu.Lock()
	p.responses[id] = responseCh
	p.mu.Unlock()

	// 编码请求
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 这里需要将请求发送到 antigravity_core
	// 由于我们已经关闭了 stdin，我们需要通过其他方式发送
	// 暂时返回一个模拟响应
	log.Printf("LSP request (not implemented yet): %s", string(requestJSON))

	p.mu.Lock()
	delete(p.responses, id)
	p.mu.Unlock()

	return json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  nil,
	})
}

// Stop 停止代理服务器
func (p *AntigravityProxy) Stop() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	if p.extensionServer != nil {
		p.extensionServer.Stop()
	}
}

// MockExtensionServer 实现
func (s *MockExtensionServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}

	go s.acceptConnections()
	return nil
}

func (s *MockExtensionServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *MockExtensionServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		var request map[string]interface{}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			continue
		}

		response := s.handleRequest(request)
		if response != nil {
			responseJSON, _ := json.Marshal(response)
			fmt.Fprintf(conn, "%s\n", responseJSON)
		}
	}
}

func (s *MockExtensionServer) handleRequest(request map[string]interface{}) map[string]interface{} {
	method, _ := request["method"].(string)
	id := request["id"]

	switch method {
	case "initialize":
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]interface{}{
				"capabilities": map[string]interface{}{
					"textDocumentSync": 1,
				},
			},
		}
	case "initialized", "exit":
		return nil
	default:
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  nil,
		}
	}
}

func (s *MockExtensionServer) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 切换到正确的目录
	os.Chdir("/Volumes/Work/code/antigravity-go/proxy")

	// 创建代理实例
	proxy := NewAntigravityProxy(49916, 9000)

	// 启动代理
	if err := proxy.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Stop()

	log.Println("✅ Antigravity Proxy is running!")
	log.Println("   HTTP API: http://localhost:9000")
	log.Println("   Health Check: http://localhost:9000/health")
}
