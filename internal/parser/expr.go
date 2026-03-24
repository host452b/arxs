package parser

import (
	"strings"
)

// ParseExpr parses a search expression like "A or B and C" and prefixes each
// term with the given field (e.g. "ti", "au", "abs").
// Operator precedence: AND > OR. No explicit operators between words = implicit AND.
func ParseExpr(expr string, field string) string {
	tokens := tokenize(expr)
	n := parseOr(tokens)
	return nodeToString(n, field)
}

// ParseAllFields expands a search expression so each term searches across
// ti, abs, and au with OR.
func ParseAllFields(expr string) string {
	tokens := tokenize(expr)
	n := parseOr(tokens)
	return allFieldsToString(n)
}

// CombineExprs joins multiple query expressions with AND or OR.
func CombineExprs(exprs []string, op string) string {
	if len(exprs) == 0 {
		return ""
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	opUpper := strings.ToUpper(op)
	return "(" + strings.Join(exprs, " "+opUpper+" ") + ")"
}

// AST

type node interface{ isNode() }

type termNode struct{ value string }

func (termNode) isNode() {}

type binaryNode struct {
	op    string // "AND" or "OR"
	left  node
	right node
}

func (binaryNode) isNode() {}

// Tokenizer

type tokenStream struct {
	tokens []string
	pos    int
}

func tokenize(expr string) *tokenStream {
	return &tokenStream{tokens: strings.Fields(expr)}
}

func (ts *tokenStream) peek() string {
	if ts.pos >= len(ts.tokens) {
		return ""
	}
	return ts.tokens[ts.pos]
}

func (ts *tokenStream) next() string {
	tok := ts.peek()
	ts.pos++
	return tok
}

func (ts *tokenStream) hasMore() bool {
	return ts.pos < len(ts.tokens)
}

func isOr(tok string) bool  { return strings.EqualFold(tok, "or") }
func isAnd(tok string) bool { return strings.EqualFold(tok, "and") }

// Recursive descent parser
// or_expr  = and_expr ("or" and_expr)*
// and_expr = term (("and" | implicit) term)*
// term     = word

func parseOr(ts *tokenStream) node {
	left := parseAnd(ts)
	for ts.hasMore() && isOr(ts.peek()) {
		ts.next()
		right := parseAnd(ts)
		left = binaryNode{op: "OR", left: left, right: right}
	}
	return left
}

func parseAnd(ts *tokenStream) node {
	left := parseTerm(ts)
	for ts.hasMore() && !isOr(ts.peek()) {
		if isAnd(ts.peek()) {
			ts.next()
		}
		right := parseTerm(ts)
		left = binaryNode{op: "AND", left: left, right: right}
	}
	return left
}

func parseTerm(ts *tokenStream) node {
	return termNode{value: ts.next()}
}

// AST to string — flattens consecutive same-op nodes for cleaner output

func flattenOp(n node, op string) []node {
	if b, ok := n.(binaryNode); ok && b.op == op {
		var out []node
		out = append(out, flattenOp(b.left, op)...)
		out = append(out, flattenOp(b.right, op)...)
		return out
	}
	return []node{n}
}

func nodeToString(n node, field string) string {
	switch v := n.(type) {
	case termNode:
		return field + ":" + v.value
	case binaryNode:
		children := flattenOp(v, v.op)
		parts := make([]string, len(children))
		for i, c := range children {
			parts[i] = nodeToString(c, field)
		}
		return "(" + strings.Join(parts, " "+v.op+" ") + ")"
	}
	return ""
}

func allFieldsToString(n node) string {
	switch v := n.(type) {
	case termNode:
		return "(ti:" + v.value + " OR abs:" + v.value + " OR au:" + v.value + ")"
	case binaryNode:
		children := flattenOp(v, v.op)
		parts := make([]string, len(children))
		for i, c := range children {
			parts[i] = allFieldsToString(c)
		}
		return "(" + strings.Join(parts, " "+v.op+" ") + ")"
	}
	return ""
}
