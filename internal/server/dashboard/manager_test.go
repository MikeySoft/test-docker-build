package dashboard

import (
	"context"
	"testing"
	"time"
)

func TestUpdateSummarySetsTimestamp(t *testing.T) {
	mgr := NewManager(nil)
	mgr.UpdateSummary(Summary{HostsTotal: 3})

	summary, err := mgr.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("GetSummary returned error: %v", err)
	}
	if summary.HostsTotal != 3 {
		t.Fatalf("expected HostsTotal 3, got %d", summary.HostsTotal)
	}
	if summary.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
}

func TestRefreshSummaryRequiresDB(t *testing.T) {
	mgr := NewManager(nil)
	if err := mgr.RefreshSummary(context.Background()); err == nil {
		t.Fatal("expected RefreshSummary to fail without database")
	}
}

func TestGetSummaryRefreshesWhenStale(t *testing.T) {
	mgr := NewManager(nil)
	// Manually zero updated time to force refresh path which should error
	mgr.summary = Summary{}
	if _, err := mgr.GetSummary(context.Background()); err == nil {
		t.Fatal("expected GetSummary to fail when refresh cannot run")
	}

	// After updating summary, GetSummary should succeed.
	mgr.UpdateSummary(Summary{UpdatedAt: time.Now(), HostsTotal: 1})
	if _, err := mgr.GetSummary(context.Background()); err != nil {
		t.Fatalf("GetSummary after update returned error: %v", err)
	}
}
