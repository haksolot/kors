package core

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	natsConnectTimeout    = 5 * time.Second
	natsReconnectWait     = 2 * time.Second
	natsMaxReconnects     = -1 // reconnect indefinitely
	natsReconnectBufSize  = 8 * 1024 * 1024 // 8 MB
)

// NewNATSConn establishes a NATS connection with automatic reconnection.
// url is the NATS server URL (e.g. "nats://localhost:4222").
// credsFile is the path to the NATS credentials file for per-service auth (may be empty for dev).
func NewNATSConn(url, credsFile string) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Timeout(natsConnectTimeout),
		nats.ReconnectWait(natsReconnectWait),
		nats.MaxReconnects(natsMaxReconnects),
		nats.ReconnectBufSize(natsReconnectBufSize),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				// Caller is responsible for logging — no global logger here.
				_ = err
			}
		}),
	}
	if credsFile != "" {
		opts = append(opts, nats.UserCredentials(credsFile))
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("NewNATSConn: connect to %s: %w", url, err)
	}
	return nc, nil
}

// Subscribe registers a queue-group subscription on subject.
// All instances of the same service should use the same queue name for load balancing.
// The handler receives each message; call msg.Ack() or msg.Nak() for JetStream messages.
func Subscribe(nc *nats.Conn, subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	sub, err := nc.QueueSubscribe(subject, queue, handler)
	if err != nil {
		return nil, fmt.Errorf("Subscribe: queue subscribe %s [%s]: %w", subject, queue, err)
	}
	return sub, nil
}

// Request performs a synchronous NATS request-reply call.
// Returns the response payload or an error if the timeout is exceeded.
func Request(ctx context.Context, nc *nats.Conn, subject string, payload []byte) ([]byte, error) {
	deadline, ok := ctx.Deadline()
	timeout := 5 * time.Second
	if ok {
		timeout = time.Until(deadline)
	}

	msg, err := nc.Request(subject, payload, timeout)
	if err != nil {
		return nil, fmt.Errorf("Request: %s: %w", subject, err)
	}
	return msg.Data, nil
}
