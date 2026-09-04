package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"

	"github.com/justwaters/sitrep/internal/transport"
)

// CPUChecker reports aggregate CPU utilization, sampled over a short
// window.
type CPUChecker struct {
	SampleWindow time.Duration
}

func NewCPUChecker() *CPUChecker {
	return &CPUChecker{SampleWindow: 500 * time.Millisecond}
}

func (c *CPUChecker) Name() string { return transport.CheckCPU }

func (c *CPUChecker) Collect(ctx context.Context) (json.RawMessage, error) {
	percents, err := cpu.PercentWithContext(ctx, c.SampleWindow, false)
	if err != nil {
		return nil, fmt.Errorf("cpu usage: %w", err)
	}
	if len(percents) == 0 {
		return nil, fmt.Errorf("cpu usage: no data returned")
	}
	result := transport.CPUResult{UsedPercent: percents[0]}
	return json.Marshal(result)
}
