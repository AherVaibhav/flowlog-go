package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flowlog/service/api/handler"
)

const sampleLog = `2 123456789012 eni-abc11111 10.0.1.5 172.16.0.50 443 52314 6 12 4800 1620000001 1620000061 ACCEPT OK
2 123456789012 eni-abc22222 10.0.1.5 8.8.8.8 53 34210 17 1 76 1620000010 1620000011 ACCEPT OK
2 123456789012 eni-abc33333 10.0.1.5 8.8.8.8 53 41022 17 1 76 1620000015 1620000016 ACCEPT OK
2 123456789012 eni-abc44444 10.0.1.5 172.16.0.50 443 60001 6 8 3200 1620000020 1620000080 ACCEPT OK
2 123456789012 eni-abc55555 10.0.1.5 198.51.100.3 54321 443 6 8 3840 1620000090 1620000100 ACCEPT OK
2 123456789012 eni-abc66666 10.0.1.5 10.0.2.9 8080 55123 6 7 3360 1620000050 1620000060 ACCEPT OK
2 123456789012 eni-def11111 192.168.0.10 10.0.1.5 80 8080 6 5 300 1620000020 1620000025 ACCEPT OK
2 123456789012 eni-def22222 172.16.0.1 10.0.1.5 3306 45678 6 100 51200 1620000060 1620000120 ACCEPT OK
2 123456789012 eni-def33333 198.51.100.3 10.0.1.5 443 54321 6 8 3840 1620000080 1620000090 REJECT OK
2 123456789012 eni-def44444 203.0.113.7 10.0.1.5 22 61000 6 3 180 1620000030 1620000035 REJECT OK
2 123456789012 eni-ghi11111 203.0.113.7 172.16.0.50 22 1024 6 3 180 1620000030 1620000035 REJECT OK
2 123456789012 eni-ghi22222 10.0.1.8 10.0.2.9 443 60000 6 20 9600 1620000040 1620000100 ACCEPT OK
2 123456789012 eni-ghi33333 10.0.1.8 10.0.2.9 443 60001 6 18 8640 1620000045 1620000105 ACCEPT OK
2 123456789012 eni-ghi44444 10.0.1.8 10.0.2.9 443 60002 6 22 10560 1620000050 1620000110 ACCEPT OK
2 123456789012 eni-ghi55555 10.0.0.1 192.168.1.1 514 514 17 2 160 1620000070 1620000071 ACCEPT OK
2 123456789012 eni-jkl11111 10.0.2.9 10.0.1.5 8443 49200 6 15 7200 1620000100 1620000160 ACCEPT OK
2 123456789012 eni-jkl22222 10.0.2.9 10.0.1.5 8443 49201 6 12 5760 1620000105 1620000165 ACCEPT OK
2 123456789012 eni-jkl33333 10.0.3.1 8.8.4.4 53 33001 17 1 76 1620000110 1620000111 ACCEPT OK
2 123456789012 eni-jkl44444 10.0.3.1 8.8.4.4 53 33002 17 1 76 1620000112 1620000113 ACCEPT OK
2 123456789012 eni-jkl55555 10.0.3.1 8.8.4.4 53 33003 17 1 76 1620000114 1620000115 ACCEPT OK
2 123456789012 eni-mno11111 172.16.5.20 10.0.1.5 3389 52000 6 30 14400 1620000120 1620000180 REJECT OK
2 123456789012 eni-mno22222 172.16.5.20 10.0.1.5 3389 52001 6 28 13440 1620000125 1620000185 REJECT OK
2 123456789012 eni-mno33333 10.0.1.5 172.16.5.20 52000 3389 6 30 14400 1620000120 1620000180 ACCEPT OK
2 123456789012 eni-mno44444 - - - - 1 - - 1620000130 1620000131 - NODATA
2 123456789012 eni-mno55555 - - - - 1 - - 1620000135 1620000136 - SKIPDATA`

