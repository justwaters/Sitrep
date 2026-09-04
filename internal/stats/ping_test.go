package stats

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/justwaters/sitrep/internal/transport"
)

// The InternetChecker shells out to iputils-ping's `-W` (whole-seconds
// timeout) semantics, which Sitrep targets on Linux only — BSD/macOS
// ping interprets -W in milliseconds instead, so these tests are
// Linux-only rather than asserting cross-platform ping behavior.

func TestInternetChecker_Loopback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ping -W semantics are Linux-specific (iputils-ping)")
	}
	if _, err := exec.LookPath("ping"); err != nil {
		t.Skip("ping not available")
	}

	c := &InternetChecker{Target: "127.0.0.1", Timeout: 2 * time.Second}
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var result transport.PingResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.PacketLoss {
		t.Error("pinging loopback should succeed")
	}
}

func TestInternetChecker_Unreachable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ping -W semantics are Linux-specific (iputils-ping)")
	}
	if _, err := exec.LookPath("ping"); err != nil {
		t.Skip("ping not available")
	}

	// TEST-NET-1 (RFC 5737): reserved for documentation, guaranteed
	// non-routable, so this reliably exercises the packet-loss path.
	c := &InternetChecker{Target: "192.0.2.1", Timeout: 1 * time.Second}
	raw, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect should not error on an unreachable host, only report loss: %v", err)
	}

	var result transport.PingResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.PacketLoss {
		t.Error("pinging an unreachable host should report packet loss")
	}
}

func TestInternetChecker_MissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := NewInternetChecker()
	if _, err := c.Collect(context.Background()); err == nil {
		t.Error("expected an error when ping is not in PATH")
	}
}
