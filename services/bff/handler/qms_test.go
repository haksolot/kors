package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pbqms "github.com/haksolot/kors/proto/gen/qms"
	qmsdomain "github.com/haksolot/kors/services/qms/domain"
)

func TestHandler_GetNC(t *testing.T) {
	stub := startNATSStub(t)
	defer stub.drain()

	ncID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	stub.handle(t, qmsdomain.SubjectNCGet, &pbqms.GetNCResponse{
		Nc: &pbqms.NonConformity{Id: ncID, Status: pbqms.NCStatus_NC_STATUS_OPEN},
	})

	h := newTestHandler(t, stub.nc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/qms/nc/"+ncID, nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), ncID)
}

func TestHandler_StartAnalysis(t *testing.T) {
	stub := startNATSStub(t)
	defer stub.drain()

	ncID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	var capturedReq pbqms.StartAnalysisRequest
	sub, err := stub.nc.Subscribe(qmsdomain.SubjectNCAnalyse, func(msg *nats.Msg) {
		_ = proto.Unmarshal(msg.Data, &capturedReq)
		b, _ := proto.Marshal(&pbqms.StartAnalysisResponse{
			Nc: &pbqms.NonConformity{Id: ncID, Status: pbqms.NCStatus_NC_STATUS_UNDER_ANALYSIS},
		})
		_ = msg.Respond(b)
	})
	require.NoError(t, err)
	defer sub.Drain() //nolint:errcheck

	h := newTestHandler(t, stub.nc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qms/nc/"+ncID+"/analyse", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// analyst_id injected from JWT claims
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", capturedReq.AnalystId)
	assert.Equal(t, ncID, capturedReq.NcId)
}

func TestHandler_CloseNC(t *testing.T) {
	stub := startNATSStub(t)
	defer stub.drain()

	ncID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	var capturedReq pbqms.CloseNCRequest
	sub, err := stub.nc.Subscribe(qmsdomain.SubjectNCClose, func(msg *nats.Msg) {
		_ = proto.Unmarshal(msg.Data, &capturedReq)
		b, _ := proto.Marshal(&pbqms.CloseNCResponse{
			Nc: &pbqms.NonConformity{Id: ncID, Status: pbqms.NCStatus_NC_STATUS_CLOSED},
		})
		_ = msg.Respond(b)
	})
	require.NoError(t, err)
	defer sub.Drain() //nolint:errcheck

	h := newTestHandler(t, stub.nc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qms/nc/"+ncID+"/close", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// closed_by injected from JWT
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", capturedReq.ClosedBy)
}

func TestHandler_CreateCAPA(t *testing.T) {
	stub := startNATSStub(t)
	defer stub.drain()

	capaID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	ncID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	stub.handle(t, qmsdomain.SubjectCAPACreate, &pbqms.CreateCAPAResponse{
		Capa: &pbqms.CAPA{Id: capaID, NcId: ncID},
	})

	h := newTestHandler(t, stub.nc)
	body := `{"nc_id":"` + ncID + `","action_type":1,"description":"fix tooling","owner_id":"owner-uuid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qms/capa", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), capaID)
}

func TestHandler_ProposeDisposition(t *testing.T) {
	stub := startNATSStub(t)
	defer stub.drain()

	ncID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	var capturedReq pbqms.ProposeDispositionRequest
	sub, err := stub.nc.Subscribe(qmsdomain.SubjectNCProposeDisposition, func(msg *nats.Msg) {
		_ = proto.Unmarshal(msg.Data, &capturedReq)
		b, _ := proto.Marshal(&pbqms.ProposeDispositionResponse{
			Nc: &pbqms.NonConformity{Id: ncID},
		})
		_ = msg.Respond(b)
	})
	require.NoError(t, err)
	defer sub.Drain() //nolint:errcheck

	h := newTestHandler(t, stub.nc)
	body := `{"disposition":1}` // NC_DISPOSITION_REWORK
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qms/nc/"+ncID+"/disposition", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", capturedReq.AnalystId)
}
