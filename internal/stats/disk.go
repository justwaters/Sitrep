package stats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shirou/gopsutil/v4/disk"

	"github.com/justwaters/sitrep/internal/transport"
)

// DiskChecker reports space usage for a single mounted filesystem path.
type DiskChecker struct {
	Path string
}

// NewDiskChecker returns a checker for path, defaulting to "/" (the root
// filesystem) if path is empty.
func NewDiskChecker(path string) *DiskChecker {
	if path == "" {
		path = "/"
	}
	return &DiskChecker{Path: path}
}

func (c *DiskChecker) Name() string { return transport.CheckDisk }

func (c *DiskChecker) Collect(ctx context.Context) (json.RawMessage, error) {
	usage, err := disk.UsageWithContext(ctx, c.Path)
	if err != nil {
		return nil, fmt.Errorf("disk usage for %s: %w", c.Path, err)
	}
	result := transport.DiskResult{
		Path:        c.Path,
		TotalBytes:  usage.Total,
		UsedBytes:   usage.Used,
		UsedPercent: usage.UsedPercent,
	}
	return json.Marshal(result)
}
