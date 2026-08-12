package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type diagnosticsInspector struct {
	stats     []types.QueueStat
	supported bool
	err       error
}

func (d diagnosticsInspector) CancelTasksForKnowledge(context.Context, string) (int, int, error) {
	return 0, 0, nil
}

func (d diagnosticsInspector) HasQueuedTasksForKnowledge(context.Context, string) (bool, error) {
	return false, nil
}

func (d diagnosticsInspector) QueueStats(context.Context) ([]types.QueueStat, bool, error) {
	return d.stats, d.supported, d.err
}

func (d diagnosticsInspector) WorkerServerStats(context.Context) ([]types.WorkerServerStat, bool, error) {
	return nil, false, nil
}

func TestGetSystemDiagnosticsDoesNotPanicWithPartialDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &SystemHandler{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/admin/diagnostics", nil)

	handler.GetSystemDiagnostics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response SystemDiagnosticsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Checks) == 0 {
		t.Fatal("expected diagnostics checks")
	}
	if response.Status != DiagnosticStatusError {
		t.Fatalf("status = %s, want %s", response.Status, DiagnosticStatusError)
	}
}

func TestGetSystemDiagnosticsWarnsWhenQueuesNeedAttention(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &SystemHandler{
		taskInspector: diagnosticsInspector{
			supported: true,
			stats: []types.QueueStat{{
				Name:     types.QueueDefault,
				Pending:  3,
				Active:   1,
				Retry:    2,
				Archived: 1,
			}},
		},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/admin/diagnostics", nil)

	handler.GetSystemDiagnostics(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response SystemDiagnosticsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var queueCheck *DiagnosticCheck
	for i := range response.Checks {
		if response.Checks[i].Key == "queues" {
			queueCheck = &response.Checks[i]
			break
		}
	}
	if queueCheck == nil {
		t.Fatal("missing queues check")
	}
	if queueCheck.Status != DiagnosticStatusWarning {
		t.Fatalf("queues status = %s, want %s", queueCheck.Status, DiagnosticStatusWarning)
	}
	if queueCheck.Metadata["retry"] != float64(2) || queueCheck.Metadata["archived"] != float64(1) {
		t.Fatalf("queue metadata mismatch: %+v", queueCheck.Metadata)
	}
}
