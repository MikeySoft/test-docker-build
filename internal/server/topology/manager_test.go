package topology

import "testing"

func TestNewManagerDefaults(t *testing.T) {
	manager := NewManager(nil, nil, 0, 0, 0)
	if manager.refreshInterval != defaultRefreshInterval {
		t.Fatalf("expected default refresh interval, got %v", manager.refreshInterval)
	}
	if manager.staleAfter != defaultRefreshInterval*defaultStaleMultiplier {
		t.Fatalf("expected default staleAfter, got %v", manager.staleAfter)
	}
	if manager.batchSize != defaultBatchSize {
		t.Fatalf("expected default batch size, got %d", manager.batchSize)
	}
}

func TestChunkStrings(t *testing.T) {
	values := []string{"a", "b", "c", "d", "e"}
	chunks := chunkStrings(values, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[2]) != 1 || chunks[2][0] != "e" {
		t.Fatalf("unexpected last chunk: %#v", chunks[2])
	}
}

func TestExtractIDsFromList(t *testing.T) {
	input := []any{
		map[string]any{"id": "one"},
		map[string]any{"id": "two"},
	}
	ids := extractIDsFromList(input)
	if len(ids) != 2 || ids[0] != "one" || ids[1] != "two" {
		t.Fatalf("unexpected ids: %#v", ids)
	}

	input = []any{
		map[string]any{"name": "vol"},
	}
	if ids := extractIDsFromList(input, "name"); len(ids) != 1 || ids[0] != "vol" {
		t.Fatalf("unexpected ids with override: %#v", ids)
	}
}

func TestCloneJSONMap(t *testing.T) {
	original := map[string]any{"a": 1}
	cloned := cloneJSONMap(original)
	if cloned["a"] != 1 {
		t.Fatalf("expected cloned value 1, got %#v", cloned["a"])
	}
	cloned["a"] = 2
	if original["a"] == 2 {
		t.Fatal("cloneJSONMap should create a copy, but mutation affected original")
	}
}
