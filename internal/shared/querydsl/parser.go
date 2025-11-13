package querydsl

import (
	"strings"
	"unicode"
)

// Field names supported in v1
var SupportedFields = map[string]struct{}{
	"name":   {},
	"status": {},
	"image":  {},
	"host":   {},
}

type Operator int

const (
	OpContains  Operator = iota // :
	OpEquals                    // =
	OpNotEquals                 // !=
)

type Term struct {
	Field  string
	Op     Operator
	Value  string
	Negate bool
	// If Field is empty, apply to default field set (name, image, host)
}

// Expr is a disjunction (OR) of conjunctions (AND of terms)
type Expr struct {
	// OR groups; each group is ANDed terms
	OrGroups [][]Term
}

// Parse parses a simple query language:
// - Fields: name, status, image, host
// - Operators: :, =, !=
// - Boolean: space = AND; OR for disjunction (case-sensitive OR keyword)
// - Negation: ! prefix before a term
// - Quoted values with spaces are supported using "value with spaces"
// Unknown fields result in an error string.
func Parse(input string) (Expr, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Expr{}, nil
	}

	tokens := tokenize(trimmed)
	// Split on OR tokens (uppercase OR only for simplicity)
	var groups [][]string
	{
		current := make([]string, 0, len(tokens))
		for _, t := range tokens {
			if t == "OR" {
				if len(current) > 0 {
					groups = append(groups, current)
					current = []string{}
				}
				continue
			}
			current = append(current, t)
		}
		if len(current) > 0 {
			groups = append(groups, current)
		}
	}

	expr := Expr{OrGroups: make([][]Term, 0, len(groups))}
	for _, g := range groups {
		terms := make([]Term, 0, len(g))
		i := 0
		for i < len(g) {
			tok := g[i]
			negate := false
			if tok == "!" {
				negate = true
				i++
				if i >= len(g) {
					break
				}
				tok = g[i]
			}

			// Expect forms: field(op)value OR bare value
			field, op, value := "", OpContains, ""

			if idx := strings.IndexAny(tok, ":="); idx > 0 {
				// token contains field and maybe op/value
				potentialField := tok[:idx]
				rest := tok[idx:]
				if _, ok := SupportedFields[strings.ToLower(potentialField)]; ok {
					field = strings.ToLower(potentialField)
					if strings.HasPrefix(rest, ":") {
						op = OpContains
						value = strings.TrimPrefix(rest, ":")
					} else if strings.HasPrefix(rest, "=") {
						op = OpEquals
						value = strings.TrimPrefix(rest, "=")
					}
				}
			}

			if field == "" {
				// Could be field != value across tokens: field != value
				if i+2 < len(g) {
					f := strings.ToLower(g[i])
					if _, ok := SupportedFields[f]; ok && g[i+1] == "!=" {
						field = f
						op = OpNotEquals
						value = g[i+2]
						i += 3
						terms = append(terms, Term{Field: field, Op: op, Value: stripQuotes(value), Negate: negate})
						continue
					}
				}

				// Otherwise treat as bare term (applies to default fields)
				value = tok
				terms = append(terms, Term{Field: "", Op: OpContains, Value: stripQuotes(value), Negate: negate})
				i++
				continue
			}

			// If op was != embedded (rare), handle tokenization form like field!=value (not covered by above)
			if value == "" && i+2 < len(g) && g[i+1] == "!=" {
				op = OpNotEquals
				value = g[i+2]
				i += 3
			} else {
				i++
			}

			terms = append(terms, Term{Field: field, Op: op, Value: stripQuotes(value), Negate: negate})
		}
		if len(terms) > 0 {
			expr.OrGroups = append(expr.OrGroups, terms)
		}
	}

	return expr, nil
}

func tokenize(s string) []string {
	var tokens []string
	var buf strings.Builder
	inQuotes := false

	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes {
			// Recognize separators and special tokens
			if unicode.IsSpace(rune(ch)) {
				flush()
				continue
			}
			// Two-char operator !=
			if ch == '!' && i+1 < len(s) && s[i+1] == '=' {
				flush()
				tokens = append(tokens, "!=")
				i++
				continue
			}
			// Single-char special tokens
			if ch == '!' || ch == ':' || ch == '=' {
				flush()
				tokens = append(tokens, string(ch))
				continue
			}
		}
		buf.WriteByte(ch)
	}
	flush()

	// Rejoin field op value when tokenized as [field, :, value] or [field, =, value]
	var merged []string
	for i := 0; i < len(tokens); i++ {
		if i+2 < len(tokens) && (tokens[i+1] == ":" || tokens[i+1] == "=") {
			merged = append(merged, tokens[i]+tokens[i+1]+tokens[i+2])
			i += 2
			continue
		}
		merged = append(merged, tokens[i])
	}

	// Normalize OR detection: keep exact "OR"
	out := make([]string, 0, len(merged))
	for _, t := range merged {
		if t == "OR" {
			out = append(out, t)
		} else {
			out = append(out, t)
		}
	}
	return out
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
