package filter

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/flowlog/service/internal/model"
)

type Criteria struct {
	srcNet *net.IPNet
	dstNet *net.IPNet
	srcLo  int
	srcHi  int
	dstLo  int
	dstHi  int

	SrcIPDesc   string
	DstIPDesc   string
	SrcPortDesc string
	DstPortDesc string
}

func (c *Criteria) Matches(r *model.Record) bool {
	if c.srcNet != nil {
		ip := net.ParseIP(r.SrcAddr())
		if ip == nil || !c.srcNet.Contains(ip) {
			return false
		}
	}
	if c.dstNet != nil {
		ip := net.ParseIP(r.DstAddr())
		if ip == nil || !c.dstNet.Contains(ip) {
			return false
		}
	}
	if c.srcLo != -1 {
		p := r.SrcPortInt()
		if p < 0 || p < c.srcLo || p > c.srcHi {
			return false
		}
	}
	if c.dstLo != -1 {
		p := r.DstPortInt()
		if p < 0 || p < c.dstLo || p > c.dstHi {
			return false
		}
	}
	return true
}

func (c *Criteria) HasFilters() bool {
	return c.srcNet != nil || c.dstNet != nil || c.srcLo != -1 || c.dstLo != -1
}

type Builder struct {
	c   Criteria
	err error
}

func NewBuilder() *Builder {
	return &Builder{c: Criteria{srcLo: -1, srcHi: -1, dstLo: -1, dstHi: -1}}
}

func (b *Builder) SrcIP(raw string) *Builder {
	if b.err != nil || raw == "" {
		return b
	}
	network, desc, err := parseCIDR(raw, "srcIp")
	if err != nil {
		b.err = err
		return b
	}
	b.c.srcNet = network
	b.c.SrcIPDesc = desc
	return b
}

func (b *Builder) DstIP(raw string) *Builder {
	if b.err != nil || raw == "" {
		return b
	}
	network, desc, err := parseCIDR(raw, "dstIp")
	if err != nil {
		b.err = err
		return b
	}
	b.c.dstNet = network
	b.c.DstIPDesc = desc
	return b
}

func (b *Builder) SrcPort(raw string) *Builder {
	if b.err != nil || raw == "" {
		return b
	}
	lo, hi, desc, err := parsePortRange(raw, "srcPort")
	if err != nil {
		b.err = err
		return b
	}
	b.c.srcLo, b.c.srcHi = lo, hi
	b.c.SrcPortDesc = desc
	return b
}

func (b *Builder) DstPort(raw string) *Builder {
	if b.err != nil || raw == "" {
		return b
	}
	lo, hi, desc, err := parsePortRange(raw, "dstPort")
	if err != nil {
		b.err = err
		return b
	}
	b.c.dstLo, b.c.dstHi = lo, hi
	b.c.DstPortDesc = desc
	return b
}

func (b *Builder) Build() (*Criteria, error) {
	if b.err != nil {
		return nil, b.err
	}
	c := b.c
	return &c, nil
}

// need to work with both "10.0.1.5" or "10.0.0.0/24".
func parseCIDR(raw, field string) (*net.IPNet, string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "/") {
		ip := net.ParseIP(raw)
		if ip == nil {
			return nil, "", fmt.Errorf("%s: %q is not a valid IPv4 address", field, raw)
		}
		if ip.To4() == nil {
			return nil, "", fmt.Errorf("%s: only IPv4 addresses are supported", field)
		}
		mask := net.CIDRMask(32, 32)
		return &net.IPNet{IP: ip.To4().Mask(mask), Mask: mask}, raw, nil
	}
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %q is not a valid CIDR: %w", field, raw, err)
	}
	if network.IP.To4() == nil {
		return nil, "", fmt.Errorf("%s: only IPv4 CIDR is supported", field)
	}
	return network, raw, nil
}

// either port or range -  "443" or "1024-65535".
func parsePortRange(raw, field string) (lo, hi int, desc string, err error) {
	raw = strings.TrimSpace(raw)
	idx := strings.Index(raw, "-")
	if idx <= 0 {
		p, e := parsePort(raw, field)
		if e != nil {
			return 0, 0, "", e
		}
		return p, p, raw, nil
	}
	lo, err = parsePort(strings.TrimSpace(raw[:idx]), field)
	if err != nil {
		return
	}
	hi, err = parsePort(strings.TrimSpace(raw[idx+1:]), field)
	if err != nil {
		return
	}
	if lo > hi {
		return 0, 0, "", fmt.Errorf("%s: range low %d > high %d", field, lo, hi)
	}
	return lo, hi, raw, nil
}

func parsePort(s, field string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a valid port number", field, s)
	}
	if n < 0 || n > 65535 {
		return 0, fmt.Errorf("%s: port %d out of range [0,65535]", field, n)
	}
	return n, nil
}
