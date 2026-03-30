package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"github.com/pressly/goose/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/protobuf/proto"

	mesdomain "github.com/haksolot/kors/services/mes/domain"
	meshandler "github.com/haksolot/kors/services/mes/handler"
	mesoutbox "github.com/haksolot/kors/services/mes/outbox"
	mesrepo "github.com/haksolot/kors/services/mes/repo"
	qmsdomain "github.com/haksolot/kors/services/qms/domain"
	qmshandler "github.com/haksolot/kors/services/qms/handler"
	qmsoutbox "github.com/haksolot/kors/services/qms/outbox"
	qmsrepo "github.com/haksolot/kors/services/qms/repo"
	qmssub "github.com/haksolot/kors/services/qms/subscriber"
)

// ── Infrastructure ─────────────────────────────────────────────────────────────

// startNATS starts a NATS container with JetStream enabled and returns the URL.
func startNATS(t *testing.T, ctx context.Context) string {
	t.Helper()
	ctr, err := tcnats.Run(ctx, "nats:2.10-alpine")
	require.NoError(t, err, "start NATS container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	url, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)
	return url
}

// startPostgres starts a Postgres container, runs migrations from migrationsPath,
// and returns a connection pool.
func startPostgres(t *testing.T, ctx context.Context, migrationsPath string) *pgxpool.Pool {
	t.Helper()
	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("kors_test"),
		tcpostgres.WithUsername("kors"),
		tcpostgres.WithPassword("kors_test"),
		tcpostgres.WithSQLDriver("pgx"),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return pool.Ping(ctx) == nil
	}, 30*time.Second, 500*time.Millisecond, "postgres not ready")
	t.Cleanup(pool.Close)

	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, migrationsPath), "run migrations: "+migrationsPath)

	return pool
}

// ── Service wiring ─────────────────────────────────────────────────────────────

// startMESService wires up the MES handler, subscribes all subjects, and starts the outbox worker.
// Returns a cancel function to stop the service.
func startMESService(t *testing.T, ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn) {
	t.Helper()
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	r := mesrepo.New(pool)
	h := meshandler.New(r, r, r, r, r, r, r, r, r, r, r, r, r, r, reg, &log)
	worker := mesoutbox.New(r, nc, log, reg)

	subs := subscribeMES(t, ctx, h, nc)
	t.Cleanup(func() {
		for _, s := range subs { _ = s.Drain() }
	})

	cancelCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go worker.Run(cancelCtx)
}

// startQMSService wires up the QMS handler, subscriber, and outbox worker.
func startQMSService(t *testing.T, ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn) {
	t.Helper()
	log := zerolog.Nop()
	reg := prometheus.NewRegistry()
	r := qmsrepo.New(pool)
	h := qmshandler.New(r, r, r, reg, &log)
	worker := qmsoutbox.New(r, nc, log, reg)
	sub := qmssub.New(r, nc, log)

	subs := subscribeQMS(t, ctx, h, nc)
	t.Cleanup(func() {
		for _, s := range subs { _ = s.Drain() }
	})

	evtSubs, err := sub.Subscribe()
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, s := range evtSubs { _ = s.Drain() }
	})

	cancelCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	go worker.Run(cancelCtx)
}

// subscribeMES registers all MES NATS request-reply subscriptions.
func subscribeMES(t *testing.T, ctx context.Context, h *meshandler.Handler, nc *nats.Conn) []*nats.Subscription {
	t.Helper()
	type entry struct {
		subject string
		fn      func(context.Context, []byte) ([]byte, error)
	}
	routes := []entry{
		{mesdomain.SubjectOFCreate, h.CreateOrder},
		{mesdomain.SubjectOFGet, h.GetOrder},
		{mesdomain.SubjectOFList, h.ListOrders},
		{mesdomain.SubjectOFSuspend, h.SuspendOrder},
		{mesdomain.SubjectOFResume, h.ResumeOrder},
		{mesdomain.SubjectOFCancel, h.CancelOrder},
		{mesdomain.SubjectOperationCreate, h.CreateOperation},
		{mesdomain.SubjectOperationGet, h.GetOperation},
		{mesdomain.SubjectOperationList, h.ListOperations},
		{mesdomain.SubjectOperationStart, h.StartOperation},
		{mesdomain.SubjectOperationComplete, h.CompleteOperation},
		{mesdomain.SubjectOperationSkip, h.SkipOperation},
		{mesdomain.SubjectLotCreate, h.CreateLot},
		{mesdomain.SubjectLotGet, h.GetLot},
		{mesdomain.SubjectSNRegister, h.RegisterSN},
		{mesdomain.SubjectSNGet, h.GetSN},
		{mesdomain.SubjectSNRelease, h.ReleaseSN},
		{mesdomain.SubjectSNScrap, h.ScrapSN},
		{mesdomain.SubjectGenealogyAdd, h.AddGenealogyEntry},
		{mesdomain.SubjectGenealogyGet, h.GetGenealogy},
		{mesdomain.SubjectOperationSignOff, h.SignOffOperation},
		{mesdomain.SubjectOperationDeclareNC, h.DeclareNC},
		{mesdomain.SubjectOFFAIApprove, h.ApproveFAI},
		{mesdomain.SubjectOperationAttachInstructions, h.AttachInstructions},
		{mesdomain.SubjectRoutingCreate, h.CreateRouting},
		{mesdomain.SubjectRoutingGet, h.GetRouting},
		{mesdomain.SubjectRoutingList, h.ListRoutings},
		{mesdomain.SubjectOFCreateFromRouting, h.CreateFromRouting},
		{mesdomain.SubjectOFDispatchList, h.GetDispatchList},
		{mesdomain.SubjectOFSetPlanning, h.SetPlanning},
	}

	var subs []*nats.Subscription
	for _, r := range routes {
		r := r
		sub, err := nc.QueueSubscribe(r.subject, mesdomain.QueueGroupMES, func(msg *nats.Msg) {
			resp, err := r.fn(ctx, msg.Data)
			if err != nil && msg.Reply != "" {
				_ = msg.Respond([]byte("error: " + err.Error()))
				return
			}
			if msg.Reply != "" {
				_ = msg.Respond(resp)
			}
		})
		require.NoError(t, err)
		subs = append(subs, sub)
	}
	return subs
}

