package server

import (
	"net"
	"sync"
)

// ConnectionTracker retains accepted sockets so protocols without native
// connection draining can terminate their active sessions during shutdown.
type ConnectionTracker struct {
	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
}

func NewConnectionTracker() *ConnectionTracker {
	return &ConnectionTracker{conns: make(map[net.Conn]struct{})}
}

func (t *ConnectionTracker) Wrap(listener net.Listener) net.Listener {
	return &trackedListener{Listener: listener, tracker: t}
}

// CloseAll prevents newly accepted sockets from escaping the tracker and
// closes every active connection.
func (t *ConnectionTracker) CloseAll() {
	t.mu.Lock()
	t.closed = true
	connections := make([]net.Conn, 0, len(t.conns))
	for connection := range t.conns {
		connections = append(connections, connection)
	}
	t.mu.Unlock()

	for _, connection := range connections {
		_ = connection.Close()
	}
}

type trackedListener struct {
	net.Listener
	tracker *ConnectionTracker
}

func (l *trackedListener) Accept() (net.Conn, error) {
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	wrapper := &trackedConnection{Conn: connection, tracker: l.tracker}
	l.tracker.mu.Lock()
	if l.tracker.closed {
		l.tracker.mu.Unlock()
		_ = connection.Close()
		return nil, net.ErrClosed
	}
	l.tracker.conns[wrapper] = struct{}{}
	l.tracker.mu.Unlock()
	return wrapper, nil
}

type trackedConnection struct {
	net.Conn
	tracker *ConnectionTracker
	once    sync.Once
}

func (c *trackedConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		c.tracker.mu.Lock()
		delete(c.tracker.conns, c)
		c.tracker.mu.Unlock()
	})
	return err
}
