package querydsl

import "testing"

func TestParseAndEvaluate_SimpleContainsAnd(t *testing.T) {
	expr, err := Parse(`name:nginx status=running`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	rec := map[string]any{
		"name":   "nginx-proxy",
		"status": "running",
	}
	if !EvaluateRecord(expr, rec) {
		t.Fatalf("expected record to match")
	}
}

func TestParseAndEvaluate_OrDisjunction(t *testing.T) {
	expr, err := Parse(`image:postgres OR image:pgvector`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	recA := map[string]any{"image": "postgres:16"}
	recB := map[string]any{"image": "pgvector:0.5"}
	recC := map[string]any{"image": "mysql:8"}
	if !EvaluateRecord(expr, recA) || !EvaluateRecord(expr, recB) {
		t.Fatalf("expected postgres/pgvector to match")
	}
	if EvaluateRecord(expr, recC) {
		t.Fatalf("expected mysql to not match")
	}
}

func TestParseAndEvaluate_Negation(t *testing.T) {
	expr, err := Parse(`!status=exited name:"api service"`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	rec := map[string]any{"status": "running", "name": "api service"}
	if !EvaluateRecord(expr, rec) {
		t.Fatalf("expected record to match negation + name")
	}
}

func TestParseAndEvaluate_NotEquals(t *testing.T) {
	expr, err := Parse(`status!=exited`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !EvaluateRecord(expr, map[string]any{"status": "running"}) {
		t.Fatalf("expected running to match != exited")
	}
	if EvaluateRecord(expr, map[string]any{"status": "exited"}) {
		t.Fatalf("expected exited to not match != exited")
	}
}

func TestParseAndEvaluate_BareTermSearchesDefaultFields(t *testing.T) {
	expr, err := Parse(`proxy`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	rec := map[string]any{"name": "nginx-proxy"}
	if !EvaluateRecord(expr, rec) {
		t.Fatalf("expected bare term to match name")
	}
}
