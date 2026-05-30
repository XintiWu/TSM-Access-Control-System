package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func newTestCache(t *testing.T) (*ReportCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := NewReportCache(mr.Addr())
	return c, mr
}

func TestReportCache_Ping(t *testing.T) {
	c, _ := newTestCache(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() = %v", err)
	}
}

func TestReportCache_GetSet_RoundTrip(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	data := []byte(`{"key":"value"}`)
	if err := c.Set(ctx, "report:test:1", data); err != nil {
		t.Fatalf("Set() = %v", err)
	}

	got, err := c.Get(ctx, "report:test:1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Get() = %q, want %q", got, data)
	}
}

func TestReportCache_Get_Miss(t *testing.T) {
	c, _ := newTestCache(t)
	got, err := c.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != nil {
		t.Errorf("Get() = %v, want nil for cache miss", got)
	}
}

func TestReportCache_Invalidate(t *testing.T) {
	c, mr := newTestCache(t)
	ctx := context.Background()

	if err := mr.Set("report:dept:org1:key1", "data1"); err != nil {
		t.Fatalf("mr.Set failed: %v", err)
	}
	if err := mr.Set("report:dept:org1:key2", "data2"); err != nil {
		t.Fatalf("mr.Set failed: %v", err)
	}
	if err := mr.Set("report:personal:user1", "other"); err != nil {
		t.Fatalf("mr.Set failed: %v", err)
	}

	if err := c.Invalidate(ctx, "report:dept:org1:*"); err != nil {
		t.Fatalf("Invalidate() = %v", err)
	}

	// Invalidated keys should be gone
	if mr.Exists("report:dept:org1:key1") {
		t.Error("expected report:dept:org1:key1 to be deleted")
	}
	if mr.Exists("report:dept:org1:key2") {
		t.Error("expected report:dept:org1:key2 to be deleted")
	}

	// Non-matching key should remain
	if !mr.Exists("report:personal:user1") {
		t.Error("expected report:personal:user1 to remain")
	}
}

func TestReportCache_Invalidate_NoMatch(t *testing.T) {
	c, _ := newTestCache(t)
	err := c.Invalidate(context.Background(), "nonexistent:*")
	if err != nil {
		t.Fatalf("Invalidate() should not error for no matches: %v", err)
	}
}
