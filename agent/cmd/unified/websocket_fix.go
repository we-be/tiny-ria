package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// ThreadSafeConn wraps a gorilla/websocket.Conn with a mutex for thread safety
type ThreadSafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewThreadSafeConn creates a new thread-safe connection wrapper
func NewThreadSafeConn(conn *websocket.Conn) *ThreadSafeConn {
	return &ThreadSafeConn{
		conn: conn,
	}
}

// WriteJSON is a thread-safe wrapper for websocket.Conn.WriteJSON
func (c *ThreadSafeConn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

// ReadJSON delegates to the underlying connection's ReadJSON method
func (c *ThreadSafeConn) ReadJSON(v interface{}) error {
	return c.conn.ReadJSON(v)
}

// Close delegates to the underlying connection's Close method
func (c *ThreadSafeConn) Close() error {
	return c.conn.Close()
}