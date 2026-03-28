package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	"google.golang.org/protobuf/proto"

	"github.com/haksolot/kors/libs/core"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/bff/handler"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── Test infrastructure ────────────────────────────────────────────────────────

// startNATSContainer starts a NATS testcontainer and returns a connected client.
func startNATSContainer(t *testing.T) *nats.Conn {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcnats.Run(ctx, "nats:2.10-alpine")
	require.NoError(t, err, "start NATS container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	url, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	return nc
}

// natsStub holds helper subscriptions that auto-reply with a fixed proto response.
type natsStub struct {
	nc   *nats.Conn
	subs []*nats.Subscription
}

func newStub(nc *nats.Conn) *natsStub { return &natsStub{nc: nc} }

func (s *natsStub) handle(t *testing.T, subject string, resp proto.Message) {
	t.Helper()
	b, err := proto.Marshal(resp)
	require.NoError(t, err)
	sub, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		_ = msg.Respond(b)
	})
	require.NoError(t, err)
	s.subs = append(s.subs, sub)
}

func (s *natsStub) drain() {
	for _, sub := range s.subs {
		_ = sub.Drain()
	}
}

func startNATSStub(t *testing.T) *natsStub {
	t.Helper()
	nc := startNATSContainer(t)
	return newStub(nc)
}

func newTestHandler(t *testing.T, nc *nats.Conn) http.Handler {
	t.Helper()
	reg := prometheus.NewRegistry()
	log := zerolog.Nop()
	h := handler.New(context.Background(), nc, core.NewNoopJWTValidator(), reg, log)
	return h.Routes()
}

// ── Tests ──────────────────────────────────────────────────────────────────────

func TestHandler_CreateOrder(t *testing.T) {
	nc := startNATSContainer(t)
	stub := newStub(nc)
	defer stub.drain()

	orderID := "11111111-1111-1111-1111-111111111111"
	stub.handle(t, mesdomain.SubjectOFCreate, &pbmes.CreateOrderResponse{
		Order: &pbmes.ManufacturingOrder{Id: orderID, Reference: "OF-001"},
	})

	h := newTestHandler(t, nc)
	body := `{"reference":"OF-001","product_id":"prod-1","quantity":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), orderID)
}

func TestHandler_GetOrder(t *testing.T) {
	nc := startNATSContainer(t)
	stub := newStub(nc)
	defer stub.drain()

	orderID := "22222222-2222-2222-2222-222222222222"
	stub.handle(t, mesdomain.SubjectOFGet, &pbmes.GetOrderResponse{
		Order: &pbmes.ManufacturingOrder{Id: orderID},
	})

	h := newTestHandler(t, nc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID, nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), orderID)
}

func TestHandler_ListOrders(t *testing.T) {
	nc := startNATSContainer(t)
	stub := newStub(nc)
	defer stub.drain()

	stub.handle(t, mesdomain.SubjectOFList, &pbmes.ListOrdersResponse{
		Orders: []*pbmes.ManufacturingOrder{{Id: "aaa"}, {Id: "bbb"}},
	})

	h := newTestHandler(t, nc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "aaa")
}

func TestHandler_StartOperation_InjectsOperatorID(t *testing.T) {
	nc := startNATSContainer(t)
	defer nc.Close()

	opID := "33333333-3333-3333-3333-333333333333"
	ofID := "44444444-4444-4444-4444-444444444444"
	var capturedReq pbmes.StartOperationRequest
	sub, err := nc.Subscribe(mesdomain.SubjectOperationStart, func(msg *nats.Msg) {
		_ = proto.Unmarshal(msg.Data, &capturedReq)
		b, _ := proto.Marshal(&pbmes.StartOperationResponse{
			Operation: &pbmes.Operation{Id: opID},
		})
		_ = msg.Respond(b)
	})
	require.NoError(t, err)
	defer sub.Drain() //nolint:errcheck

	h := newTestHandler(t, nc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+ofID+"/operations/"+opID+"/start", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// operator_id must come from JWT claims, never from client body
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", capturedReq.OperatorId)
	assert.Contains(t, capturedReq.OperatorRoles, "kors-admin")
}

func TestHandler_DeclareNC_InjectsCallerID(t *testing.T) {
	nc := startNATSContainer(t)
	defer nc.Close()

	ofID := "55555555-5555-5555-5555-555555555555"
	opID := "66666666-6666-6666-6666-666666666666"
	var capturedReq pbmes.DeclareNCRequest
	sub, err := nc.Subscribe(mesdomain.SubjectOperationDeclareNC, func(msg *nats.Msg) {
		_ = proto.Unmarshal(msg.Data, &capturedReq)
		b, _ := proto.Marshal(&pbmes.DeclareNCResponse{EventId: "evt-1"})
		_ = msg.Respond(b)
	})
	require.NoError(t, err)
	defer sub.Drain() //nolint:errcheck

	h := newTestHandler(t, nc)
	body := `{"defect_code":"DIM_OUT_OF_TOLERANCE","description":"part too long","affected_quantity":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+ofID+"/operations/"+opID+"/nc",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	// declared_by must come from JWT, never from the client body
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", capturedReq.DeclaredBy)
	assert.Equal(t, opID, capturedReq.OperationId)
	assert.Equal(t, ofID, capturedReq.OfId)
}

func TestHandler_Health(t *testing.T) {
	nc := startNATSContainer(t)
	h := newTestHandler(t, nc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "kors-bff")
}

func TestHandler_NATS_Timeout_Returns502(t *testing.T) {
	nc := startNATSContainer(t)
	// No subscriber registered — request will time out
	h := newTestHandler(t, nc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/some-id", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}
