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

// ── Tools & Gauges (BLOC 8) ──────────────────────────────────────────────────

// CreateTool handles kors.mes.tool.create.
func (h *Handler) CreateTool(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CreateTool")
	defer span.End()
	start := time.Now()

	var req pbmes.CreateToolRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectToolCreate, start, fmt.Errorf("CreateTool: unmarshal: %w", err))
	}

	var lastCal, nextCal *time.Time
	if req.LastCalibrationAt != nil {
		t := req.LastCalibrationAt.AsTime()
		lastCal = &t
	}
	if req.NextCalibrationAt != nil {
		t := req.NextCalibrationAt.AsTime()
		nextCal = &t
	}

	tool, err := domain.NewTool(req.SerialNumber, req.Name, req.Description, req.Category, lastCal, nextCal, int(req.MaxCycles))
	if err != nil {
		return h.fail(domain.SubjectToolCreate, start, fmt.Errorf("CreateTool: %w", err))
	}

	evt, err := proto.Marshal(&pbmes.ToolCreatedEvent{
		EventId:      uuid.NewString(),
		ToolId:       tool.ID,
		SerialNumber: tool.SerialNumber,
		Name:         tool.Name,
	})
	if err != nil {
		return h.fail(domain.SubjectToolCreate, start, fmt.Errorf("CreateTool: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveTool(ctx, tool); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "ToolCreated",
			Subject:   domain.SubjectToolCreated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectToolCreate, start, fmt.Errorf("CreateTool: tx: %w", err))
	}

	h.log.Info().Str("tool_id", tool.ID).Str("sn", tool.SerialNumber).Msg("tool created")
	h.succeed(domain.SubjectToolCreate, start)
	return proto.Marshal(&pbmes.CreateToolResponse{Tool: toolToProto(tool)})
}

// GetTool handles kors.mes.tool.get.
func (h *Handler) GetTool(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.GetTool")
	defer span.End()
	start := time.Now()

	var req pbmes.GetToolRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectToolGet, start, fmt.Errorf("GetTool: unmarshal: %w", err))
	}

	tool, err := h.tools.FindToolByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectToolGet, start, fmt.Errorf("GetTool: %w", err))
	}

	h.succeed(domain.SubjectToolGet, start)
	return proto.Marshal(&pbmes.GetToolResponse{Tool: toolToProto(tool)})
}

// ListTools handles kors.mes.tool.list.
func (h *Handler) ListTools(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListTools")
	defer span.End()
	start := time.Now()

	var req pbmes.ListToolsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectToolList, start, fmt.Errorf("ListTools: unmarshal: %w", err))
	}

	tools, err := h.tools.ListTools(ctx, int(req.Limit), int(req.Offset))
	if err != nil {
		return h.fail(domain.SubjectToolList, start, fmt.Errorf("ListTools: %w", err))
	}

	pbTools := make([]*pbmes.Tool, 0, len(tools))
	for _, t := range tools {
		pbTools = append(pbTools, toolToProto(t))
	}
	h.succeed(domain.SubjectToolList, start)
	return proto.Marshal(&pbmes.ListToolsResponse{Tools: pbTools})
}

// CalibrateTool handles kors.mes.tool.calibrate.
func (h *Handler) CalibrateTool(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.CalibrateTool")
	defer span.End()
	start := time.Now()

	var req pbmes.CalibrateToolRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectToolCalibrate, start, fmt.Errorf("CalibrateTool: unmarshal: %w", err))
	}

	tool, err := h.tools.FindToolByID(ctx, req.Id)
	if err != nil {
		return h.fail(domain.SubjectToolCalibrate, start, fmt.Errorf("CalibrateTool: find: %w", err))
	}

	last := req.LastCalibrationAt.AsTime()
	next := req.NextCalibrationAt.AsTime()
	tool.Calibrate(last, next)

	evt, err := proto.Marshal(&pbmes.ToolCalibrationUpdatedEvent{
		EventId:           uuid.NewString(),
		ToolId:            tool.ID,
		NextCalibrationAt: timestamppb.New(next),
		PerformedBy:       req.PerformedBy,
	})
	if err != nil {
		return h.fail(domain.SubjectToolCalibrate, start, fmt.Errorf("CalibrateTool: marshal event: %w", err))
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.UpdateTool(ctx, tool); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "ToolCalibrationUpdated",
			Subject:   domain.SubjectToolCalibrationUpdated,
			Payload:   evt,
		})
	}); err != nil {
		return h.fail(domain.SubjectToolCalibrate, start, fmt.Errorf("CalibrateTool: tx: %w", err))
	}

	h.log.Info().Str("tool_id", tool.ID).Msg("tool calibrated")
	h.succeed(domain.SubjectToolCalibrate, start)
	return proto.Marshal(&pbmes.CalibrateToolResponse{Tool: toolToProto(tool)})
}

