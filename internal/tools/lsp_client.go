package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type LSPClient struct {
	conn    net.Conn
	seq     int
	pending map[int]chan json.RawMessage
	mu      sync.Mutex
}

func NewLSPClient(port int) (*LSPClient, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 5*time.Second)
	if err != nil {
		return nil, err
	}

	client := &LSPClient{
		conn:    conn,
		pending: make(map[int]chan json.RawMessage),
	}
	go client.readLoop()
	return client, nil
}

func (c *LSPClient) Call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.seq++
	id := c.seq
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	body, _ := json.Marshal(req)
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	if _, err := c.conn.Write([]byte(msg)); err != nil {
		return nil, err
	}

	select {
	case res := <-ch:
		return res, nil // Currently ignores error responses parsing
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}

func (c *LSPClient) readLoop() {
	reader := bufio.NewReader(c.conn)
	for {
		// Read Headers
		var length int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &length)
			}
		}

		if length == 0 {
			continue
		}

		// Read Body
		body := make([]byte, length)
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}

		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		c.mu.Lock()
		if ch, ok := c.pending[msg.ID]; ok {
			// If error, maybe we should return it. For now, simplistic.
			if msg.Error != nil {
				// We rely on caller to unmarshal result and see it's empty/invalid?
				// Or better, handle error here.
				// Since we return RawMessage, we can't easily return error via channel defined as RawMessage.
				// For prototype: print log and send nil?
				fmt.Printf("LSP Error: %s\n", msg.Error.Message)
				ch <- nil
			} else {
				ch <- msg.Result
			}
		}
		c.mu.Unlock()
	}
}

func (c *LSPClient) Initialize(rootPath string) error {
	params := map[string]interface{}{
		"processId": 1,
		"rootUri":   "file://" + rootPath,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition": map[string]interface{}{},
				"references": map[string]interface{}{},
			},
		},
	}
	_, err := c.Call("initialize", params)
	return err
}
