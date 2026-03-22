package server

import (
	"sync"

	"github.com/gorilla/websocket"
)

// TerminalClient wraps a websocket connection with a thread-safe send channel.
type TerminalClient struct {
	manager *TerminalManager
	conn    *websocket.Conn
	send    chan []byte
}

func (c *TerminalClient) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
			return
		}
	}
}

// TerminalManager manages active terminal clients.
type TerminalManager struct {
	clients    map[*TerminalClient]bool
	register   chan *TerminalClient
	unregister chan *TerminalClient
	broadcast  chan []byte
	mu         sync.Mutex
}

func NewTerminalManager() *TerminalManager {
	tm := &TerminalManager{
		clients:    make(map[*TerminalClient]bool),
		register:   make(chan *TerminalClient),
		unregister: make(chan *TerminalClient),
		broadcast:  make(chan []byte),
	}
	go tm.run()
	return tm
}

func (tm *TerminalManager) run() {
	for {
		select {
		case client := <-tm.register:
			tm.mu.Lock()
			tm.clients[client] = true
			tm.mu.Unlock()
		case client := <-tm.unregister:
			tm.mu.Lock()
			if _, ok := tm.clients[client]; ok {
				delete(tm.clients, client)
				close(client.send)
			}
			tm.mu.Unlock()
		case message := <-tm.broadcast:
			tm.mu.Lock()
			for client := range tm.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(tm.clients, client)
				}
			}
			tm.mu.Unlock()
		}
	}
}

// Broadcast sends a message to all connected terminals.
// This is safe to call from any goroutine (e.g. Agent Shell Tool).
func (tm *TerminalManager) Broadcast(data []byte) {
	tm.broadcast <- data
}
