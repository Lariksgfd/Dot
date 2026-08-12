package errors

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// LexError
// ---------------------------------------------------------------------------

func TestNewLexError_Fields(t *testing.T) {
	e := NewLexError("main.dot", 3, 14, 42, 5, "unterminated string literal")
	if e.File != "main.dot" || e.Line != 3 || e.Column != 14 || e.Offset != 42 {
		t.Errorf("location = %s:%d:%d offset %d, want main.dot:3:14 offset 42",
			e.File, e.Line, e.Column, e.Offset)
	}
	if e.Length != 5 {
		t.Errorf("Length = %d, want 5", e.Length)
	}
	if e.Message != "unterminated string literal" {
		t.Errorf("Message = %q", e.Message)
	}
	if e.Hint != "" {
		t.Errorf("Hint = %q, want empty", e.Hint)
	}
	if e.Severity != SeverityError {
		t.Errorf("Severity = %v, want SeverityError", e.Severity)
	}
}

func TestNewLexError_ClampsLength(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		if got := NewLexError("f", 1, 1, 0, n, "m").Length; got != 1 {
			t.Errorf("NewLexError(length=%d).Length = %d, want 1", n, got)
		}
	}
}

func TestLexError_Error(t *testing.T) {
	e := NewLexError("main.dot", 3, 14, 42, 5, "unterminated string literal")
	if got, want := e.Error(), "main.dot:3:14: unterminated string literal"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	h := e.WithHint("close it with '\"'")
	want := `main.dot:3:14: unterminated string literal (hint: close it with '"')`
	if got := h.Error(); got != want {
		t.Errorf("Error() with hint = %q, want %q", got, want)
	}
}

func TestLexError_WithHint_DoesNotMutate(t *testing.T) {
	e := NewLexError("f", 1, 1, 0, 1, "m")
	h := e.WithHint("a hint")
	if e.Hint != "" {
		t.Errorf("original Hint = %q, want empty", e.Hint)
	}
	if h.Hint != "a hint" {
		t.Errorf("copy Hint = %q, want %q", h.Hint, "a hint")
	}
	if h == e {
		t.Error("WithHint returned the same pointer, want a copy")
	}
}

func TestSeverity_String(t *testing.T) {
	if got := SeverityError.String(); got != "error" {
		t.Errorf("SeverityError.String() = %q, want %q", got, "error")
	}
	if got := SeverityWarning.String(); got != "warning" {
		t.Errorf("SeverityWarning.String() = %q, want %q", got, "warning")
	}
}

// ---------------------------------------------------------------------------
// ErrorList
// ---------------------------------------------------------------------------

func TestErrorList_AddLenErr(t *testing.T) {
	var l ErrorList
	if l.Len() != 0 {
		t.Errorf("Len() = %d, want 0", l.Len())
	}
	if l.Err() != nil {
		t.Errorf("Err() = %v, want nil on an empty list", l.Err())
	}

	e1 := NewLexError("f", 1, 1, 0, 1, "first")
	e2 := NewLexError("f", 2, 1, 5, 1, "second")
	if !l.Add(e1) {
		t.Error("Add returned false on the first error")
	}
	if !l.Add(e2) {
		t.Error("Add returned false on the second error")
	}
	if l.Len() != 2 {
		t.Errorf("Len() = %d, want 2", l.Len())
	}
	if l.Err() == nil {
		t.Error("Err() = nil, want the list itself")
	}
	if l.Err() != error(&l) {
		t.Error("Err() did not return the list itself")
	}
	if got, want := l.Error(), "f:1:1: first\nf:2:1: second"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if l.Truncated {
		t.Error("Truncated = true, want false")
	}
}

func TestErrorList_AddNilIsNoOp(t *testing.T) {
	var l ErrorList
	if !l.Add(nil) {
		t.Error("Add(nil) = false, want true")
	}
	if l.Len() != 0 {
		t.Errorf("Len() = %d, want 0", l.Len())
	}
}

