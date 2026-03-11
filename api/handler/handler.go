package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/flowlog/service/internal/filter"
	"github.com/flowlog/service/internal/model"
	"github.com/flowlog/service/internal/parser"
)

const (
	maxUploadBytes = 20 * 1024 * 1024                   // 20 MB — hard parser limit
	readLimit      = maxUploadBytes + (5 * 1024 * 1024) // 25 MB — HTTP read limit
)

type Handler struct {
	parser *parser.Parser
	logger *slog.Logger
}

func New(logger *slog.Logger) *Handler {
	return &Handler{
		parser: parser.New(),
		logger: logger,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /api/v1/flowlogs/parse", h.Parse)
}

type HealthResponse struct {
	Status string `json:"status"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status: "UP",
	})
}

type ParseResponse struct {
	Stats            StatsDTO             `json:"stats"`
	Filters          FiltersDTO           `json:"filters"`
	Records          []RecordDTO          `json:"records"`
	ConnectionCounts []ConnectionCountDTO `json:"connectionCounts"`
}

type StatsDTO struct {
	TotalRecords        int    `json:"totalRecords"`
	MatchedRecords      int    `json:"matchedRecords"`
	SkippedRecords      int    `json:"skippedRecords"`
	DistinctConnections int    `json:"distinctConnections"`
	Filename            string `json:"filename"`
}

type FiltersDTO struct {
	SrcIP   string `json:"srcIp,omitempty"`
	DstIP   string `json:"dstIp,omitempty"`
	SrcPort string `json:"srcPort,omitempty"`
	DstPort string `json:"dstPort,omitempty"`
}

type RecordDTO struct {
	Fields map[string]string `json:"fields"`
}

type ConnectionCountDTO struct {
	SrcAddr      string `json:"srcAddr"`
	SrcPort      int    `json:"srcPort"`
	DstAddr      string `json:"dstAddr"`
	DstPort      int    `json:"dstPort"`
	Protocol     int    `json:"protocol"`
	ProtocolName string `json:"protocolName"`
	Count        int64  `json:"count"`
}

func (h *Handler) Parse(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, readLimit))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", err.Error())
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "request body is empty", "")
		return
	}
	if int64(len(data)) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "body exceeds 20 MB limit", "")
		return
	}

	for _, b := range data {
		if b > 127 {
			writeError(w, http.StatusBadRequest, "file validation failed", parser.ErrNonASCII.Error())
			return
		}
	}

	q := r.URL.Query()
	criteria, err := filter.NewBuilder().
		SrcIP(q.Get("srcIp")).
		DstIP(q.Get("dstIp")).
		SrcPort(q.Get("srcPort")).
		DstPort(q.Get("dstPort")).
		Build()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid filter parameter", err.Error())
		return
	}

	result, err := h.parser.ParseReader(bytes.NewReader(data), criteria)
	if err != nil {
		h.logger.Error("parse error", "err", err)
		writeError(w, http.StatusInternalServerError, "parse error", err.Error())
		return
	}

	filename := q.Get("filename")
	if filename == "" {
		filename = "unknown"
	}

	h.logger.Info("parsed",
		"file", filename,
		"total", result.TotalRecords,
		"matched", result.MatchedCount(),
		"connections", result.DistinctConnectionCount(),
	)

	writeJSON(w, http.StatusOK, buildResponse(result, criteria, filename))
}
func buildResponse(result *model.ParseResult, c *filter.Criteria, filename string) ParseResponse {
	records := make([]RecordDTO, 0, len(result.MatchedRecords))
	for _, rec := range result.MatchedRecords {
		fields := make(map[string]string, len(rec.Columns))
		for _, col := range rec.Columns {
			fields[col] = rec.Get(col)
		}
		records = append(records, RecordDTO{Fields: fields})
	}

	counts := make([]ConnectionCountDTO, 0, len(result.ConnectionOrder))
	for _, key := range result.ConnectionOrder {
		counts = append(counts, ConnectionCountDTO{
			SrcAddr:      key.SrcAddr,
			SrcPort:      key.SrcPort,
			DstAddr:      key.DstAddr,
			DstPort:      key.DstPort,
			Protocol:     key.Protocol,
			ProtocolName: key.ProtocolName(),
			Count:        result.ConnectionCounts[key],
		})
	}
	filters := FiltersDTO{}
	if c != nil {
		filters.SrcIP = c.SrcIPDesc
		filters.DstIP = c.DstIPDesc
		filters.SrcPort = c.SrcPortDesc
		filters.DstPort = c.DstPortDesc
	}

	return ParseResponse{
		Stats: StatsDTO{
			TotalRecords:        result.TotalRecords,
			MatchedRecords:      result.MatchedCount(),
			SkippedRecords:      result.SkippedRecords,
			DistinctConnections: result.DistinctConnectionCount(),
			Filename:            filename,
		},
		Filters:          filters,
		Records:          records,
		ConnectionCounts: counts,
	}
}

type ErrorResponse struct {
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Detail    string `json:"detail,omitempty"`
	Timestamp string `json:"timestamp"`
}

func writeError(w http.ResponseWriter, status int, msg, detail string) {
	writeJSON(w, status, ErrorResponse{
		Status:    status,
		Error:     msg,
		Detail:    detail,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "err", err)
	}
}