// AssignToolToOperation handles kors.mes.tool.assign_to_operation.
func (h *Handler) AssignToolToOperation(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.AssignToolToOperation")
	defer span.End()
	start := time.Now()

	var req pbmes.AssignToolToOperationRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectToolAssignToOperation, start, fmt.Errorf("AssignToolToOperation: unmarshal: %w", err))
	}

	// Verify tool exists and is valid
	tool, err := h.tools.FindToolByID(ctx, req.ToolId)
	if err != nil {
		return h.fail(domain.SubjectToolAssignToOperation, start, fmt.Errorf("AssignToolToOperation: find tool: %w", err))
	}

	now := time.Now().UTC()
	if !tool.IsCalibrationValid(now) {
		return h.fail(domain.SubjectToolAssignToOperation, start, domain.ErrToolExpired)
	}
	if tool.Status == domain.ToolStatusBlocked {
		return h.fail(domain.SubjectToolAssignToOperation, start, domain.ErrToolBlocked)
	}
	if !tool.HasRemainingLife() {
		return h.fail(domain.SubjectToolAssignToOperation, start, domain.ErrToolMaxCyclesReached)
	}

	if err := h.store.WithTx(ctx, func(tx domain.TxOps) error {
		return tx.LinkToolToOperation(ctx, req.OperationId, req.ToolId)
	}); err != nil {
		return h.fail(domain.SubjectToolAssignToOperation, start, fmt.Errorf("AssignToolToOperation: tx: %w", err))
	}

	h.log.Info().Str("operation_id", req.OperationId).Str("tool_id", req.ToolId).Msg("tool assigned to operation")
	h.succeed(domain.SubjectToolAssignToOperation, start)
	return proto.Marshal(&pbmes.AssignToolToOperationResponse{Success: true})
}

// ListOperationTools handles kors.mes.operation.tools.list.
func (h *Handler) ListOperationTools(ctx context.Context, payload []byte) ([]byte, error) {
	ctx, span := core.StartSpan(ctx, "handler.ListOperationTools")
	defer span.End()
	start := time.Now()

	var req pbmes.GetOperationToolsRequest
	if err := proto.Unmarshal(payload, &req); err != nil {
		return h.fail(domain.SubjectOperationToolsList, start, fmt.Errorf("ListOperationTools: unmarshal: %w", err))
	}

	tools, err := h.tools.ListToolsByOperation(ctx, req.OperationId)
	if err != nil {
		return h.fail(domain.SubjectOperationToolsList, start, fmt.Errorf("ListOperationTools: %w", err))
	}

	pbTools := make([]*pbmes.Tool, 0, len(tools))
	for _, t := range tools {
		pbTools = append(pbTools, toolToProto(t))
	}
	h.succeed(domain.SubjectOperationToolsList, start)
	return proto.Marshal(&pbmes.GetOperationToolsResponse{Tools: pbTools})
}

// ── Converters ────────────────────────────────────────────────────────────────

func toolToProto(t *domain.Tool) *pbmes.Tool {
	pb := &pbmes.Tool{
		Id:            t.ID,
		SerialNumber:  t.SerialNumber,
		Name:          t.Name,
		Description:   t.Description,
		Category:      t.Category,
		Status:        domainToolStatusToProto(t.Status),
		CurrentCycles: int32(t.CurrentCycles),
		MaxCycles:     int32(t.MaxCycles),
		CreatedAt:     timestamppb.New(t.CreatedAt),
	}
	if t.LastCalibrationAt != nil {
		pb.LastCalibrationAt = timestamppb.New(*t.LastCalibrationAt)
	}
	if t.NextCalibrationAt != nil {
		pb.NextCalibrationAt = timestamppb.New(*t.NextCalibrationAt)
	}
	return pb
}

func domainToolStatusToProto(s domain.ToolStatus) pbmes.ToolStatus {
	switch s {
	case domain.ToolStatusValid:
		return pbmes.ToolStatus_TOOL_STATUS_VALID
	case domain.ToolStatusExpired:
		return pbmes.ToolStatus_TOOL_STATUS_EXPIRED
	case domain.ToolStatusBlocked:
		return pbmes.ToolStatus_TOOL_STATUS_BLOCKED
	case domain.ToolStatusDecommissioned:
		return pbmes.ToolStatus_TOOL_STATUS_DECOMMISSIONED
	default:
		return pbmes.ToolStatus_TOOL_STATUS_UNSPECIFIED
	}
}
