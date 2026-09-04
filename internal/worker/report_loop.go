package worker

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/stats"
	"github.com/justwaters/sitrep/internal/transport"
)

// tickTimeout bounds a single collect-and-report cycle: comfortably above
// stats.checkTimeout (checks run concurrently) plus network round trip,
// and well under any sane reporting interval.
const tickTimeout = 30 * time.Second

// ReportLoop runs the worker's collect-and-report cycle on the interval
// currently held in Config, self-adjusting interval and enabled checks
// whenever the manager's ReportAck carries different values — this is the
// entire "config push" mechanism; there is no separate channel.
type ReportLoop struct {
	Config     *config.WorkerConfig
	ConfigPath string
	Client     *http.Client
	Stats      *stats.Registry
	Logger     *slog.Logger

	mu sync.Mutex
}

// NewReportLoop constructs a ReportLoop. cfg is mutated in place (and
// re-saved to configPath) as the manager pushes config updates.
func NewReportLoop(cfg *config.WorkerConfig, configPath string, client *http.Client, statsRegistry *stats.Registry, logger *slog.Logger) *ReportLoop {
	return &ReportLoop{
		Config:     cfg,
		ConfigPath: configPath,
		Client:     client,
		Stats:      statsRegistry,
		Logger:     logger,
	}
}

// Run blocks, reporting on the current interval until ctx is done.
func (rl *ReportLoop) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(rl.currentInterval()):
			rl.tick(ctx)
		}
	}
}

func (rl *ReportLoop) tick(ctx context.Context) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = string(rl.snapshotWorkerID())
	}

	workerID, managerAddr, checks := rl.snapshot()

	cctx, cancel := context.WithTimeout(ctx, tickTimeout)
	defer cancel()

	results := rl.Stats.CollectEnabled(cctx, checks)
	report := transport.Report{
		WorkerID:  workerID,
		Hostname:  hostname,
		Timestamp: time.Now().Unix(),
		Stats:     results,
	}

	ack, err := transport.SendReport(cctx, rl.Client, managerAddr, report)
	if err != nil {
		rl.logger().Warn("report failed", "manager_addr", managerAddr, "err", err)
		return
	}
	rl.applyAck(ack)
}

func (rl *ReportLoop) snapshot() (transport.WorkerID, string, []string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	checks := make([]string, len(rl.Config.EnabledChecks))
	copy(checks, rl.Config.EnabledChecks)
	return rl.Config.WorkerID, rl.Config.ManagerAddr, checks
}

func (rl *ReportLoop) snapshotWorkerID() transport.WorkerID {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.Config.WorkerID
}

func (rl *ReportLoop) currentInterval() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return time.Duration(rl.Config.IntervalSeconds) * time.Second
}

func (rl *ReportLoop) applyAck(ack *transport.ReportAck) {
	rl.mu.Lock()
	changed := rl.Config.IntervalSeconds != ack.IntervalSeconds || !equalChecks(rl.Config.EnabledChecks, ack.EnabledChecks)
	var cfgCopy config.WorkerConfig
	if changed {
		rl.Config.IntervalSeconds = ack.IntervalSeconds
		rl.Config.EnabledChecks = append([]string(nil), ack.EnabledChecks...)
	}
	cfgCopy = *rl.Config
	rl.mu.Unlock()

	if !changed {
		return
	}
	if err := cfgCopy.Save(rl.ConfigPath); err != nil {
		rl.logger().Warn("failed to persist manager-pushed config", "err", err)
		return
	}
	rl.logger().Info("config updated by manager",
		"interval_seconds", cfgCopy.IntervalSeconds, "enabled_checks", cfgCopy.EnabledChecks)
}

func (rl *ReportLoop) logger() *slog.Logger {
	if rl.Logger != nil {
		return rl.Logger
	}
	return slog.Default()
}

func equalChecks(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
