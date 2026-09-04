package registry

import (
	"sync"
	"testing"

	"github.com/justwaters/sitrep/internal/transport"
)

func TestUpsertGetList(t *testing.T) {
	r := New()
	report := &transport.Report{WorkerID: "wkr_1", Name: "host1", Timestamp: 123}
	r.Upsert(report)

	entry, ok := r.Get("wkr_1")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if entry.Name != "host1" {
		t.Errorf("name = %q, want host1", entry.Name)
	}
	if entry.LastReport.Timestamp != 123 {
		t.Errorf("timestamp = %d, want 123", entry.LastReport.Timestamp)
	}
	if entry.EnrolledAt.IsZero() {
		t.Error("expected EnrolledAt to be set on first report")
	}

	if got := len(r.List()); got != 1 {
		t.Errorf("List() length = %d, want 1", got)
	}
}

func TestGetUnknown(t *testing.T) {
	r := New()
	if _, ok := r.Get("wkr_nonexistent"); ok {
		t.Error("expected ok=false for unknown worker")
	}
}

func TestUpsertPreservesEnrolledAt(t *testing.T) {
	r := New()
	r.Upsert(&transport.Report{WorkerID: "wkr_1", Timestamp: 1})
	first, _ := r.Get("wkr_1")

	r.Upsert(&transport.Report{WorkerID: "wkr_1", Timestamp: 2})
	second, _ := r.Get("wkr_1")

	if !first.EnrolledAt.Equal(second.EnrolledAt) {
		t.Error("EnrolledAt should not change on subsequent reports")
	}
	if second.LastReport.Timestamp != 2 {
		t.Errorf("expected LastReport to be updated to the latest report")
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			const id = transport.WorkerID("wkr_concurrent")
			r.Upsert(&transport.Report{WorkerID: id, Name: "h"})
			r.Get(id)
			r.List()
		}()
	}
	wg.Wait()
}
