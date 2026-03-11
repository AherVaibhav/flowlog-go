package model

import (
	"fmt"
	"strconv"
)

const FieldNotApplicable = "-"

var DefaultV2Columns = []string{
	"version", "account-id", "interface-id",
	"srcaddr", "dstaddr",
	"srcport", "dstport",
	"protocol", "packets", "bytes",
	"start", "end", "action", "log-status",
}

type Record struct {
	LineNumber int
	Columns    []string
	Fields     map[string]string
}

func (r *Record) Get(field string) string {
	if v, ok := r.Fields[field]; ok {
		return v
	}
	return FieldNotApplicable
}

func (r *Record) IsPresent(field string) bool {
	v, ok := r.Fields[field]
	return ok && v != FieldNotApplicable
}

func (r *Record) SrcAddr() string {
	return r.Get("srcaddr")
}
func (r *Record) DstAddr() string {
	return r.Get("dstaddr")
}
func (r *Record) Action() string {
	return r.Get("action")
}
func (r *Record) LogStatus() string {
	return r.Get("log-status")
}

func (r *Record) SrcPortInt() int {
	return parsePortInt(r.Get("srcport"))
}
func (r *Record) DstPortInt() int {
	return parsePortInt(r.Get("dstport"))
}
func (r *Record) ProtocolInt() int {
	v := r.Get("protocol")
	if v == FieldNotApplicable {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

func parsePortInt(v string) int {
	if v == FieldNotApplicable {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

type ConnectionKey struct {
	SrcAddr  string
	SrcPort  int
	DstAddr  string
	DstPort  int
	Protocol int
}

func ConnectionKeyFrom(r *Record) ConnectionKey {
	return ConnectionKey{
		SrcAddr:  r.SrcAddr(),
		SrcPort:  r.SrcPortInt(),
		DstAddr:  r.DstAddr(),
		DstPort:  r.DstPortInt(),
		Protocol: r.ProtocolInt(),
	}
}

func (c ConnectionKey) ProtocolName() string {
	switch c.Protocol {
	case 1:
		return "ICMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 58:
		return "ICMPv6"
	case -1:
		return "-"
	default:
		return fmt.Sprintf("%d", c.Protocol)
	}
}

func (c ConnectionKey) String() string {
	sp := strconv.Itoa(c.SrcPort)
	if c.SrcPort == -1 {
		sp = "-"
	}
	dp := strconv.Itoa(c.DstPort)
	if c.DstPort == -1 {
		dp = "-"
	}
	return fmt.Sprintf("%s:%s -> %s:%s proto=%s",
		c.SrcAddr, sp, c.DstAddr, dp, c.ProtocolName())
}

type ParseResult struct {
	TotalRecords   int
	SkippedRecords int
	MatchedRecords []*Record

	ConnectionCounts map[ConnectionKey]int64

	ConnectionOrder []ConnectionKey
}

func (r *ParseResult) MatchedCount() int {
	return len(r.MatchedRecords)
}
func (r *ParseResult) DistinctConnectionCount() int {
	return len(r.ConnectionCounts)
}