func TestErrorList_MaxErrorsTruncation(t *testing.T) {
	var l ErrorList
	for i := 0; i < MaxErrors; i++ {
		if !l.Add(NewLexError("f", i+1, 1, i, 1, fmt.Sprintf("e%d", i))) {
			t.Fatalf("Add returned false at error %d, want true up to MaxErrors", i)
		}
	}
	if l.Len() != MaxErrors {
		t.Fatalf("Len() = %d, want %d", l.Len(), MaxErrors)
	}
	if l.Truncated {
		t.Error("Truncated = true before exceeding MaxErrors")
	}
	if l.Add(NewLexError("f", 999, 1, 0, 1, "overflow")) {
		t.Error("Add returned true past MaxErrors, want false")
	}
	if !l.Truncated {
		t.Error("Truncated = false after exceeding MaxErrors")
	}
	if l.Len() != MaxErrors {
		t.Errorf("Len() = %d after overflow, want %d", l.Len(), MaxErrors)
	}
	if !strings.Contains(l.Error(), "too many errors") {
		t.Errorf("Error() = %q, want it to mention 'too many errors'", l.Error())
	}
}

func TestErrorList_NilReceiver(t *testing.T) {
	var l *ErrorList
	if l.Len() != 0 {
		t.Errorf("nil.Len() = %d, want 0", l.Len())
	}
	if l.Err() != nil {
		t.Errorf("nil.Err() = %v, want nil", l.Err())
	}
}

// ---------------------------------------------------------------------------
// FormatLexError
// ---------------------------------------------------------------------------

const sampleSource = "fn main() {\n    x = 1\n    name = \"hello\n}\n"

func TestFormatLexError_Shape(t *testing.T) {
	e := NewLexError("main.dot", 3, 12, 0, 6, "unterminated string literal").
		WithHint("strings must be closed on the same line")
	got := FormatLexError(e, sampleSource)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5:\n%s", len(lines), got)
	}
	if want := "error: unterminated string literal at main.dot:3:12"; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	if want := "  |"; lines[1] != want {
		t.Errorf("gutter line = %q, want %q", lines[1], want)
	}
	if want := "3 |     name = \"hello"; lines[2] != want {
		t.Errorf("source line = %q, want %q", lines[2], want)
	}
	if want := "  |            ^^^^^^"; lines[3] != want {
		t.Errorf("caret line = %q, want %q", lines[3], want)
	}
	if want := "  | strings must be closed on the same line"; lines[4] != want {
		t.Errorf("hint line = %q, want %q", lines[4], want)
	}
}

func TestFormatLexError_CaretRunLength(t *testing.T) {
	src := "abcdefghij\n"
	for _, n := range []int{1, 3, 10} {
		e := NewLexError("f.dot", 1, 1, 0, n, "boom")
		lines := strings.Split(FormatLexError(e, src), "\n")
		caret := strings.TrimPrefix(lines[len(lines)-1], "1 | ")
		caret = strings.TrimPrefix(caret, "  | ")
		if caret != strings.Repeat("^", n) {
			t.Errorf("length %d: caret = %q, want %q", n, caret, strings.Repeat("^", n))
		}
	}
}

func TestFormatLexError_NoHintOmitsHintLine(t *testing.T) {
	e := NewLexError("f.dot", 1, 1, 0, 1, "boom")
	got := FormatLexError(e, "abc\n")
	if strings.Count(got, "\n") != 3 {
		t.Errorf("got %d lines, want 4 (no hint line):\n%s", strings.Count(got, "\n")+1, got)
	}
}

