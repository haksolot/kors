package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	mesdomain "github.com/haksolot/kors/services/mes/domain"
)

// ── CSV Import/Export (§14) ──────────────────────────────────────────────────

func (h *Handler) exportWorkstationsCSV(w http.ResponseWriter, r *http.Request) {
	var resp pbmes.ListWorkstationsResponse
	// Get first 1000 workstations
	req := &pbmes.ListWorkstationsRequest{Limit: 1000, Offset: 0}
	if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationList, req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=workstations.csv")

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"ID", "Name", "Description", "Capacity", "NominalRate", "Status"})

	for _, ws := range resp.Workstations {
		_ = writer.Write([]string{
			ws.Id,
			ws.Name,
			ws.Description,
			strconv.Itoa(int(ws.Capacity)),
			fmt.Sprintf("%.2f", ws.NominalRate),
			ws.Status.String(),
		})
	}
	writer.Flush()
}

func (h *Handler) importWorkstationsCSV(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	// Skip header
	if _, err := csvReader.Read(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid csv")
		return
	}

	count := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		// Header: ID, Name, Description, Capacity, NominalRate, Status
		if len(record) < 5 {
			continue
		}

		capacity, _ := strconv.Atoi(record[3])
		nominalRate, _ := strconv.ParseFloat(record[4], 64)

		req := &pbmes.CreateWorkstationRequest{
			Name:        record[1],
			Description: record[2],
			Capacity:    int32(capacity),
			NominalRate: nominalRate,
		}

		var resp pbmes.CreateWorkstationResponse
		if err := h.natsReq(r.Context(), mesdomain.SubjectWorkstationCreate, req, &resp); err != nil {
			h.log.Warn().Err(err).Str("ws_name", req.Name).Msg("failed to import workstation")
			continue
		}
		count++
	}

	writeRawJSON(w, http.StatusOK, map[string]interface{}{"imported": count})
}

func (h *Handler) exportToolsCSV(w http.ResponseWriter, r *http.Request) {
	var resp pbmes.ListToolsResponse
	req := &pbmes.ListToolsRequest{Limit: 1000, Offset: 0}
	if err := h.natsReq(r.Context(), mesdomain.SubjectToolList, req, &resp); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=tools.csv")

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"ID", "Name", "SerialNumber", "Status", "MaxCycles", "CurrentCycles", "LastCalibration"})

	for _, t := range resp.Tools {
		_ = writer.Write([]string{
			t.Id,
			t.Name,
			t.SerialNumber,
			t.Status.String(),
			strconv.Itoa(int(t.MaxCycles)),
			strconv.Itoa(int(t.CurrentCycles)),
			t.LastCalibrationAt.AsTime().Format("2006-01-02"),
		})
	}
	writer.Flush()
}
