package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
	"github.com/haksolot/kors/services/mes/handler"
)

func TestHandler_GetSupervisorDashboard(t *testing.T) {
	dashRepo := &mockDashboardRepo{}
	h := newTestHandlerWithDash(dashRepo)

	t.Run("success", func(t *testing.T) {
		snapshot := &domain.SupervisorSnapshot{
			ActiveOrders: []*domain.Order{
				{ID: "of-1", Reference: "OF-001"},
			},
			Workstations: []*domain.WorkstationSnapshot{
				{
					WorkstationID:   "ws-1",
					WorkstationName: "Machine 1",
					Status:          domain.WorkstationStatusInProduction,
					CurrentOFID:     "of-1",
					OEE: domain.OEEData{
						TRS: 0.85,
					},
				},
			},
		}

		dashRepo.On("GetSupervisorSnapshot", mock.Anything).Return(snapshot, nil).Once()

		resp, err := h.GetSupervisorDashboard(context.Background(), nil)
		assert.NoError(t, err)

		var out pbmes.GetSupervisorDashboardResponse
		assert.NoError(t, proto.Unmarshal(resp, &out))
		assert.Len(t, out.ActiveOrders, 1)
		assert.Len(t, out.Workstations, 1)
		assert.Equal(t, 0.85, out.Workstations[0].Trs)
		dashRepo.AssertExpectations(t)
	})
}

func TestHandler_GetTRSByPeriod(t *testing.T) {
	dashRepo := &mockDashboardRepo{}
	h := newTestHandlerWithDash(dashRepo)

	t.Run("success", func(t *testing.T) {
		dashRepo.On("GetTRSByPeriod", mock.Anything, mock.MatchedBy(func(f domain.TRSFilter) bool {
			return f.Granularity == domain.TRSPeriodDay
		})).Return([]*domain.TRSDataPoint{
			{Period: "2026-03-30", TRS: 0.8},
		}, nil).Once()

		req, _ := proto.Marshal(&pbmes.GetTRSByPeriodRequest{
			Granularity: pbmes.TRSPeriodGranularity_TRS_GRANULARITY_DAY,
			From:        timestamppb.New(time.Now().Add(-24 * time.Hour)),
			To:          timestamppb.New(time.Now()),
		})

		resp, err := h.GetTRSByPeriod(context.Background(), req)
		assert.NoError(t, err)

		var out pbmes.GetTRSByPeriodResponse
		assert.NoError(t, proto.Unmarshal(resp, &out))
		assert.Len(t, out.Points, 1)
		assert.Equal(t, 0.8, out.Points[0].Trs)
		dashRepo.AssertExpectations(t)
	})
}

func newTestHandlerWithDash(dash *mockDashboardRepo) *handler.Handler {
	// We need a way to inject our specific dash mock.
	// Let's modify mock_test.go to add another helper or just use the long way.
	return handler.New(
		&mockOrderRepo{},
		&mockOperationRepo{},
		&mockTraceabilityRepo{},
		&mockRoutingRepo{},
		&mockQualificationRepo{},
		&mockWorkstationRepo{},
		&mockTimeTrackingRepo{},
		&mockToolRepo{},
		&mockMaterialRepo{},
		&mockQualityRepo{},
		&mockAlertRepo{},
		&mockAuditRepo{},
		&mockComplianceRepo{},
		dash,
		newMockTransactor(),
		prometheus.NewRegistry(),
		func() *zerolog.Logger { l := zerolog.Nop(); return &l }(),
	)
}
