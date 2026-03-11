package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flowlog/service/internal/filter"
	"github.com/flowlog/service/internal/model"
	"github.com/flowlog/service/internal/parser"
)

func tmpLog(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const sampleLog = `2 123456789012 eni-001 10.0.1.5 172.16.0.50 443 52314 6 12 4800 1620000001 1620000061 ACCEPT OK
2 123456789012 eni-002 10.0.1.5 8.8.8.8 53 34210 17 1 76 1620000010 1620000011 ACCEPT OK
2 123456789012 eni-003 192.168.0.10 10.0.1.5 80 8080 6 5 300 1620000020 1620000025 ACCEPT OK
2 123456789012 eni-004 203.0.113.7 172.16.0.50 22 1024 6 3 180 1620000030 1620000035 REJECT OK
2 123456789012 eni-005 10.0.1.8 10.0.2.9 443 60000 6 20 9600 1620000040 1620000100 ACCEPT OK
2 123456789012 eni-006 10.0.1.5 10.0.2.9 8080 55123 6 7 3360 1620000050 1620000060 ACCEPT OK
2 123456789012 eni-007 172.16.0.1 10.0.1.5 3306 45678 6 100 51200 1620000060 1620000120 ACCEPT OK
2 123456789012 eni-008 10.0.0.1 192.168.1.1 514 514 17 2 160 1620000070 1620000071 ACCEPT OK
2 123456789012 eni-009 198.51.100.3 10.0.1.5 443 54321 6 8 3840 1620000080 1620000090 REJECT OK
2 123456789012 eni-010 10.0.1.5 198.51.100.3 54321 443 6 8 3840 1620000090 1620000100 ACCEPT OK
`

func parse(t *testing.T, content string, c *filter.Criteria) *model.ParseResult {
	t.Helper()
	p := parser.New()
	r, err := p.ParseReader(strings.NewReader(content), c)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return r
}

func buildCriteria(t *testing.T, fn func(*filter.Builder) *filter.Builder) *filter.Criteria {
	t.Helper()
	c, err := fn(filter.NewBuilder()).Build()
	if err != nil {
		t.Fatalf("unexpected criteria error: %v", err)
	}
	return c
}

func TestValidation_MissingFile(t *testing.T) {
	_, err := parser.New().Parse("/no/such/file.log", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// File validation
func TestValidation_FileTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.log")
	line := []byte("2 123456789012 eni-x 10.0.1.1 10.0.0.1 80 1234 6 1 40 1000 1060 ACCEPT OK\n")
	f, _ := os.Create(path)
	for written := int64(0); written <= 20*1024*1024; written += int64(len(line)) {
		f.Write(line)
	}
	f.Close()
	_, err := parser.New().Parse(path, nil)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestValidation_NonASCII(t *testing.T) {
	base := []byte("2 123456789012 eni-x 10.0.1.1 10.0.0.1 80 1234 6 1 40 1000 1060 ACCEPT OK\nbad ")
	data := append(base, 0xFF, '\n')
	path := filepath.Join(t.TempDir(), "bad.log")
	os.WriteFile(path, data, 0644)
	_, err := parser.New().Parse(path, nil)
	if err == nil {
		t.Fatal("expected error for non-ASCII file")
	}
}

// No filter
func TestNoFilter_AllRecordsReturned(t *testing.T) {
	r := parse(t, sampleLog, nil)
	if r.TotalRecords != 10 {
		t.Errorf("total: want 10, got %d", r.TotalRecords)
	}
	if r.MatchedCount() != 10 {
		t.Errorf("matched: want 10, got %d", r.MatchedCount())
	}
	if r.SkippedRecords != 0 {
		t.Errorf("skipped: want 0, got %d", r.SkippedRecords)
	}
}

// Source IP filters
func TestFilter_SrcIPExact(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcIP("10.0.1.5") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 4 {
		t.Errorf("want 4 matched, got %d", r.MatchedCount())
	}
	for _, rec := range r.MatchedRecords {
		if rec.SrcAddr() != "10.0.1.5" {
			t.Errorf("unexpected srcaddr %q", rec.SrcAddr())
		}
	}
}

func TestFilter_SrcCIDR(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcIP("10.0.0.0/16") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 6 {
		t.Errorf("want 6 matched, got %d", r.MatchedCount())
	}
}

// Destination IP filters
func TestFilter_DstIPExact(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.DstIP("10.0.1.5") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 3 {
		t.Errorf("want 3 matched, got %d", r.MatchedCount())
	}
	for _, rec := range r.MatchedRecords {
		if rec.DstAddr() != "10.0.1.5" {
			t.Errorf("unexpected dstaddr %q", rec.DstAddr())
		}
	}
}

// Source port filters
func TestFilter_SrcPortExact(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcPort("443") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 3 {
		t.Errorf("want 3 matched, got %d", r.MatchedCount())
	}
	for _, rec := range r.MatchedRecords {
		if rec.Get("srcport") != "443" {
			t.Errorf("unexpected srcport %q", rec.Get("srcport"))
		}
	}
}

func TestFilter_SrcPortRange(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcPort("400-500") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 3 {
		t.Errorf("want 3 matched, got %d", r.MatchedCount())
	}
	for _, rec := range r.MatchedRecords {
		p := rec.SrcPortInt()
		if p < 400 || p > 500 {
			t.Errorf("srcport %d out of [400,500]", p)
		}
	}
}

// Destination port filters
func TestFilter_DstPortExact(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.DstPort("443") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 1 {
		t.Errorf("want 1 matched, got %d", r.MatchedCount())
	}
	if r.MatchedRecords[0].Get("dstport") != "443" {
		t.Errorf("unexpected dstport %q", r.MatchedRecords[0].Get("dstport"))
	}
}

func TestFilter_DstPortRange(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.DstPort("80-8080") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() == 0 {
		t.Error("expected at least one match for dstport range 80-8080")
	}
	for _, rec := range r.MatchedRecords {
		p := rec.DstPortInt()
		if p < 80 || p > 8080 {
			t.Errorf("dstport %d out of [80,8080]", p)
		}
	}
}

// AND filters
func TestFilter_SrcIPAndDstIP(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder {
		return b.SrcIP("10.0.1.5").DstIP("198.51.100.3")
	})
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 1 {
		t.Errorf("want 1 matched, got %d", r.MatchedCount())
	}
	rec := r.MatchedRecords[0]
	if rec.SrcAddr() != "10.0.1.5" || rec.DstAddr() != "198.51.100.3" {
		t.Errorf("wrong record: %v", rec.Fields)
	}
}

func TestFilter_SrcIPAndDstPort(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder {
		return b.SrcIP("10.0.1.5").DstPort("443")
	})
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 1 {
		t.Errorf("want 1 matched, got %d", r.MatchedCount())
	}
}

func TestFilter_AllFourFilters_NoMatch(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder {
		return b.SrcIP("10.0.1.5").DstIP("8.8.8.8").SrcPort("443").DstPort("443")
	})
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 0 {
		t.Errorf("want 0 matched, got %d", r.MatchedCount())
	}
}

