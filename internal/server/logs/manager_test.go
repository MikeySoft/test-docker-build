package logs

import (
	"testing"
	"time"
)

func TestManagerAddAndList(t *testing.T) {
	mgr := NewManager(5)
	first := mgr.Add(Entry{Message: "one"})
	second := mgr.Add(Entry{Message: "two"})

	entries := mgr.List("", 10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != first.ID || entries[1].ID != second.ID {
		t.Fatalf("entries not in order")
	}

	mgr.Add(Entry{Message: "three"})
	afterSecond := mgr.List(second.ID, 10)
	if len(afterSecond) != 1 || afterSecond[0].Message != "three" {
		t.Fatalf("expected entries after second message to include new entries")
	}
}

func TestSubscribeReceivesEntries(t *testing.T) {
	mgr := NewManager(5)
	ch, unsubscribe := mgr.Subscribe()
	defer unsubscribe()

	want := Entry{Message: "hello", Timestamp: time.Now()}
	mgr.Add(want)

	select {
	case got := <-ch:
		if got.Message != "hello" {
			t.Fatalf("expected message hello, got %s", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}
