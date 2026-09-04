// Package stats implements Sitrep's pluggable system-status checks: each
// check (internet reachability, disk space, DRAM, CPU, git auth status)
// implements the Checker interface and is looked up by name from a
// Registry, so the manager-configured enabled-checks list drives which
// ones actually run on a worker.
package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/justwaters/sitrep/internal/transport"
)

// checkTimeout bounds a single Checker.Collect call. It's well under the
// shortest sane reporting interval, and short enough that one hung check
// (e.g. ping against a firewalled network) can't stall a whole report
// cycle for the others.
const checkTimeout = 5 * time.Second

// Checker collects a single named system statistic.
type Checker interface {
	// Name is the stable key used as Report.Stats' map key and in
	// EnabledChecks lists (see transport.AllChecks).
	Name() string
	// Collect gathers the current value. The returned JSON must match the
	// documented per-check payload type in internal/transport/protocol.go
	// (e.g. transport.DiskResult for the "disk" checker).
	Collect(ctx context.Context) (json.RawMessage, error)
}

// Registry holds the set of checkers Sitrep knows how to run, keyed by
// name.
type Registry struct {
	checkers map[string]Checker
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{checkers: make(map[string]Checker)}
}

// NewDefaultRegistry returns a registry with all five built-in checkers
// registered, using their default configuration.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewInternetChecker())
	r.Register(NewDiskChecker(""))
	r.Register(NewDRAMChecker())
	r.Register(NewCPUChecker())
	r.Register(NewGitAuthChecker())
	return r
}

// Register adds or replaces a checker under its own Name().
func (r *Registry) Register(c Checker) {
	r.checkers[c.Name()] = c
}

// Get looks up a checker by name.
func (r *Registry) Get(name string) (Checker, bool) {
	c, ok := r.checkers[name]
	return c, ok
}

// CollectEnabled runs every checker named in enabled concurrently, each
// under its own timeout and panic recovery (checkers shell out to
// external binaries, so a crash or hang in one must not affect the
// others), and returns a map of transport.StatResult keyed by check name.
// Unknown check names are silently skipped.
func (r *Registry) CollectEnabled(ctx context.Context, enabled []string) map[string]transport.StatResult {
	results := make(map[string]transport.StatResult, len(enabled))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, name := range enabled {
		checker, ok := r.checkers[name]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(name string, c Checker) {
			defer wg.Done()
			result := runChecker(ctx, c)
			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, checker)
	}
	wg.Wait()
	return results
}

func runChecker(ctx context.Context, c Checker) (result transport.StatResult) {
	defer func() {
		if p := recover(); p != nil {
			result = transport.StatResult{OK: false, Error: fmt.Sprintf("checker panic: %v", p)}
		}
	}()

	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	val, err := c.Collect(cctx)
	if err != nil {
		return transport.StatResult{OK: false, Error: err.Error()}
	}
	return transport.StatResult{OK: true, Value: val}
}