func TestFilter_SrcPortAndDstPort(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder {
		return b.SrcPort("443").DstPort("52314")
	})
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 1 {
		t.Errorf("want 1 matched, got %d", r.MatchedCount())
	}
}

// AWS '-' sentinel
const nodataLog = "2 123456789012 eni-x - - - - - - - 1620000100 1620000160 - NODATA\n"

func TestSentinel_DashNeverMatchesIPFilter(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcIP("0.0.0.0/0") })
	r := parse(t, nodataLog, c)
	if r.MatchedCount() != 0 {
		t.Errorf("expected 0 matches for '-' srcaddr with CIDR filter, got %d", r.MatchedCount())
	}
}

func TestSentinel_DashReturnedWithoutFilter(t *testing.T) {
	r := parse(t, nodataLog, nil)
	if r.MatchedCount() != 1 {
		t.Errorf("expected 1 match (no filter), got %d", r.MatchedCount())
	}
}

// ── 9. Custom-format log (with header) ───────────────────────────────────────

const customLog = `version account-id interface-id srcaddr dstaddr srcport dstport protocol packets bytes start end action log-status vpc-id
3 123456789012 eni-b01 10.0.1.5 172.16.0.50 443 52314 6 12 4800 1620000001 1620000061 ACCEPT OK vpc-abc123
3 123456789012 eni-b02 10.0.2.20 8.8.4.4 53 45678 17 1 76 1620000010 1620000011 ACCEPT OK vpc-abc123
`

