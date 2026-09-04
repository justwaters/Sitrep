package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/justwaters/sitrep/internal/transport"
)

// InternetChecker reports internet reachability by shelling out to the
// system `ping` binary rather than opening a raw ICMP socket in-process —
// raw sockets need root or CAP_NET_RAW, while the system ping binary is
// already privileged (setuid or file-capability) on essentially every
// Linux distro.
type InternetChecker struct {
	Target  string
	Timeout time.Duration
}

func NewInternetChecker() *InternetChecker {
	return &InternetChecker{Target: "8.8.8.8", Timeout: 2 * time.Second}
}

func (c *InternetChecker) Name() string { return transport.CheckInternet }

func (c *InternetChecker) Collect(ctx context.Context) (json.RawMessage, error) {
	if _, err := exec.LookPath("ping"); err != nil {
		return nil, fmt.Errorf("ping binary not found in PATH: %w", err)
	}

	timeoutSec := int(c.Timeout.Round(time.Second) / time.Second)
	if timeoutSec < 1 {
		timeoutSec = 1
	}

	start := time.Now()
	// -W is a whole-seconds reply timeout under iputils-ping (the
	// standard Linux ping); Sitrep targets Linux only.
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", strconv.Itoa(timeoutSec), c.Target)
	runErr := cmd.Run()
	elapsed := time.Since(start)

	// A failed/lost ping is itself a valid, successfully-collected
	// result (reachability=false) — not a Collect error. Collect errors
	// are reserved for "the check itself couldn't run".
	result := transport.PingResult{
		ReachableMs: float64(elapsed.Milliseconds()),
		PacketLoss:  runErr != nil,
	}
	return json.Marshal(result)
}
