package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pbqms "github.com/haksolot/kors/proto/gen/qms"
	"github.com/haksolot/kors/services/qms/domain"
)

// ── GetNC ─────────────────────────────────────────────────────────────────────

func TestHandler_GetNC(t *testing.T) {
	t.Run("existing NC is returned", func(t *testing.T) {
		nc, _ := domain.NewNC("op-1", "of-1", "DEF-001", "scratch", 1, nil, "op-uuid")
		ncs := &mockNCRepo{}
		ncs.On("FindNCByID", mock.Anything, nc.ID).Return(nc, nil)

		h := newTestHandler(ncs, &mockCAPARepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbqms.GetNCRequest{Id: nc.ID})

		resp, err := h.GetNC(context.Background(), payload)
		require.NoError(t, err)

		var response pbqms.GetNCResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, nc.ID, response.Nc.Id)
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_OPEN, response.Nc.Status)
		ncs.AssertExpectations(t)
	})

	t.Run("not found returns error", func(t *testing.T) {
		ncs := &mockNCRepo{}
		ncs.On("FindNCByID", mock.Anything, "missing").Return(nil, domain.ErrNCNotFound)

		h := newTestHandler(ncs, &mockCAPARepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbqms.GetNCRequest{Id: "missing"})

		resp, err := h.GetNC(context.Background(), payload)
		require.Error(t, err)
		assert.Nil(t, resp)
	})
}

// ── ListNCs ───────────────────────────────────────────────────────────────────

func TestHandler_ListNCs(t *testing.T) {
	t.Run("returns all NCs when no filter", func(t *testing.T) {
		nc1, _ := domain.NewNC("op-1", "of-1", "DEF-001", "", 1, nil, "op-uuid")
		nc2, _ := domain.NewNC("op-2", "of-1", "DEF-002", "", 2, nil, "op-uuid")

		ncs := &mockNCRepo{}
		ncs.On("ListNCs", mock.Anything, domain.ListNCsFilter{}).Return([]*domain.NonConformity{nc1, nc2}, nil)

		h := newTestHandler(ncs, &mockCAPARepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbqms.ListNCsRequest{})

		resp, err := h.ListNCs(context.Background(), payload)
		require.NoError(t, err)

		var response pbqms.ListNCsResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Len(t, response.Ncs, 2)
	})
}

// ── StartAnalysis ─────────────────────────────────────────────────────────────

func TestHandler_StartAnalysis(t *testing.T) {
	t.Run("transitions OPEN to UNDER_ANALYSIS", func(t *testing.T) {
		nc, _ := domain.NewNC("op-1", "of-1", "DEF-001", "scratch", 1, nil, "op-uuid")
		ncs := &mockNCRepo{}
		ncs.On("FindNCByID", mock.Anything, nc.ID).Return(nc, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateNC", mock.Anything, mock.AnythingOfType("*domain.NonConformity")).Return(nil)

		h := newTestHandler(ncs, &mockCAPARepo{}, store)
		payload, _ := proto.Marshal(&pbqms.StartAnalysisRequest{NcId: nc.ID, AnalystId: "analyst-uuid"})

		resp, err := h.StartAnalysis(context.Background(), payload)
		require.NoError(t, err)

		var response pbqms.StartAnalysisResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_UNDER_ANALYSIS, response.Nc.Status)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})

	t.Run("invalid transition returns error without tx", func(t *testing.T) {
		nc, _ := domain.NewNC("op-1", "of-1", "DEF-001", "", 1, nil, "op-uuid")
		// move to under_analysis first
		_ = nc.StartAnalysis("analyst-1")

		ncs := &mockNCRepo{}
		ncs.On("FindNCByID", mock.Anything, nc.ID).Return(nc, nil)

		h := newTestHandler(ncs, &mockCAPARepo{}, newMockTransactor())
		payload, _ := proto.Marshal(&pbqms.StartAnalysisRequest{NcId: nc.ID, AnalystId: "analyst-uuid"})

		resp, err := h.StartAnalysis(context.Background(), payload)
		require.Error(t, err)
		assert.Nil(t, resp)
	})
}

// ── CloseNC ───────────────────────────────────────────────────────────────────

func TestHandler_CloseNC(t *testing.T) {
	t.Run("closes NC and writes outbox", func(t *testing.T) {
		nc, _ := domain.NewNC("op-1", "of-1", "DEF-001", "scratch", 1, nil, "op-uuid")
		_ = nc.StartAnalysis("analyst-1")
		_ = nc.ProposeDisposition(domain.NCDispositionScrap, "analyst-1")

		ncs := &mockNCRepo{}
		ncs.On("FindNCByID", mock.Anything, nc.ID).Return(nc, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateNC", mock.Anything, mock.AnythingOfType("*domain.NonConformity")).Return(nil)
		store.Ops.On("InsertOutbox", mock.Anything, "NCClosed").Return(nil)

		h := newTestHandler(ncs, &mockCAPARepo{}, store)
		payload, _ := proto.Marshal(&pbqms.CloseNCRequest{NcId: nc.ID, ClosedBy: "manager-uuid"})

		resp, err := h.CloseNC(context.Background(), payload)
		require.NoError(t, err)

		var response pbqms.CloseNCResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbqms.NCStatus_NC_STATUS_CLOSED, response.Nc.Status)
		assert.Equal(t, "manager-uuid", response.Nc.ClosedBy)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})
}