func TestCustomFormat_HeaderDetected(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcIP("10.0.1.5") })
	r := parse(t, customLog, c)
	if r.TotalRecords != 2 {
		t.Errorf("total: want 2, got %d", r.TotalRecords)
	}
	if r.MatchedCount() != 1 {
		t.Errorf("matched: want 1, got %d", r.MatchedCount())
	}
	if r.MatchedRecords[0].Get("vpc-id") != "vpc-abc123" {
		t.Errorf("custom field vpc-id not parsed correctly")
	}
}

func TestCustomFormat_PortFilter(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcPort("443") })
	r := parse(t, customLog, c)
	if r.MatchedCount() != 1 {
		t.Errorf("want 1, got %d", r.MatchedCount())
	}
}

// Connection counts
const connLog = `2 123456789012 eni-a 10.0.1.5 8.8.8.8 12345 53 17 1 76 100 161 ACCEPT OK
2 123456789012 eni-b 10.0.1.5 8.8.8.8 12345 53 17 2 152 200 261 ACCEPT OK
2 123456789012 eni-c 10.0.1.5 8.8.4.4 12345 53 17 1 76 300 361 ACCEPT OK
`

func TestConnectionCounts_TwoDistinct(t *testing.T) {
	r := parse(t, connLog, nil)
	if r.DistinctConnectionCount() != 2 {
		t.Errorf("want 2 distinct connections, got %d", r.DistinctConnectionCount())
	}
}

func TestConnectionCounts_Accumulation(t *testing.T) {
	r := parse(t, connLog, nil)
	// First key (10.0.1.5:12345 -> 8.8.8.8:53 proto=17) should have count 2
	found := false
	for _, key := range r.ConnectionOrder {
		if key.DstAddr == "8.8.8.8" {
			if r.ConnectionCounts[key] != 2 {
				t.Errorf("want count 2 for 8.8.8.8, got %d", r.ConnectionCounts[key])
			}
			found = true
		}
	}
	if !found {
		t.Error("connection key for 8.8.8.8 not found")
	}
}

func TestConnectionCounts_ScopedToMatched(t *testing.T) {
	c := buildCriteria(t, func(b *filter.Builder) *filter.Builder { return b.SrcIP("10.0.1.5") })
	r := parse(t, sampleLog, c)
	if r.MatchedCount() != 4 {
		t.Errorf("want 4 matched, got %d", r.MatchedCount())
	}
	if r.DistinctConnectionCount() != 4 {
		t.Errorf("want 4 distinct connections, got %d", r.DistinctConnectionCount())
	}
	for _, key := range r.ConnectionOrder {
		if r.ConnectionCounts[key] != 1 {
			t.Errorf("want count 1 per connection, got %d for %s", r.ConnectionCounts[key], key)
		}
	}
}

func TestConnectionCounts_DuplicateRows(t *testing.T) {
	line := "2 123456789012 eni-x 10.0.1.1 10.0.0.2 80 54321 6 10 5000 100 160 ACCEPT OK\n"
	r := parse(t, strings.Repeat(line, 5), nil)
	if r.DistinctConnectionCount() != 1 {
		t.Errorf("want 1 distinct connection, got %d", r.DistinctConnectionCount())
	}
	for _, key := range r.ConnectionOrder {
		if r.ConnectionCounts[key] != 5 {
			t.Errorf("want count 5, got %d", r.ConnectionCounts[key])
		}
	}
}

func TestConnectionCounts_ProtocolDistinguishes(t *testing.T) {
	// Same src/dst/ports, different protocol → 2 distinct connections
	log := `2 123456789012 eni-a 10.0.1.5 8.8.8.8 1000 53 6 1 40 100 160 ACCEPT OK
2 123456789012 eni-b 10.0.1.5 8.8.8.8 1000 53 17 1 40 100 160 ACCEPT OK
`
	r := parse(t, log, nil)
	if r.DistinctConnectionCount() != 2 {
		t.Errorf("TCP vs UDP should be 2 distinct connections, got %d", r.DistinctConnectionCount())
	}
}

// Blank lines and comments
func TestBlankAndCommentLines(t *testing.T) {
	log := "\n# comment\n" +
		"2 123456789012 eni-a 10.0.1.5 8.8.8.8 443 53 17 1 76 100 160 ACCEPT OK\n" +
		"\n# another\n" +
		"2 123456789012 eni-b 10.0.1.6 8.8.8.8 80 53 17 1 76 100 160 ACCEPT OK\n"
	r := parse(t, log, nil)
	if r.TotalRecords != 2 {
		t.Errorf("want 2 total, got %d", r.TotalRecords)
	}
}
