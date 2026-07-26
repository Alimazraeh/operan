package execution

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/operan/modules/03-agent-orchestration/internal/events"
	"github.com/operan/modules/03-agent-orchestration/internal/store"
)

// arabicSentence is the shape of real content on this hot path — an agent
// draft or a request body — not a synthetic single repeated letter. Arabic
// letters are two bytes in UTF-8; mixed with 1-byte ASCII spaces, the
// resulting byte offsets are unaligned with character boundaries, so a
// byte-oriented cut has no reason to land on one.
const arabicSentence = "مرحبا بكم في منصة أوبران لإدارة العمليات والوكلاء الآليين "

// arabicText returns n copies of arabicSentence, long enough to exceed every
// n that bound() is called with in this package (max 8000).
func arabicText(copies int) string {
	return strings.Repeat(arabicSentence, copies)
}

// bound() must truncate by rune, not by byte. A byte-oriented cut through
// Arabic text (s[:n]) has no reason to land on a character boundary and
// routinely splits a multi-byte rune, producing invalid UTF-8.
//
// Verified against the pre-fix code (s[:n] byte slice): this test fails —
// utf8.ValidString is false and the rune count does not match — because the
// cut at byte offset 8000 lands inside an Arabic character. See the WO-1
// report for the exact revert/run/restore transcript.
func TestBoundIsRuneSafeOnArabicText(t *testing.T) {
	s := arabicText(300) // 32100 bytes / 17400 runes — well past every call-site bound
	got := bound(s, 8000)

	if !utf8.ValidString(got) {
		t.Fatalf("bound() produced invalid UTF-8 — a multi-byte rune was split: %q", got)
	}
	const wantRunes = 8001 // 8000 kept runes + the "…" marker
	if gotRunes := utf8.RuneCountInString(got); gotRunes != wantRunes {
		t.Fatalf("rune count = %d, want %d", gotRunes, wantRunes)
	}
}

// A minimal, hand-checkable case: one ASCII byte followed by ten two-byte
// Arabic runes. Cutting at byte offset 4 lands inside rune index 2 (which
// occupies bytes 3-4) — a direct demonstration that the old code's "n" was a
// byte offset, not a character count.
func TestBoundSplitsArabicRuneAtEvenByteOffset(t *testing.T) {
	s := "x" + strings.Repeat("م", 10) // 21 bytes, 11 runes
	got := bound(s, 4)

	if !utf8.ValidString(got) {
		t.Fatalf("bound() produced invalid UTF-8: %q", got)
	}
	const wantRunes = 5 // 4 kept runes + "…"
	if gotRunes := utf8.RuneCountInString(got); gotRunes != wantRunes {
		t.Fatalf("rune count = %d, want %d", gotRunes, wantRunes)
	}
}

// dag_engine.go bounds a completed node's "text" output into the
// node_completed event's details through its own code path, independent of
// bound() in node_handler.go. This proves that site specifically: a node
// producing long Arabic text must reach the execution history — what /state
// and the request timeline are reconstructed from — as valid, correctly
// bounded UTF-8.
func TestDAGEngineNodeOutputIsRuneSafeOnArabicText(t *testing.T) {
	longText := arabicText(300) // same shape as the node_handler.go case above

	st := store.NewWorkflowStore()
	wf := &store.Workflow{
		ID: "wf-arabic-truncation", TenantID: "t1", Status: store.WorkflowStatusPending,
		Graph: store.WorkflowGraph{Nodes: []store.WorkflowNode{
			{ID: "draft-1", Type: store.WorkflowNodeAction, Action: "Draft"},
		}},
	}
	if _, err := st.Create(wf); err != nil {
		t.Fatal(err)
	}

	handler := func(ctx context.Context, node store.WorkflowNode, workflowID string, variables map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"text": longText}, nil
	}
	eng := NewEngine(st, events.NewPublisher(), handler, events.StackLangGraph)
	if err := eng.StartWorkflow(wf.ID); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := st.GetByID(wf.ID)
		if got.Status == store.WorkflowStatusCompleted || got.Status == store.WorkflowStatusFailed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	var output string
	found := false
	for _, ev := range st.GetExecutionHistory(wf.ID) {
		if ev.EventType != "node_completed" {
			continue
		}
		output, found = ev.Details["output"].(string)
	}
	if !found {
		t.Fatal("no node_completed event carried an output field")
	}
	if !utf8.ValidString(output) {
		t.Fatalf("node_completed event's output is invalid UTF-8 — a multi-byte rune was split: %q", output)
	}
	const wantRunes = 8001 // 8000 kept runes + "…"
	if gotRunes := utf8.RuneCountInString(output); gotRunes != wantRunes {
		t.Fatalf("output rune count = %d, want %d", gotRunes, wantRunes)
	}
}
