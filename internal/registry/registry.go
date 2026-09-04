// Package registry holds the manager's in-memory table of known workers
// and their most recent report. By design there is no persistence layer:
// this is a live-status view, not a history/trend store, and is lost on
// manager restart.
package registry

import (
	"sort"
	"sync"
	"time"

	"github.com/justwaters/sitrep/internal/transport"
)

// WorkerEntry is a worker's current known state.
type WorkerEntry struct {
	ID         transport.WorkerID
	Name       string
	EnrolledAt time.Time
	LastSeen   time.Time
	LastReport *transport.Report
}

// Registry is the manager's worker table, safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	workers map[transport.WorkerID]*WorkerEntry
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{workers: make(map[transport.WorkerID]*WorkerEntry)}
}

// Upsert records a fresh report, creating the worker's entry (with
// EnrolledAt set to now) if this is the first report seen from it.
func (r *Registry) Upsert(report *transport.Report) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.workers[report.WorkerID]
	if !ok {
		entry = &WorkerEntry{ID: report.WorkerID, EnrolledAt: time.Now()}
		r.workers[report.WorkerID] = entry
	}
	entry.Name = report.Name
	entry.LastSeen = time.Now()
	entry.LastReport = report
}

// Get returns a copy of the entry for id, if known.
func (r *Registry) Get(id transport.WorkerID) (*WorkerEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.workers[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// List returns a copy of every known worker's entry, sorted by ID for
// stable output.
func (r *Registry) List() []*WorkerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*WorkerEntry, 0, len(r.workers))
	for _, e := range r.workers {
		cp := *e
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
