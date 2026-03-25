package browser

import (
	"sync/atomic"
)

// NotificationHub is a broadcast channel for string notifications.
// It is safe to call Send and Close concurrently.
type NotificationHub struct {
	ch     chan string
	closed atomic.Bool
}

// NewNotificationHub creates a NotificationHub with a buffered channel of capacity 100.
func NewNotificationHub() *NotificationHub {
	return &NotificationHub{
		ch: make(chan string, 100),
	}
}

// Send enqueues a notification message. It is a no-op if the hub is already closed
// or if the channel buffer is full (non-blocking).
func (n *NotificationHub) Send(msg string) {
	if n.closed.Load() {
		return
	}
	select {
	case n.ch <- msg:
	default:
	}
}

// Ch returns the read-only notification channel for consumers.
func (n *NotificationHub) Ch() <-chan string {
	return n.ch
}

// Close marks the hub as closed and closes the underlying channel.
// It is safe to call Close more than once.
func (n *NotificationHub) Close() {
	if n.closed.Swap(true) {
		return
	}
	close(n.ch)
}
