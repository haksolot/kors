package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	pbqms "github.com/haksolot/kors/proto/gen/qms"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
	qmsdomain "github.com/haksolot/kors/services/qms/domain"
)

// ── Role-based event filter ────────────────────────────────────────────────────

// roleFilter maps a NATS subject to the roles that may receive it.
// nil = all authenticated roles.
var roleFilter = map[string][]string{
	mesdomain.SubjectOFCreated:          nil,
	mesdomain.SubjectOFSuspended:        nil,
	mesdomain.SubjectOFResumed:          nil,
	mesdomain.SubjectOFCancelled:        nil,
	mesdomain.SubjectOperationStarted:   nil,
	mesdomain.SubjectOperationCompleted: nil,
	mesdomain.SubjectNCDeclared:         {"kors-quality", "kors-admin"},
	qmsdomain.SubjectNCOpened:           {"kors-quality", "kors-admin"},
	qmsdomain.SubjectNCClosed:           {"kors-quality", "kors-admin"},
	qmsdomain.SubjectCAPACreated:        {"kors-quality", "kors-admin"},
}

// eventFactory maps each NATS subject to the proto message it carries.
var eventFactory = map[string]func() proto.Message{
	mesdomain.SubjectOFCreated:          func() proto.Message { return &pbmes.OFCreatedEvent{} },
	mesdomain.SubjectOFSuspended:        func() proto.Message { return &pbmes.OFSuspendedEvent{} },
	mesdomain.SubjectOFResumed:          func() proto.Message { return &pbmes.OFResumedEvent{} },
	mesdomain.SubjectOFCancelled:        func() proto.Message { return &pbmes.OFCancelledEvent{} },
	mesdomain.SubjectOperationStarted:   func() proto.Message { return &pbmes.OperationStartedEvent{} },
	mesdomain.SubjectOperationCompleted: func() proto.Message { return &pbmes.OperationCompletedEvent{} },
	mesdomain.SubjectNCDeclared:         func() proto.Message { return &pbmes.NCDeclaredEvent{} },
	qmsdomain.SubjectNCOpened:           func() proto.Message { return &pbqms.NCOpenedEvent{} },
	qmsdomain.SubjectNCClosed:           func() proto.Message { return &pbqms.NCClosedEvent{} },
	qmsdomain.SubjectCAPACreated:        func() proto.Message { return &pbqms.CAPACreatedEvent{} },
}

// EventSubjects returns all NATS subjects the BFF should subscribe to for WebSocket fan-out.
func EventSubjects() []string {
	subjects := make([]string, 0, len(eventFactory))
	for s := range eventFactory {
		subjects = append(subjects, s)
	}
	return subjects
}

// ── Hub ────────────────────────────────────────────────────────────────────────

type wsClient struct {
	roles  []string
	send   chan []byte
	ctx    context.Context
	cancel context.CancelFunc
}

type hubMsg struct {
	subject string
	payload []byte // JSON-encoded WS message
}

// Hub manages all active WebSocket connections and fans out events.
type Hub struct {
	clients    map[*wsClient]struct{}
	register   chan *wsClient
	unregister chan *wsClient
	publish    chan hubMsg
	log        zerolog.Logger
}

func newHub(log zerolog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*wsClient]struct{}),
		register:   make(chan *wsClient, 8),
		unregister: make(chan *wsClient, 8),
		publish:    make(chan hubMsg, 256),
		log:        log,
	}
}

// Run is the Hub's main goroutine. Must be called exactly once.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				c.cancel()
			}
			return
		case c := <-h.register:
			h.clients[c] = struct{}{}
			h.log.Debug().Int("clients", len(h.clients)).Msg("ws: client connected")
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				c.cancel()
				h.log.Debug().Int("clients", len(h.clients)).Msg("ws: client disconnected")
			}
		case msg := <-h.publish:
			allowed := roleFilter[msg.subject]
			for c := range h.clients {
				if canReceive(c.roles, allowed) {
					select {
					case c.send <- msg.payload:
					default:
						// Client too slow — disconnect it.
						delete(h.clients, c)
						c.cancel()
					}
				}
			}
		}
	}
}

// Publish converts raw proto bytes from NATS into a JSON WebSocket message and
// enqueues it for fan-out. Safe to call from any goroutine.
func (h *Hub) Publish(subject string, protoBytes []byte) {
	factory, ok := eventFactory[subject]
	if !ok {
		return
	}
	msg := factory()
	if err := proto.Unmarshal(protoBytes, msg); err != nil {
		h.log.Error().Err(err).Str("subject", subject).Msg("ws: unmarshal proto event")
		return
	}
	payloadJSON, err := protoJSONMarshal.Marshal(msg)
	if err != nil {
		h.log.Error().Err(err).Str("subject", subject).Msg("ws: marshal proto to JSON")
		return
	}
	type wsMsg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	envelope, err := json.Marshal(wsMsg{Type: subject, Payload: payloadJSON})
	if err != nil {
		return
	}
	select {
	case h.publish <- hubMsg{subject: subject, payload: envelope}:
	default:
		h.log.Warn().Str("subject", subject).Msg("ws: hub publish channel full, dropping event")
	}
}

func canReceive(clientRoles, allowed []string) bool {
	if allowed == nil {
		return true
	}
	for _, r := range clientRoles {
		for _, a := range allowed {
			if r == a {
				return true
			}
		}
	}
	return false
}

// ── NATS subscriptions for WebSocket events ────────────────────────────────────

// SubscribeEvents subscribes to all event subjects and forwards them to the hub.
// Returns the subscriptions so the caller can drain them on shutdown.
func (h *Hub) SubscribeEvents(nc *nats.Conn) ([]*nats.Subscription, error) {
	var subs []*nats.Subscription
	for _, subject := range EventSubjects() {
		subject := subject
		sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			h.Publish(subject, msg.Data)
		})
		if err != nil {
			return nil, fmt.Errorf("ws: subscribe %s: %w", subject, err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// ── HTTP handler ───────────────────────────────────────────────────────────────

// ServeWS handles a WebSocket upgrade. The JWT is expected as ?token=<jwt>.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "missing token query parameter")
		return
	}
	claims, err := h.validator.ValidateJWT(r.Context(), tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow cross-origin for tablet clients
	})
	if err != nil {
		h.log.Error().Err(err).Msg("ws: accept failed")
		return
	}

	clientCtx, cancel := context.WithCancel(r.Context())
	c := &wsClient{
		roles:  claims.Roles,
		send:   make(chan []byte, 32),
		ctx:    clientCtx,
		cancel: cancel,
	}

	h.hub.register <- c
	defer func() {
		h.hub.unregister <- c
		conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	// Write goroutine — sends buffered messages to the WebSocket client.
	go func() {
		for {
			select {
			case data := <-c.send:
				if err := conn.Write(clientCtx, websocket.MessageText, data); err != nil {
					cancel()
					return
				}
			case <-clientCtx.Done():
				return
			}
		}
	}()

	// Read loop — keeps the connection alive and detects client disconnect.
	for {
		if _, _, err := conn.Read(clientCtx); err != nil {
			return
		}
	}
}
