package stats

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeChecker struct {
	name string
	fn   func(ctx context.Context) (json.RawMessage, error)
}

func (f *fakeChecker) Name() string { return f.name }
func (f *fakeChecker) Collect(ctx context.Context) (json.RawMessage, error) {
	return f.fn(ctx)
}

func TestCollectEnabled_SuccessErrorAndUnknown(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeChecker{name: "ok", fn: func(ctx context.Context) (json.RawMessage, error) {
		return json.RawMessage(`{"value":1}`), nil
	}})
	r.Register(&fakeChecker{name: "bad", fn: func(ctx context.Context) (json.RawMessage, error) {
		return nil, errors.New("boom")
	}})

	results := r.CollectEnabled(context.Background(), []string{"ok", "bad", "unknown"})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (unknown check name should be skipped)", len(results))
	}
	if !results["ok"].OK {
		t.Error("ok checker should report OK=true")
	}
	if results["bad"].OK {
		t.Error("bad checker should report OK=false")
	}
	if results["bad"].Error == "" {
		t.Error("bad checker should carry an error message")
	}
}

func TestCollectEnabled_PanicRecovered(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeChecker{name: "panicky", fn: func(ctx context.Context) (json.RawMessage, error) {
		panic("boom")
	}})

	results := r.CollectEnabled(context.Background(), []string{"panicky"})
	if results["panicky"].OK {
		t.Error("panicking checker should report OK=false, not crash the batch")
	}
}

func TestCollectEnabled_Timeout(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeChecker{name: "slow", fn: func(ctx context.Context) (json.RawMessage, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Hour):
			return json.RawMessage(`{}`), nil
		}
	}})

	start := time.Now()
	results := r.CollectEnabled(context.Background(), []string{"slow"})
	if elapsed := time.Since(start); elapsed > checkTimeout+2*time.Second {
		t.Errorf("CollectEnabled took %v, expected to be bounded by checkTimeout (%v)", elapsed, checkTimeout)
	}
	if results["slow"].OK {
		t.Error("timed-out checker should report OK=false")
	}
}

func TestNewDefaultRegistryHasAllChecks(t *testing.T) {
	r := NewDefaultRegistry()
	for _, name := range []string{"internet", "disk", "dram", "cpu", "gitauth"} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("default registry missing checker %q", name)
		}
	}
}
