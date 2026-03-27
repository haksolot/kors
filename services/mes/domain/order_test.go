package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haksolot/kors/services/mes/domain"
)

func TestNewOrder(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		productID string
		quantity  int
		wantErr   error
	}{
		{
			name:      "valid order",
			reference: "OF-2026-001",
			productID: "prod-uuid-123",
			quantity:  100,
		},
		{
			name:      "empty reference returns error",
			reference: "",
			productID: "prod-uuid-123",
			quantity:  10,
			wantErr:   domain.ErrInvalidReference,
		},
		{
			name:      "empty product ID returns error",
			reference: "OF-2026-002",
			productID: "",
			quantity:  10,
			wantErr:   domain.ErrInvalidProductID,
		},
		{
			name:      "zero quantity returns error",
			reference: "OF-2026-003",
			productID: "prod-uuid-123",
			quantity:  0,
			wantErr:   domain.ErrInvalidQuantity,
		},
		{
			name:      "negative quantity returns error",
			reference: "OF-2026-004",
			productID: "prod-uuid-123",
			quantity:  -5,
			wantErr:   domain.ErrInvalidQuantity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			order, err := domain.NewOrder(tc.reference, tc.productID, tc.quantity)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, order)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, order)
			assert.NotEmpty(t, order.ID)
			assert.Equal(t, tc.reference, order.Reference)
			assert.Equal(t, tc.productID, order.ProductID)
			assert.Equal(t, tc.quantity, order.Quantity)
			assert.Equal(t, domain.OrderStatusPlanned, order.Status)
			assert.False(t, order.CreatedAt.IsZero())
			assert.Nil(t, order.StartedAt)
			assert.Nil(t, order.CompletedAt)
		})
	}
}

func TestOrder_Start(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *domain.Order
		wantErr error
	}{
		{
			name: "planned order can be started",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				return o
			},
		},
		{
			name: "in-progress order cannot be started again",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				return o
			},
			wantErr: domain.ErrOrderAlreadyStarted,
		},
		{
			name: "completed order cannot be started",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				_ = o.Complete()
				return o
			},
			wantErr: domain.ErrInvalidTransition,
		},
		{
			name: "cancelled order cannot be started",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Cancel("test")
				return o
			},
			wantErr: domain.ErrOrderCancelled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			order := tc.setup()
			err := order.Start()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OrderStatusInProgress, order.Status)
			require.NotNil(t, order.StartedAt)
			assert.False(t, order.StartedAt.IsZero())
		})
	}
}

func TestOrder_Complete(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *domain.Order
		wantErr error
	}{
		{
			name: "in-progress order can be completed",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				return o
			},
		},
		{
			name: "planned order cannot be completed",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				return o
			},
			wantErr: domain.ErrOrderNotInProgress,
		},
		{
			name: "already completed order cannot be completed again",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				_ = o.Complete()
				return o
			},
			wantErr: domain.ErrOrderAlreadyComplete,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			order := tc.setup()
			err := order.Complete()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OrderStatusCompleted, order.Status)
			require.NotNil(t, order.CompletedAt)
			assert.False(t, order.CompletedAt.IsZero())
		})
	}
}

func TestOrder_Suspend(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *domain.Order
		reason  string
		wantErr error
	}{
		{
			name: "in-progress order can be suspended",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				return o
			},
			reason: "machine breakdown",
		},
		{
			name: "planned order cannot be suspended",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				return o
			},
			reason:  "some reason",
			wantErr: domain.ErrOrderNotInProgress,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			order := tc.setup()
			err := order.Suspend(tc.reason)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OrderStatusSuspended, order.Status)
		})
	}
}

func TestOrder_Cancel(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *domain.Order
		reason  string
		wantErr error
	}{
		{
			name: "planned order can be cancelled",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				return o
			},
			reason: "client request",
		},
		{
			name: "in-progress order can be cancelled",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				return o
			},
			reason: "machine failure",
		},
		{
			name: "completed order cannot be cancelled",
			setup: func() *domain.Order {
				o, _ := domain.NewOrder("OF-001", "prod-1", 10)
				_ = o.Start()
				_ = o.Complete()
				return o
			},
			reason:  "too late",
			wantErr: domain.ErrInvalidTransition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			order := tc.setup()
			err := order.Cancel(tc.reason)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.OrderStatusCancelled, order.Status)
		})
	}
}

func TestOrder_Resume(t *testing.T) {
	t.Run("suspended order can be resumed", func(t *testing.T) {
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		require.NoError(t, order.Start())
		require.NoError(t, order.Suspend("pause"))
		require.NoError(t, order.Resume())
		assert.Equal(t, domain.OrderStatusInProgress, order.Status)
	})

	t.Run("planned order cannot be resumed", func(t *testing.T) {
		order, _ := domain.NewOrder("OF-001", "prod-1", 10)
		err := order.Resume()
		require.ErrorIs(t, err, domain.ErrInvalidTransition)
	})
}

// TestOrder_UpdatedAt verifies that UpdatedAt is refreshed on each transition.
func TestOrder_UpdatedAt(t *testing.T) {
	order, _ := domain.NewOrder("OF-001", "prod-1", 10)
	before := order.UpdatedAt

	time.Sleep(time.Millisecond)
	require.NoError(t, order.Start())
	assert.True(t, order.UpdatedAt.After(before))
}
