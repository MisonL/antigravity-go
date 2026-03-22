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

// MockExtensionServer 模拟 VSCode 扩展服务器
type MockExtensionServer struct {
	listener net.Listener
	port     int
	mu       sync.Mutex
}

// NewMockExtensionServer 创建一个新的模拟扩展服务器
func NewMockExtensionServer(port int) *MockExtensionServer {
	return &MockExtensionServer{
		port: port,
	}
}

// Start 启动模拟扩展服务器
func (s *MockExtensionServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	log.Printf("Mock extension server listening on port %d", s.port)

	go s.acceptConnections()

	return nil
}

// acceptConnections 接受连接
func (s *MockExtensionServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *MockExtensionServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("New connection from %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	// 读取请求
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading: %v", err)
			}
			break
		}

		log.Printf("Received: %s", line)

		// 解析请求
		var request map[string]interface{}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		// 处理请求
		response := s.handleRequest(request)

		// 发送响应
		responseJSON, _ := json.Marshal(response)
		fmt.Fprintf(conn, "%s\n", responseJSON)
	}
}

// handleRequest 处理请求
func (s *MockExtensionServer) handleRequest(request map[string]interface{}) map[string]interface{} {
	method, _ := request["method"].(string)
	id := request["id"]

	log.Printf("Method: %s", method)

	switch method {
	case "initialize":
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]interface{}{
				"capabilities": map[string]interface{}{
					"textDocumentSync": 1,
					"completionProvider": map[string]interface{}{
						"resolveProvider": false,
					},
					"hoverProvider": true,
				},
			},
		}

	case "initialized":
		return nil // 通知，不需要响应

	case "shutdown":
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": nil,
		}

	case "exit":
		return nil // 通知，不需要响应

	default:
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": fmt.Sprintf("Method not supported: %s", method),
			},
		}
	}
}

// Stop 停止服务器
func (s *MockExtensionServer) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// generateMetadata 生成 metadata 消息
func generateMetadata() []byte {
	buf := new(bytes.Buffer)

	// Tag: field 1, wire type 2 (length-delimited)
	buf.WriteByte(0x0a) // (1 << 3) | 2

	// 简单的 JSON 对象
	value := []byte("{}")
	buf.WriteByte(byte(len(value)))
	buf.Write(value)

	return buf.Bytes()
}

// startAntigravityCore 启动 antigravity_core
func startAntigravityCore(extensionPort int) (*exec.Cmd, error) {
	metadata := generateMetadata()

	cmd := exec.Command("../antigravity_core",
		"--enable_lsp",
		"--app_data_dir", "test_data",
		"--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com",
		"--api_server_url", "http://127.0.0.1:50001",
		fmt.Sprintf("--extension_server_port=%d", extensionPort),
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start antigravity_core: %w", err)
	}

	// 发送 metadata
	_, err = stdin.Write(metadata)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}
	stdin.Close()

	// 读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("Antigravity: %s", scanner.Text())
		}
	}()

	// 读取 stdout (LSP 响应)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Printf("LSP Response: %s", scanner.Text())
		}
	}()

	return cmd, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 切换到正确的目录
	os.Chdir("/Volumes/Work/code/antigravity-go/proxy")

	// 启动模拟扩展服务器
	extensionPort := 49916
	mockServer := NewMockExtensionServer(extensionPort)

	if err := mockServer.Start(); err != nil {
		log.Fatalf("Failed to start mock extension server: %v", err)
	}
	defer mockServer.Stop()

	// 等待服务器启动
	time.Sleep(1 * time.Second)

	// 启动 antigravity_core
	log.Println("Starting antigravity_core...")
	cmd, err := startAntigravityCore(extensionPort)
	if err != nil {
		log.Fatalf("Failed to start antigravity_core: %v", err)
	}
	defer cmd.Process.Kill()

	// 等待一段时间
	log.Println("Waiting for antigravity_core to initialize...")
	time.Sleep(10 * time.Second)

	// 检查进程状态
	if cmd.ProcessState == nil {
		log.Println("✅ antigravity_core is still running!")
	} else {
		log.Printf("❌ antigravity_core exited with status: %v", cmd.ProcessState)
	}

	// 启动 HTTP 服务器提供 API
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"message": "Antigravity proxy is running",
		})
	})

	httpPort := 8080
	log.Printf("Starting HTTP server on port %d", httpPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
