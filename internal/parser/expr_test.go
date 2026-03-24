package parser

import (
	"testing"
)

func TestParseSingleTerm(t *testing.T) {
	// "transformer" with field "ti" → "ti:transformer"
	got := ParseExpr("transformer", "ti")
	want := "ti:transformer"
	if got != want {
		t.Errorf("ParseExpr(%q, %q) = %q, want %q", "transformer", "ti", got, want)
	}
}

func TestParseOrExpr(t *testing.T) {
	// "transformer or attention" with field "ti" → "ti:transformer OR ti:attention"
	got := ParseExpr("transformer or attention", "ti")
	want := "(ti:transformer OR ti:attention)"
	if got != want {
		t.Errorf("ParseExpr(%q, %q) = %q, want %q", "transformer or attention", "ti", got, want)
	}
}

func TestParseAndExpr(t *testing.T) {
	// "vaswani and hinton" with field "au" → "au:vaswani AND au:hinton"
	got := ParseExpr("vaswani and hinton", "au")
	want := "(au:vaswani AND au:hinton)"
	if got != want {
		t.Errorf("ParseExpr(%q, %q) = %q, want %q", "vaswani and hinton", "au", got, want)
	}
}

func TestParseAndPrecedenceOverOr(t *testing.T) {
	// "A or B and C" → AND binds tighter → "A OR (B AND C)"
	got := ParseExpr("A or B and C", "ti")
	want := "(ti:A OR (ti:B AND ti:C))"
	if got != want {
		t.Errorf("ParseExpr(%q, %q) = %q, want %q", "A or B and C", "ti", got, want)
	}
}

func TestParseMultipleOr(t *testing.T) {
	got := ParseExpr("transformer or attention or self-attention", "ti")
	want := "(ti:transformer OR ti:attention OR ti:self-attention)"
	if got != want {
		t.Errorf("ParseExpr(%q, %q) = %q, want %q", "transformer or attention or self-attention", "ti", got, want)
	}
}

func TestParseImplicitAnd(t *testing.T) {
	// "reinforcement learning" (no operator) → implicit AND
	got := ParseExpr("reinforcement learning", "abs")
	want := "(abs:reinforcement AND abs:learning)"
	if got != want {
		t.Errorf("ParseExpr(%q, %q) = %q, want %q", "reinforcement learning", "abs", got, want)
	}
}

func TestParseAllFields(t *testing.T) {
	// ParseAllFields expands to (ti:X OR abs:X OR au:X)
	got := ParseAllFields("transformer")
	want := "(ti:transformer OR abs:transformer OR au:transformer)"
	if got != want {
		t.Errorf("ParseAllFields(%q) = %q, want %q", "transformer", got, want)
	}
}

func TestParseAllFieldsWithOr(t *testing.T) {
	got := ParseAllFields("transformer or attention")
	want := "((ti:transformer OR abs:transformer OR au:transformer) OR (ti:attention OR abs:attention OR au:attention))"
	if got != want {
		t.Errorf("ParseAllFields(%q) = %q, want %q", "transformer or attention", got, want)
	}
}

func TestCombineExprs(t *testing.T) {
	exprs := []string{"ti:transformer", "(au:vaswani AND au:hinton)"}

	gotAnd := CombineExprs(exprs, "and")
	wantAnd := "(ti:transformer AND (au:vaswani AND au:hinton))"
	if gotAnd != wantAnd {
		t.Errorf("CombineExprs AND = %q, want %q", gotAnd, wantAnd)
	}

	gotOr := CombineExprs(exprs, "or")
	wantOr := "(ti:transformer OR (au:vaswani AND au:hinton))"
	if gotOr != wantOr {
		t.Errorf("CombineExprs OR = %q, want %q", gotOr, wantOr)
	}
}