func TestFormatLexError_MultiDigitLineWidensGutter(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 120; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	src := b.String()

	cases := []struct {
		line   int
		gutter string
	}{
		{7, "  |"},
		{42, "   |"},
		{120, "    |"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("line%d", tc.line), func(t *testing.T) {
			e := NewLexError("f.dot", tc.line, 1, 0, 1, "boom")
			lines := strings.Split(FormatLexError(e, src), "\n")
			if lines[1] != tc.gutter {
				t.Errorf("gutter line = %q, want %q", lines[1], tc.gutter)
			}
			wantSrc := fmt.Sprintf("%d | line %d", tc.line, tc.line)
			if lines[2] != wantSrc {
				t.Errorf("source line = %q, want %q", lines[2], wantSrc)
			}
			wantCaret := tc.gutter + " ^"
			if lines[3] != wantCaret {
				t.Errorf("caret line = %q, want %q", lines[3], wantCaret)
			}
		})
	}
}

func TestFormatLexError_TabsPreservedInCaretPadding(t *testing.T) {
	src := "\t\tx = 1\n"
	e := NewLexError("f.dot", 1, 3, 2, 1, "boom")
	lines := strings.Split(FormatLexError(e, src), "\n")
	if want := "  | \t\t^"; lines[3] != want {
		t.Errorf("caret line = %q, want %q", lines[3], want)
	}
}

func TestFormatLexError_UnicodeColumnsUseRunes(t *testing.T) {
	src := "日本語 = 1\n"
	e := NewLexError("f.dot", 1, 5, 0, 1, "boom")
	lines := strings.Split(FormatLexError(e, src), "\n")
	if want := "  |     ^"; lines[3] != want {
		t.Errorf("caret line = %q, want %q", lines[3], want)
	}
}

func TestFormatLexError_MissingSourceLine(t *testing.T) {
	e := NewLexError("f.dot", 99, 1, 0, 1, "boom").WithHint("a hint")
	got := FormatLexError(e, "only one line\n")
	if strings.Contains(got, "|") {
		t.Errorf("got a gutter for a missing source line:\n%s", got)
	}
	if !strings.Contains(got, "a hint") {
		t.Errorf("hint missing from %q", got)
	}
}

func TestFormatLexError_Nil(t *testing.T) {
	if got := FormatLexError(nil, "x"); got != "" {
		t.Errorf("FormatLexError(nil) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// FormatAll
// ---------------------------------------------------------------------------

func TestFormatAll_SummaryAndSeparation(t *testing.T) {
	src := "aaa\nbbb\n"
	var l ErrorList
	l.Add(NewLexError("f.dot", 1, 1, 0, 3, "first"))
	l.Add(NewLexError("f.dot", 2, 2, 5, 1, "second"))
	got := FormatAll(&l, src)

	if !strings.Contains(got, "error: first at f.dot:1:1") {
		t.Errorf("missing the first diagnostic:\n%s", got)
	}
	if !strings.Contains(got, "error: second at f.dot:2:2") {
		t.Errorf("missing the second diagnostic:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n\n2 errors") {
		t.Errorf("missing the '2 errors' summary:\n%s", got)
	}
}

func TestFormatAll_SingularSummary(t *testing.T) {
	var l ErrorList
	l.Add(NewLexError("f.dot", 1, 1, 0, 1, "only"))
	if got := FormatAll(&l, "abc\n"); !strings.HasSuffix(got, "1 error") {
		t.Errorf("summary = ...%q, want it to end with '1 error'", got)
	}
}

func TestFormatAll_Empty(t *testing.T) {
	var l ErrorList
	if got := FormatAll(&l, "abc"); got != "" {
		t.Errorf("FormatAll(empty) = %q, want empty", got)
	}
	if got := FormatAll(nil, "abc"); got != "" {
		t.Errorf("FormatAll(nil) = %q, want empty", got)
	}
}

func TestFormatAll_TruncationNote(t *testing.T) {
	var l ErrorList
	for i := 0; i <= MaxErrors; i++ {
		l.Add(NewLexError("f.dot", 1, 1, 0, 1, "boom"))
	}
	got := FormatAll(&l, "abc\n")
	if !strings.Contains(got, "too many errors") {
		t.Errorf("missing the truncation note:\n%s", got[max(0, len(got)-200):])
	}
}
