package stats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/justwaters/sitrep/internal/transport"
)

// These exercise the gopsutil-backed checkers against the real local
// machine (there's nothing meaningful to fake for disk/mem/cpu sampling),
// asserting only that they return plausible, non-error data.

func TestDiskChecker(t *testing.T) {
	c := NewDiskChecker("")
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	var result transport.DiskResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.TotalBytes == 0 {
		t.Error("expected non-zero total bytes")
	}
	if result.Path != "/" {
		t.Errorf("Path = %q, want / (default)", result.Path)
	}
}

func TestDRAMChecker(t *testing.T) {
	c := NewDRAMChecker()
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	var result transport.MemResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.TotalBytes == 0 {
		t.Error("expected non-zero total bytes")
	}
}

func TestCPUChecker(t *testing.T) {
	c := &CPUChecker{SampleWindow: 100 * time.Millisecond}
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	var result transport.CPUResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.UsedPercent < 0 {
		t.Errorf("UsedPercent = %v, want >= 0", result.UsedPercent)
	}
}