// helpers
func newHandler() *handler.Handler {
	return handler.New(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func post(t *testing.T, h *handler.Handler, body string, params map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	target := "/api/v1/flowlogs/parse"
	if len(params) > 0 {
		target += "?"
		first := true
		for k, v := range params {
			if !first {
				target += "&"
			}
			target += k + "=" + v
			first = false
		}
	}
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var result map[string]any
	json.Unmarshal(rec.Body.Bytes(), &result)
	return rec, result
}

func toInt(v any) int {
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return -1
}

// health
func TestHealth_Returns200(t *testing.T) {
	mux := http.NewServeMux()
	newHandler().RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "UP" {
		t.Errorf("want status=UP, got %v", body["status"])
	}
}

// no filter
func TestParse_NoFilter_Returns10Records(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	stats := body["stats"].(map[string]any)
	if toInt(stats["totalRecords"]) != 25 {
		t.Errorf("totalRecords: want 25, got %v", stats["totalRecords"])
	}
	if toInt(stats["matchedRecords"]) != 25 {
		t.Errorf("matchedRecords: want 25, got %v", stats["matchedRecords"])
	}
}

// source IP filter
func TestParse_SrcIP_Exact(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"srcIp": "10.0.1.5"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 7 {
		t.Errorf("want 7, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

func TestParse_SrcIP_CIDR(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"srcIp": "10.0.0.0%2F16"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 16 {
		t.Errorf("want 16, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

// sestination IP filter
func TestParse_DstIP_Exact(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"dstIp": "10.0.1.5"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 8 {
		t.Errorf("want 8, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

// port filters
func TestParse_DstPort_Exact(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"dstPort": "443"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 1 {
		t.Errorf("want 1, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

func TestParse_SrcPort_Range(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"srcPort": "400-500"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 6 {
		t.Errorf("want 6, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

// combined filters
func TestParse_SrcIPAndDstIP(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{
		"srcIp": "10.0.1.5",
		"dstIp": "198.51.100.3",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 1 {
		t.Errorf("want 1, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

func TestParse_SrcIPAndDstPort(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{
		"srcIp":   "10.0.1.5",
		"dstPort": "443",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["matchedRecords"]) != 1 {
		t.Errorf("want 1, got %v", body["stats"].(map[string]any)["matchedRecords"])
	}
}

// connection counts
func TestParse_ConnectionCounts_Present(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"srcIp": "10.0.1.5"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	stats := body["stats"].(map[string]any)
	if toInt(stats["distinctConnections"]) != 7 {
		t.Errorf("want 7 distinct connections, got %v", stats["distinctConnections"])
	}
	counts := body["connectionCounts"].([]any)
	if len(counts) != 7 {
		t.Errorf("want 7 connection count entries, got %d", len(counts))
	}
	if counts[0].(map[string]any)["protocolName"] == nil {
		t.Error("protocolName missing from connectionCounts entry")
	}
}

func TestParse_ConnectionCounts_Accumulate(t *testing.T) {
	line := "2 123456789012 eni-x 10.0.1.5 8.8.8.8 443 53 6 1 40 100 160 ACCEPT OK\n"
	rec, body := post(t, newHandler(), strings.Repeat(line, 3), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if toInt(body["stats"].(map[string]any)["distinctConnections"]) != 1 {
		t.Errorf("want 1 distinct connection, got %v", body["stats"].(map[string]any)["distinctConnections"])
	}
	first := body["connectionCounts"].([]any)[0].(map[string]any)
	if toInt(first["count"]) != 3 {
		t.Errorf("want count=3, got %v", first["count"])
	}
}

// filename query param
func TestParse_FilenameParam(t *testing.T) {
	rec, body := post(t, newHandler(), sampleLog, map[string]string{"filename": "myflows.log"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if body["stats"].(map[string]any)["filename"] != "myflows.log" {
		t.Errorf("want filename=myflows.log, got %v", body["stats"].(map[string]any)["filename"])
	}
}

// erros
func TestParse_EmptyBody_400(t *testing.T) {
	rec, _ := post(t, newHandler(), "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestParse_InvalidSrcIp_400(t *testing.T) {
	rec, _ := post(t, newHandler(), sampleLog, map[string]string{"srcIp": "999.999.999.999"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestParse_InvalidPort_400(t *testing.T) {
	rec, _ := post(t, newHandler(), sampleLog, map[string]string{"dstPort": "not-a-port"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestParse_NonAsciiBody_400(t *testing.T) {
	bad := string(append([]byte(sampleLog), 0xFF, '\n'))
	rec, _ := post(t, newHandler(), bad, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// custom-format
func TestParse_CustomFormat(t *testing.T) {
	custom := `version account-id interface-id srcaddr dstaddr srcport dstport protocol packets bytes start end action log-status
3 123456789012 eni-b01 10.0.1.5 172.16.0.50 443 52314 6 12 4800 1620000001 1620000061 ACCEPT OK
3 123456789012 eni-b02 10.0.2.20 8.8.4.4 53 45678 17 1 76 1620000010 1620000011 ACCEPT OK
`
	rec, body := post(t, newHandler(), custom, map[string]string{"srcIp": "10.0.1.5"})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	stats := body["stats"].(map[string]any)
	if toInt(stats["totalRecords"]) != 2 {
		t.Errorf("total: want 2, got %v", stats["totalRecords"])
	}
	if toInt(stats["matchedRecords"]) != 1 {
		t.Errorf("matched: want 1, got %v", stats["matchedRecords"])
	}
}
