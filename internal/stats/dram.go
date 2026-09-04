package stats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"

	"github.com/justwaters/sitrep/internal/transport"
)

// DRAMChecker reports system memory usage.
type DRAMChecker struct{}

func NewDRAMChecker() *DRAMChecker { return &DRAMChecker{} }

func (c *DRAMChecker) Name() string { return transport.CheckDRAM }

func (c *DRAMChecker) Collect(ctx context.Context) (json.RawMessage, error) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("memory usage: %w", err)
	}
	result := transport.MemResult{
		TotalBytes:  vm.Total,
		UsedBytes:   vm.Used,
		UsedPercent: vm.UsedPercent,
	}
	return json.Marshal(result)
}
