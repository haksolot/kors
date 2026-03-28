package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	pbqms "github.com/haksolot/kors/proto/gen/qms"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
	mesrepo "github.com/haksolot/kors/services/mes/repo"
	qmsdomain "github.com/haksolot/kors/services/qms/domain"
	qmsrepo "github.com/haksolot/kors/services/qms/repo"
)

// TestE2EScenario is the full end-to-end test for MES + QMS.
// It starts real infrastructure via testcontainers, wires both services in-process,
// and exercises the complete workflow: OF → Operation lifecycle → NC declaration →
// QMS NC lifecycle → CAPA lifecycle.
func TestE2EScenario(t *testing.T) {
	ctx := context.Background()

	// ── Infrastructure ────────────────────────────────────────────────────────
	t.Log("starting infrastructure...")
	natsURL := startNATS(t, ctx)
	mesPool := startPostgres(t, ctx, "../services/mes/migrations")
	qmsPool := startPostgres(t, ctx, "../services/qms/migrations")

	// Connect test client to NATS
	nc, err := natsConnect(natsURL)
	require.NoError(t, err)
	defer nc.Drain()

	// ── Services ──────────────────────────────────────────────────────────────
	t.Log("wiring MES + QMS services...")
	startMESService(t, ctx, mesPool, nc)
	startQMSService(t, ctx, qmsPool, nc)

	// Give subscriptions time to register before sending requests
	time.Sleep(100 * time.Millisecond)

	// ── Phase 1 — Manufacturing Order lifecycle ───────────────────────────────
	t.Run("Phase_1_OF_lifecycle", func(t *testing.T) {
		t.Log("creating manufacturing order...")
		var createResp pbmes.CreateOrderResponse
		natsReq(t, nc, mesdomain.SubjectOFCreate, &pbmes.CreateOrderRequest{
			Reference: "OF-E2E-001",
			ProductId: "00000000-0000-0000-0000-000000000001",
			Quantity:  5,
		}, &createResp)

		require.NotNil(t, createResp.Order)
		assert.Equal(t, "OF-E2E-001", createResp.Order.Reference)
		assert.Equal(t, pbmes.OrderStatus_ORDER_STATUS_PLANNED, createResp.Order.Status)
		ofID := createResp.Order.Id
		assert.NotEmpty(t, ofID)

		// GetOrder
		var getResp pbmes.GetOrderResponse
		natsReq(t, nc, mesdomain.SubjectOFGet, &pbmes.GetOrderRequest{Id: ofID}, &getResp)
		assert.Equal(t, ofID, getResp.Order.Id)

		// ListOrders
		var listResp pbmes.ListOrdersResponse
		natsReq(t, nc, mesdomain.SubjectOFList, &pbmes.ListOrdersRequest{}, &listResp)
		assert.GreaterOrEqual(t, len(listResp.Orders), 1)
	})

	// ── Phase 2 — Routing + CreateFromRouting ─────────────────────────────────
	var routingID string
	var fromRoutingOFID string
	var op1ID, op2ID string

	t.Run("Phase_2_Routing_CreateFromRouting", func(t *testing.T) {
		t.Log("creating routing with 2 steps...")
		var createRoutingResp pbmes.CreateRoutingResponse
		natsReq(t, nc, mesdomain.SubjectRoutingCreate, &pbmes.CreateRoutingRequest{
			ProductId: "00000000-0000-0000-0000-000000000001",
			Name:      "Standard Assembly Routing",
			Version:   1,
			Activate:  true,
			Steps: []*pbmes.CreateRoutingStepInput{
				{
					StepNumber:             1,
					Name:                   "Machining",
					PlannedDurationSeconds: 600,
				},
				{
					StepNumber:             2,
					Name:                   "Assembly",
					PlannedDurationSeconds: 900,
				},
			},
		}, &createRoutingResp)

		require.NotNil(t, createRoutingResp.Routing)
		routingID = createRoutingResp.Routing.Id
		assert.NotEmpty(t, routingID)
		assert.Len(t, createRoutingResp.Routing.Steps, 2)

		// CreateFromRouting — creates OF + 2 operations in one transaction
		t.Log("creating OF from routing (single transaction)...")
		var fromRoutingResp pbmes.CreateFromRoutingResponse
		natsReq(t, nc, mesdomain.SubjectOFCreateFromRouting, &pbmes.CreateFromRoutingRequest{
			RoutingId: routingID,
			Reference: "OF-E2E-FROM-ROUTING",
			Quantity:  2,
		}, &fromRoutingResp)

		require.NotNil(t, fromRoutingResp.Order)
		fromRoutingOFID = fromRoutingResp.Order.Id
		assert.NotEmpty(t, fromRoutingOFID)

		// ListOperations — expect 2 operations created
		var listOpsResp pbmes.ListOperationsResponse
		natsReq(t, nc, mesdomain.SubjectOperationList, &pbmes.ListOperationsRequest{OfId: fromRoutingOFID}, &listOpsResp)
		require.Len(t, listOpsResp.Operations, 2)

		op1ID = listOpsResp.Operations[0].Id
		op2ID = listOpsResp.Operations[1].Id

		// Verify planned durations were set from routing steps
		assert.Greater(t, listOpsResp.Operations[0].PlannedDurationSeconds, int32(0))
		assert.Greater(t, listOpsResp.Operations[1].PlannedDurationSeconds, int32(0))
	})

	// ── Phase 3 — Operation lifecycle + cycle time ────────────────────────────
	t.Run("Phase_3_Operation_lifecycle", func(t *testing.T) {
		require.NotEmpty(t, op1ID, "op1ID must be set from Phase 2")

		// Capture operation.started event
		startedCh := captureEvent(t, nc, mesdomain.SubjectOperationStarted)
		completedCh := captureEvent(t, nc, mesdomain.SubjectOperationCompleted)

		t.Log("starting operation 1...")
		var startResp pbmes.StartOperationResponse
		natsReq(t, nc, mesdomain.SubjectOperationStart, &pbmes.StartOperationRequest{
			OperationId:   op1ID,
			OperatorId:    "00000000-0000-0000-0000-000000000001",
			OperatorRoles: []string{"kors-operateur"},
		}, &startResp)

		require.NotNil(t, startResp.Operation)
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_IN_PROGRESS, startResp.Operation.Status)

		// Wait for started event (published by outbox worker)
		startedData := waitEvent(t, startedCh, 5*time.Second, "kors.mes.operation.started")
		var startedEvt pbmes.OperationStartedEvent
		require.NoError(t, proto.Unmarshal(startedData, &startedEvt))
		assert.Equal(t, op1ID, startedEvt.OperationId)

		// Small delay to ensure actual_duration_seconds > 0
		time.Sleep(50 * time.Millisecond)

		t.Log("completing operation 1...")
		var completeResp pbmes.CompleteOperationResponse
		natsReq(t, nc, mesdomain.SubjectOperationComplete, &pbmes.CompleteOperationRequest{
			OperationId: op1ID,
		}, &completeResp)

		require.NotNil(t, completeResp.Operation)
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_COMPLETED, completeResp.Operation.Status)

		// Wait for completed event
		completedData := waitEvent(t, completedCh, 5*time.Second, "kors.mes.operation.completed")
		var completedEvt pbmes.OperationCompletedEvent
		require.NoError(t, proto.Unmarshal(completedData, &completedEvt))
		assert.Equal(t, op1ID, completedEvt.OperationId)
		assert.GreaterOrEqual(t, completedEvt.PlannedDurationSeconds, int32(600), "planned duration from routing step")
	})

	// ── Phase 4 — NC declaration + QMS subscriber (cross-service) ─────────────
	var ncID string
	t.Run("Phase_4_NC_declaration_QMS_subscriber", func(t *testing.T) {
		require.NotEmpty(t, op2ID, "op2ID must be set from Phase 2")

		// Subscribe to MES nc.declared event before triggering
		ncDeclaredCh := captureEvent(t, nc, mesdomain.SubjectNCDeclared)

		t.Log("starting operation 2 before declaring NC...")
		var startResp pbmes.StartOperationResponse
		natsReq(t, nc, mesdomain.SubjectOperationStart, &pbmes.StartOperationRequest{
			OperationId:   op2ID,
			OperatorId:    "00000000-0000-0000-0000-000000000001",
			OperatorRoles: []string{"kors-operateur"},
		}, &startResp)
		assert.Equal(t, pbmes.OperationStatus_OPERATION_STATUS_IN_PROGRESS, startResp.Operation.Status)

		t.Log("declaring NC on operation 2...")
		var declareResp pbmes.DeclareNCResponse
		natsReq(t, nc, mesdomain.SubjectOperationDeclareNC, &pbmes.DeclareNCRequest{
			OperationId:      op2ID,
			OfId:             fromRoutingOFID,
			DefectCode:       "SURFACE_DEFECT",
			Description:      "scratch detected during assembly",
			AffectedQuantity: 1,
			DeclaredBy:       "operator-uuid",
		}, &declareResp)
		assert.NotEmpty(t, declareResp.EventId, "NC event ID must be returned")

		// Wait for MES nc.declared event
		ncDeclaredData := waitEvent(t, ncDeclaredCh, 5*time.Second, "kors.mes.nc.declared")
		var ncDeclEvt pbmes.NCDeclaredEvent
		require.NoError(t, proto.Unmarshal(ncDeclaredData, &ncDeclEvt))
		assert.Equal(t, op2ID, ncDeclEvt.OperationId)
		assert.Equal(t, "SURFACE_DEFECT", ncDeclEvt.DefectCode)

		// Wait for QMS subscriber to create the NC (async — poll with timeout)
		t.Log("waiting for QMS subscriber to create NC...")
		qmsR := qmsrepo.New(qmsPool)
		require.Eventually(t, func() bool {
			nc2, err := qmsR.FindNCByOperationID(context.Background(), op2ID)
			if err != nil {
				return false
			}
			ncID = nc2.ID
			return true
		}, 8*time.Second, 200*time.Millisecond, "QMS subscriber did not create NC from MES event")

		assert.NotEmpty(t, ncID)

		// GetNC via NATS
		var getNCResp pbqms.GetNCResponse
		natsReq(t, nc, qmsdomain.SubjectNCGet, &pbqms.GetNCRequest{Id: ncID}, &getNCResp)
		assert.Equal(t, ncID, getNCResp.Nc.Id)
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_OPEN, getNCResp.Nc.Status)
		assert.Equal(t, "SURFACE_DEFECT", getNCResp.Nc.DefectCode)

		// Idempotency: re-declare NC for the same operation — must not create a duplicate
		t.Log("verifying idempotency (re-declare same operation)...")
		var declareResp2 pbmes.DeclareNCResponse
		natsReq(t, nc, mesdomain.SubjectOperationDeclareNC, &pbmes.DeclareNCRequest{
			OperationId:      op2ID,
			OfId:             fromRoutingOFID,
			DefectCode:       "SURFACE_DEFECT",
			Description:      "duplicate declaration",
			AffectedQuantity: 1,
			DeclaredBy:       "operator-uuid",
		}, &declareResp2)

		// Wait a moment then verify still exactly 1 NC for this operation
		time.Sleep(500 * time.Millisecond)
		ncAgain, err := qmsR.FindNCByOperationID(context.Background(), op2ID)
		require.NoError(t, err)
		assert.Equal(t, ncID, ncAgain.ID, "duplicate NC must not have been created")
	})

	// ── Phase 5 — QMS NC lifecycle ────────────────────────────────────────────
	t.Run("Phase_5_QMS_NC_lifecycle", func(t *testing.T) {
		require.NotEmpty(t, ncID, "ncID must be set from Phase 4")

		// OPEN → UNDER_ANALYSIS
		var analyseResp pbqms.StartAnalysisResponse
		natsReq(t, nc, qmsdomain.SubjectNCAnalyse, &pbqms.StartAnalysisRequest{
			NcId:     ncID,
			AnalystId: "analyst-uuid",
		}, &analyseResp)
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_UNDER_ANALYSIS, analyseResp.Nc.Status)

		// UNDER_ANALYSIS → PENDING_DISPOSITION
		var dispResp pbqms.ProposeDispositionResponse
		natsReq(t, nc, qmsdomain.SubjectNCProposeDisposition, &pbqms.ProposeDispositionRequest{
			NcId:        ncID,
			Disposition: pbqms.NCDisposition_NC_DISPOSITION_SCRAP,
			AnalystId:  "analyst-uuid",
		}, &dispResp)
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_PENDING_DISPOSITION, dispResp.Nc.Status)
		assert.Equal(t, pbqms.NCDisposition_NC_DISPOSITION_SCRAP, dispResp.Nc.Disposition)

		// Capture nc.closed event
		ncClosedCh := captureEvent(t, nc, qmsdomain.SubjectNCClosed)

		// PENDING_DISPOSITION → CLOSED
		var closeResp pbqms.CloseNCResponse
		natsReq(t, nc, qmsdomain.SubjectNCClose, &pbqms.CloseNCRequest{
			NcId:     ncID,
			ClosedBy: "manager-uuid",
		}, &closeResp)
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_CLOSED, closeResp.Nc.Status)
		assert.Equal(t, "manager-uuid", closeResp.Nc.ClosedBy)
		assert.NotNil(t, closeResp.Nc.ClosedAt)

		// Wait for nc.closed event
		closedData := waitEvent(t, ncClosedCh, 5*time.Second, "kors.qms.nc.closed")
		var closedEvt pbqms.NCClosedEvent
		require.NoError(t, proto.Unmarshal(closedData, &closedEvt))
		assert.Equal(t, ncID, closedEvt.NcId)
		assert.Equal(t, pbqms.NCDisposition_NC_DISPOSITION_SCRAP, closedEvt.Disposition)
	})

	// ── Phase 6 — CAPA lifecycle ──────────────────────────────────────────────
	t.Run("Phase_6_CAPA_lifecycle", func(t *testing.T) {
		require.NotEmpty(t, ncID, "ncID must be set from Phase 4")

		// Capture capa.created event
		capaCreatedCh := captureEvent(t, nc, qmsdomain.SubjectCAPACreated)

		// Create CAPA
		var createCapaResp pbqms.CreateCAPAResponse
		natsReq(t, nc, qmsdomain.SubjectCAPACreate, &pbqms.CreateCAPARequest{
			NcId:        ncID,
			ActionType:  pbqms.CAPAActionType_CAPA_ACTION_TYPE_CORRECTIVE,
			Description: "Replace defective component and update inspection checklist",
			OwnerId:     "quality-engineer-uuid",
		}, &createCapaResp)

		require.NotNil(t, createCapaResp.Capa)
		capaID := createCapaResp.Capa.Id
		assert.NotEmpty(t, capaID)
		assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_OPEN, createCapaResp.Capa.Status)

		// Wait for capa.created event
		capaCreatedData := waitEvent(t, capaCreatedCh, 5*time.Second, "kors.qms.capa.created")
		var capaEvt pbqms.CAPACreatedEvent
		require.NoError(t, proto.Unmarshal(capaCreatedData, &capaEvt))
		assert.Equal(t, capaID, capaEvt.CapaId)

		// OPEN → IN_PROGRESS
		var startCapaResp pbqms.StartCAPAResponse
		natsReq(t, nc, qmsdomain.SubjectCAPAStart, &pbqms.StartCAPARequest{CapaId: capaID}, &startCapaResp)
		assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_IN_PROGRESS, startCapaResp.Capa.Status)

		// IN_PROGRESS → COMPLETED
		var completeCapaResp pbqms.CompleteCAPAResponse
		natsReq(t, nc, qmsdomain.SubjectCAPAComplete, &pbqms.CompleteCAPARequest{CapaId: capaID}, &completeCapaResp)
		assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_COMPLETED, completeCapaResp.Capa.Status)
		assert.NotNil(t, completeCapaResp.Capa.CompletedAt, "completed_at must be set")

		// GetCAPA — verify final state
		var getCapaResp pbqms.GetCAPAResponse
		natsReq(t, nc, qmsdomain.SubjectCAPAGet, &pbqms.GetCAPARequest{Id: capaID}, &getCapaResp)
		assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_COMPLETED, getCapaResp.Capa.Status)
	})

	// ── Phase 7 — Outbox validation ───────────────────────────────────────────
	t.Run("Phase_7_Outbox_validation", func(t *testing.T) {
		// Wait for both outbox workers to drain all pending events
		mesR := mesrepo.New(mesPool)
		qmsR := qmsrepo.New(qmsPool)

		require.Eventually(t, func() bool {
			entries, err := mesR.ListUnpublishedOutbox(context.Background(), 100)
			return err == nil && len(entries) == 0
		}, 8*time.Second, 200*time.Millisecond, "MES outbox not drained")

		require.Eventually(t, func() bool {
			entries, err := qmsR.ListUnpublishedOutbox(context.Background(), 100)
			return err == nil && len(entries) == 0
		}, 8*time.Second, 200*time.Millisecond, "QMS outbox not drained")

		t.Log("✓ MES outbox drained")
		t.Log("✓ QMS outbox drained")
	})
}
