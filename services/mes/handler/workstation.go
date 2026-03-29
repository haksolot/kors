package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/haksolot/kors/libs/core"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

// ── Workstations (BLOC 6) ─────────────────────────────────────────────────────

// CreateWorkstation handles kors.mes.workstation.create.
func (h *Handler) CreateWorkstation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateWorkstation")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateWorkstationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectWorkstationCreate, start, fmt.Errorf("CreateWorkstation: unmarshal: %w", err))
	}

	ws, err := domain.NewWorkstation(req.Name, req.Description, int(req.Capacity), req.NominalRate)
	if err != nil {
		return h.fail(domain.SubjectWorkstationCreate, start, fmt.Errorf("CreateWorkstation: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.WorkstationCreatedEvent{
		EventId:       uuid.NewString(),
		WorkstationId: ws.ID,
		Name:          ws.Name,
		Capacity:      int32(ws.Capacity),
		NominalRate:   ws.NominalRate,
		CreatedAt:     timestamppb.New(ws.CreatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectWorkstationCreate, start, fmt.Errorf("CreateWorkstation: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveWorkstation(ctx, ws); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "WorkstationCreated",
			Subject:   domain.SubjectWorkstationCreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectWorkstationCreate, start, fmt.Errorf("CreateWorkstation: tx: %w", err))
	}

	h.log.Info().Str("workstation_id", ws.ID).Str("name", ws.Name).Msg("workstation created")
	h.succeed(domain.SubjectWorkstationCreate, start)
	return proto.Marshal(&pbmes.CreateWorkstationResponse{Workstation: workstationToProto(ws)})
}

// GetWorkstation handles kors.mes.workstation.get.
func (h *Handler) GetWorkstation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetWorkstation")
	defer span.End()
	start := time.Now()

	var req pbmes.GetWorkstationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectWorkstationGet, start, fmt.Errorf("GetWorkstation: unmarshal: %w", err))
	}

	ws, err := h.workstations.FindWorkstationByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectWorkstationGet, start, fmt.Errorf("GetWorkstation: %w", err))
	}

	h.succeed(domain.SubjectWorkstationGet, start)
	return proto.Marshal(&pbmes.GetWorkstationResponse{Workstation: workstationToProto(ws)})
}

// ListWorkstations handles kors.mes.workstation.list.
func (h *Handler) ListWorkstations(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListWorkstations")
	defer span.End()
	start := time.Now()

	var req pbmes.ListWorkstationsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectWorkstationList, start, fmt.Errorf("ListWorkstations: unmarshal: %w", err))
	}

	workstations, err := h.workstations.ListWorkstations(ctx, int(req.Limit), int(req.Offset))
	if err != nil {
		return h.fail(domain.SubjectWorkstationList, start, fmt.Errorf("ListWorkstations: %w", err))
	}

	pbWorkstations := make([]*pbmes.Workstation, 0, len(workstations))
	for _, ws := range workstations {
		pbWorkstations = append(pbWorkstations, workstationToProto(ws))
	}
	h.succeed(domain.SubjectWorkstationList, start)
	return proto.Marshal(&pbmes.ListWorkstationsResponse{Workstations: pbWorkstations})
}

// UpdateWorkstationStatus handles kors.mes.workstation.update_status.
func (h *Handler) UpdateWorkstationStatus(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.UpdateWorkstationStatus")
	defer span.End()
	start := time.Now()

	var req pbmes.UpdateWorkstationStatusRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectWorkstationUpdateStatus, start, fmt.Errorf("UpdateWorkstationStatus: unmarshal: %w", err))
	}

	ws, err := h.workstations.FindWorkstationByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectWorkstationUpdateStatus, start, fmt.Errorf("UpdateWorkstationStatus: find: %w", err))
	}

	oldStatus := string(ws.Status)
	newStatus := protoWorkstationStatusToDomain(req.Status)

	if err := ws.UpdateStatus(newStatus); err != nil {
		return h.fail(domain.SubjectWorkstationUpdateStatus, start, fmt.Errorf("UpdateWorkstationStatus: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.WorkstationStatusChangedEvent{
		EventId:       uuid.NewString(),
		WorkstationId: ws.ID,
		OldStatus:     oldStatus,
		NewStatus:     string(newStatus),
		ChangedAt:     timestamppb.New(ws.UpdatedAt),
	})
	if err != nil {
		return h.fail(domain.SubjectWorkstationUpdateStatus, start, fmt.Errorf("UpdateWorkstationStatus: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateWorkstation(ctx, ws); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "WorkstationStatusChanged",
			Subject:   domain.SubjectWorkstationStatusChanged,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectWorkstationUpdateStatus, start, fmt.Errorf("UpdateWorkstationStatus: tx: %w", err))
	}

	h.log.Info().Str("workstation_id", ws.ID).Str("status", string(ws.Status)).Msg("workstation status updated")
	h.succeed(domain.SubjectWorkstationUpdateStatus, start)
	return proto.Marshal(&pbmes.UpdateWorkstationStatusResponse{Workstation: workstationToProto(ws)})
}

// ── Converters ────────────────────────────────────────────────────────────────

func workstationToProto(w *domain.Workstation) *pbmes.Workstation {
	return &pbmes.Workstation{
		Id:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		Capacity:    int32(w.Capacity),
		NominalRate: w.NominalRate,
		Status:      domainWorkstationStatusToProto(w.Status),
		CreatedAt:   timestamppb.New(w.CreatedAt),
		UpdatedAt:   timestamppb.New(w.UpdatedAt),
	}
}

func domainWorkstationStatusToProto(s domain.WorkstationStatus) pbmes.WorkstationStatus {
	switch s {
	case domain.WorkstationStatusAvailable:
		return pbmes.WorkstationStatus_WORKSTATION_STATUS_AVAILABLE
	case domain.WorkstationStatusInProduction:
		return pbmes.WorkstationStatus_WORKSTATION_STATUS_IN_PRODUCTION
	case domain.WorkstationStatusDown:
		return pbmes.WorkstationStatus_WORKSTATION_STATUS_DOWN
	case domain.WorkstationStatusMaintenance:
		return pbmes.WorkstationStatus_WORKSTATION_STATUS_MAINTENANCE
	default:
		return pbmes.WorkstationStatus_WORKSTATION_STATUS_UNSPECIFIED
	}
}

func protoWorkstationStatusToDomain(s pbmes.WorkstationStatus) domain.WorkstationStatus {
	switch s {
	case pbmes.WorkstationStatus_WORKSTATION_STATUS_AVAILABLE:
		return domain.WorkstationStatusAvailable
	case pbmes.WorkstationStatus_WORKSTATION_STATUS_IN_PRODUCTION:
		return domain.WorkstationStatusInProduction
	case pbmes.WorkstationStatus_WORKSTATION_STATUS_DOWN:
		return domain.WorkstationStatusDown
	case pbmes.WorkstationStatus_WORKSTATION_STATUS_MAINTENANCE:
		return domain.WorkstationStatusMaintenance
	default:
		return domain.WorkstationStatusAvailable
	}
}
