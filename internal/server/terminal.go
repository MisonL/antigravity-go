package server

import (
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// Terminal WebSocket Handler
func (s *Server) HandleTerminalWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return isAllowedLocalOrigin(r) },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Terminal WS upgrade error: %v", err)
		return
	}

	// Create and register client
	client := &TerminalClient{
		manager: s.tm,
		conn:    conn,
		send:    make(chan []byte, 256),
	}
	s.tm.register <- client

	// Start write pump in a goroutine
	go client.writePump()

	// Defer cleanup (unregister handles closing connection)
	defer func() {
		s.tm.unregister <- client
	}()

	// Start a shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = os.Environ()

	// Create PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("PTY start error: %v", err)
		msg := []byte("Error starting shell: " + err.Error())
		client.send <- msg
		return
	}
	defer func() {
		// Close PTY and waiting for process to exit
		ptmx.Close()
		cmd.Process.Kill()
		cmd.Wait() // Ensure process is reaped
	}()

	// Read from PTY -> Client Send Channel
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				break
			}
			// Important: Copy data to avoid race condition with buffer reuse
			data := make([]byte, n)
			copy(data, buf[:n])

			select {
			case client.send <- data:
			default:
				// If channel is full, we might drop packets or disconnect.
				// For now, let's just break, as it indicates a slow consumer.
				// Better to close connection than block PTY.
				return
			}
		}
	}()

	// Read from WS -> PTY (Control)
	// This blocks until connection is closed
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_, _ = ptmx.Write(data)
	}
}
