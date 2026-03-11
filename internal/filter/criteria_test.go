package filter_test

import (
	"testing"

	"github.com/flowlog/service/internal/filter"
)

func TestFilterCriteria_InvalidIP(t *testing.T) {
	_, err := filter.NewBuilder().SrcIP("999.999.999.999").Build()
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}
}

func TestFilterCriteria_InvalidCIDRPrefix(t *testing.T) {
	_, err := filter.NewBuilder().SrcIP("10.0.0.0/33").Build()
	if err == nil {
		t.Fatal("expected error for prefix > 32")
	}
}

func TestFilterCriteria_InvalidPort(t *testing.T) {
	_, err := filter.NewBuilder().DstPort("70000").Build()
	if err == nil {
		t.Fatal("expected error for port > 65535")
	}
}

func TestFilterCriteria_InvertedRange(t *testing.T) {
	_, err := filter.NewBuilder().SrcPort("8080-80").Build()
	if err == nil {
		t.Fatal("expected error for inverted range")
	}
}

func TestFilterCriteria_NonNumericPort(t *testing.T) {
	_, err := filter.NewBuilder().DstPort("https").Build()
	if err == nil {
		t.Fatal("expected error for non-numeric port")
	}
}

func TestFilterCriteria_ValidCIDR(t *testing.T) {
	_, err := filter.NewBuilder().SrcIP("10.0.0.0/24").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFilterCriteria_HasFilters(t *testing.T) {
	empty, _ := filter.NewBuilder().Build()
	if empty.HasFilters() {
		t.Error("empty builder should have no filters")
	}
	withSrc, _ := filter.NewBuilder().SrcIP("10.0.1.5").Build()
	if !withSrc.HasFilters() {
		t.Error("builder with srcIp should have filters")
	}
}