// subscribeQMS registers all QMS NATS request-reply subscriptions.
func subscribeQMS(t *testing.T, ctx context.Context, h *qmshandler.Handler, nc *nats.Conn) []*nats.Subscription {
	t.Helper()
	type entry struct {
		subject string
		fn      func(context.Context, []byte) ([]byte, error)
	}
	routes := []entry{
		{qmsdomain.SubjectNCGet, h.GetNC},
		{qmsdomain.SubjectNCList, h.ListNCs},
		{qmsdomain.SubjectNCAnalyse, h.StartAnalysis},
		{qmsdomain.SubjectNCProposeDisposition, h.ProposeDisposition},
		{qmsdomain.SubjectNCClose, h.CloseNC},
		{qmsdomain.SubjectCAPACreate, h.CreateCAPA},
		{qmsdomain.SubjectCAPAGet, h.GetCAPA},
		{qmsdomain.SubjectCAPAList, h.ListCAPAs},
		{qmsdomain.SubjectCAPAStart, h.StartCAPA},
		{qmsdomain.SubjectCAPAComplete, h.CompleteCAPA},
	}

	var subs []*nats.Subscription
	for _, r := range routes {
		r := r
		sub, err := nc.QueueSubscribe(r.subject, qmsdomain.QueueGroupQMS, func(msg *nats.Msg) {
			resp, err := r.fn(ctx, msg.Data)
			if err != nil && msg.Reply != "" {
				_ = msg.Respond([]byte("error: " + err.Error()))
				return
			}
			if msg.Reply != "" {
				_ = msg.Respond(resp)
			}
		})
		require.NoError(t, err)
		subs = append(subs, sub)
	}
	return subs
}

// natsConnect connects a NATS client and registers cleanup.
func natsConnect(url string) (*nats.Conn, error) {
	return nats.Connect(url,
		nats.MaxReconnects(5),
		nats.ReconnectWait(200*time.Millisecond),
	)
}

// ── NATS helpers ───────────────────────────────────────────────────────────────

// natsReq sends a proto-encoded request and unmarshal the response into resp.
func natsReq(t *testing.T, nc *nats.Conn, subject string, req proto.Message, resp proto.Message) {
	t.Helper()
	payload, err := proto.Marshal(req)
	require.NoError(t, err)

	msg, err := nc.Request(subject, payload, 5*time.Second)
	require.NoError(t, err, "NATS request to %s", subject)

	// Check for handler-returned error string
	if len(msg.Data) > 7 && string(msg.Data[:7]) == "error: " {
		t.Fatalf("handler returned error for %s: %s", subject, string(msg.Data))
	}

	require.NoError(t, proto.Unmarshal(msg.Data, resp), "unmarshal response from %s", subject)
}

// captureEvent subscribes to a subject and returns a channel that receives raw messages.
func captureEvent(t *testing.T, nc *nats.Conn, subject string) chan []byte {
	t.Helper()
	ch := make(chan []byte, 16)
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		data := make([]byte, len(msg.Data))
		copy(data, msg.Data)
		ch <- data
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	return ch
}

// waitEvent waits for exactly one event on ch within the timeout.
func waitEvent(t *testing.T, ch chan []byte, timeout time.Duration, description string) []byte {
	t.Helper()
	select {
	case data := <-ch:
		return data
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for event: %s", description)
		return nil
	}
}
