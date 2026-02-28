package bench

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollectorLogsBodyLimitRejects413(t *testing.T) {
	c := NewCollector(0, "/dev/null")

	bigBody := bytes.NewReader(make([]byte, maxCollectorBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bigBody)
	rr := httptest.NewRecorder()
	c.handleLogs(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
}

func TestCollectorMetricsBodyLimitRejects413(t *testing.T) {
	c := NewCollector(0, "/dev/null")

	bigBody := bytes.NewReader(make([]byte, maxCollectorBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bigBody)
	rr := httptest.NewRecorder()
	c.handleMetrics(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
}

func TestCollectorSmallBodyAccepted(t *testing.T) {
	c := NewCollector(0, "/dev/null")

	body := []byte(`{"resourceLogs":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	c.handleLogs(rr, req)
	if rr.Code == http.StatusRequestEntityTooLarge {
		t.Error("small body should not return 413")
	}
}
