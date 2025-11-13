package querydsl

import (
	"strings"
)

// EvaluateRecord evaluates an Expr against a generic record map.
// Supported field lookups (case-insensitive):
// - name: record["name"] or record["Name"]
// - status: record["status"]
// - image: record["image"], record["image_name"], record["Image"]
// - host: record["host"], record["host_name"], record["hostName"]
// Bare terms (no field) apply to name, image, host.
func EvaluateRecord(expr Expr, rec map[string]any) bool {
	if len(expr.OrGroups) == 0 {
		return true
	}
	for _, group := range expr.OrGroups {
		if evalAndGroup(group, rec) {
			return true
		}
	}
	return false
}

func evalAndGroup(terms []Term, rec map[string]any) bool {
	for _, t := range terms {
		matched := false
		if t.Field == "" {
			// Bare term: check default fields
			matched = matchFieldValue(OpContains, t.Value, valueFor(rec, "name")) ||
				matchFieldValue(OpContains, t.Value, valueFor(rec, "image")) ||
				matchFieldValue(OpContains, t.Value, valueFor(rec, "host"))
		} else {
			matched = matchFieldValue(t.Op, t.Value, valueFor(rec, t.Field))
		}
		if t.Negate {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}

func valueFor(rec map[string]any, field string) string {
	// try common keys
	keys := []string{field, strings.Title(field), strings.ToUpper(field)}
	switch field {
	case "image":
		keys = append(keys, "image_name", "Image")
	case "host":
		keys = append(keys, "host_name", "hostName", "HostName")
	}
	for _, k := range keys {
		if v, ok := rec[k]; ok {
			if s, ok2 := v.(string); ok2 {
				return strings.ToLower(s)
			}
		}
	}
	return ""
}

func matchFieldValue(op Operator, needle string, hay string) bool {
	n := strings.ToLower(needle)
	switch op {
	case OpContains:
		return n == "" || strings.Contains(hay, n)
	case OpEquals:
		return hay == n
	case OpNotEquals:
		return hay != n
	default:
		return false
	}
}