// ── CreateCAPA ────────────────────────────────────────────────────────────────

func TestHandler_CreateCAPA(t *testing.T) {
	tests := []struct {
		name      string
		req       *pbqms.CreateCAPARequest
		setupMock func(*mockTransactor)
		wantErr   bool
	}{
		{
			name: "valid corrective CAPA is created",
			req: &pbqms.CreateCAPARequest{
				NcId:        "nc-uuid",
				ActionType:  pbqms.CAPAActionType_CAPA_ACTION_TYPE_CORRECTIVE,
				Description: "Fix the defect",
				OwnerId:     "owner-uuid",
			},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(nil)
				st.Ops.On("SaveCAPA", mock.Anything, mock.AnythingOfType("*domain.CAPA")).Return(nil)
				st.Ops.On("InsertOutbox", mock.Anything, "CAPACreated").Return(nil)
			},
		},
		{
			name: "empty nc_id returns error",
			req: &pbqms.CreateCAPARequest{
				ActionType:  pbqms.CAPAActionType_CAPA_ACTION_TYPE_CORRECTIVE,
				Description: "Fix the defect",
				OwnerId:     "owner-uuid",
			},
			setupMock: func(_ *mockTransactor) {},
			wantErr:   true,
		},
		{
			name: "db error is propagated",
			req: &pbqms.CreateCAPARequest{
				NcId:        "nc-uuid",
				ActionType:  pbqms.CAPAActionType_CAPA_ACTION_TYPE_PREVENTIVE,
				Description: "Prevent recurrence",
				OwnerId:     "owner-uuid",
			},
			setupMock: func(st *mockTransactor) {
				st.On("WithTx", mock.Anything).Return(errDB)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockTransactor()
			tc.setupMock(store)

			h := newTestHandler(&mockNCRepo{}, &mockCAPARepo{}, store)
			payload, _ := proto.Marshal(tc.req)

			resp, err := h.CreateCAPA(context.Background(), payload)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)

			var response pbqms.CreateCAPAResponse
			require.NoError(t, proto.Unmarshal(resp, &response))
			assert.Equal(t, tc.req.NcId, response.Capa.NcId)
			assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_OPEN, response.Capa.Status)
			store.AssertExpectations(t)
			store.Ops.AssertExpectations(t)
		})
	}
}

// ── StartCAPA ─────────────────────────────────────────────────────────────────

func TestHandler_StartCAPA(t *testing.T) {
	t.Run("transitions OPEN to IN_PROGRESS", func(t *testing.T) {
		capa, _ := domain.NewCAPA("nc-uuid", domain.CAPAActionCorrective, "Fix it", "owner-uuid", nil)

		capas := &mockCAPARepo{}
		capas.On("FindCAPAByID", mock.Anything, capa.ID).Return(capa, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateCAPA", mock.Anything, mock.AnythingOfType("*domain.CAPA")).Return(nil)

		h := newTestHandler(&mockNCRepo{}, capas, store)
		payload, _ := proto.Marshal(&pbqms.StartCAPARequest{CapaId: capa.ID})

		resp, err := h.StartCAPA(context.Background(), payload)
		require.NoError(t, err)

		var response pbqms.StartCAPAResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_IN_PROGRESS, response.Capa.Status)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})
}

// ── CompleteCAPA ──────────────────────────────────────────────────────────────

func TestHandler_CompleteCAPA(t *testing.T) {
	t.Run("transitions IN_PROGRESS to COMPLETED", func(t *testing.T) {
		capa, _ := domain.NewCAPA("nc-uuid", domain.CAPAActionCorrective, "Fix it", "owner-uuid", nil)
		_ = capa.Start()

		capas := &mockCAPARepo{}
		capas.On("FindCAPAByID", mock.Anything, capa.ID).Return(capa, nil)

		store := newMockTransactor()
		store.On("WithTx", mock.Anything).Return(nil)
		store.Ops.On("UpdateCAPA", mock.Anything, mock.AnythingOfType("*domain.CAPA")).Return(nil)

		h := newTestHandler(&mockNCRepo{}, capas, store)
		payload, _ := proto.Marshal(&pbqms.CompleteCAPARequest{CapaId: capa.ID})

		resp, err := h.CompleteCAPA(context.Background(), payload)
		require.NoError(t, err)

		var response pbqms.CompleteCAPAResponse
		require.NoError(t, proto.Unmarshal(resp, &response))
		assert.Equal(t, pbqms.CAPAStatus_CAPA_STATUS_COMPLETED, response.Capa.Status)
		assert.NotNil(t, response.Capa.CompletedAt)
		store.AssertExpectations(t)
		store.Ops.AssertExpectations(t)
	})
}
