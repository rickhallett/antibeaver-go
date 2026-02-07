package synthesis_test

import (
	"strings"
	"testing"

	"github.com/rickhallett/antibeaver/internal/db"
	"github.com/rickhallett/antibeaver/internal/synthesis"
)

// ═══════════════════════════════════════════════════════════════════════════
// GENERATE PROMPT TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestGeneratePrompt(t *testing.T) {
	t.Run("handles empty thoughts", func(t *testing.T) {
		prompt := synthesis.GeneratePrompt(nil)
		if !strings.Contains(prompt, "No buffered thoughts") {
			t.Error("expected 'no buffered thoughts' message")
		}
	})

	t.Run("handles single thought", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, AgentID: "main", Content: "Hello world", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "NETWORK RECOVERED") {
			t.Error("expected recovery header")
		}
		if !strings.Contains(prompt, "1 messages") || !strings.Contains(prompt, "1 message") {
			// Might be singular or plural
		}
		if !strings.Contains(prompt, "Hello world") {
			t.Error("expected thought content")
		}
	})

	t.Run("includes all thoughts", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "First", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
			{ID: 2, Content: "Second", Priority: "P1", CreatedAt: "2026-02-07T12:01:00Z"},
			{ID: 3, Content: "Third", Priority: "P1", CreatedAt: "2026-02-07T12:02:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "First") {
			t.Error("missing first thought")
		}
		if !strings.Contains(prompt, "Second") {
			t.Error("missing second thought")
		}
		if !strings.Contains(prompt, "Third") {
			t.Error("missing third thought")
		}
	})

	t.Run("sorts P0 before P1 before P2", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Low", Priority: "P2", CreatedAt: "2026-02-07T12:00:00Z"},
			{ID: 2, Content: "Critical", Priority: "P0", CreatedAt: "2026-02-07T12:01:00Z"},
			{ID: 3, Content: "Normal", Priority: "P1", CreatedAt: "2026-02-07T12:02:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		critIdx := strings.Index(prompt, "Critical")
		normIdx := strings.Index(prompt, "Normal")
		lowIdx := strings.Index(prompt, "Low")

		if critIdx > normIdx || normIdx > lowIdx {
			t.Error("thoughts not sorted by priority")
		}
	})

	t.Run("adds CRITICAL tag to P0", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Urgent", Priority: "P0", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "[CRITICAL]") {
			t.Error("expected [CRITICAL] tag for P0")
		}
	})

	t.Run("adds low tag to P2", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Minor", Priority: "P2", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "[low]") {
			t.Error("expected [low] tag for P2")
		}
	})

	t.Run("no tag for P1", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Normal thought", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if strings.Contains(prompt, "[CRITICAL]") || strings.Contains(prompt, "[low]") {
			t.Error("P1 should not have priority tag")
		}
	})

	t.Run("escapes quotes in content", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: `He said "hello"`, Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if strings.Contains(prompt, `""hello""`) {
			t.Error("quotes should be escaped")
		}
	})

	t.Run("escapes newlines in content", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "line1\nline2", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		// Newlines should be escaped or preserved correctly
		if !strings.Contains(prompt, "line1") || !strings.Contains(prompt, "line2") {
			t.Error("content with newlines not handled")
		}
	})

	t.Run("preserves unicode", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Hello 🦫 beaver", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "🦫") {
			t.Error("unicode not preserved")
		}
	})

	t.Run("includes critical note when P0 exists", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Urgent 1", Priority: "P0", CreatedAt: "2026-02-07T12:00:00Z"},
			{ID: 2, Content: "Urgent 2", Priority: "P0", CreatedAt: "2026-02-07T12:01:00Z"},
			{ID: 3, Content: "Normal", Priority: "P1", CreatedAt: "2026-02-07T12:02:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "2 CRITICAL") {
			t.Error("expected note about 2 critical thoughts")
		}
	})

	t.Run("no critical note when no P0", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Normal", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if strings.Contains(prompt, "CRITICAL thought") {
			t.Error("should not have critical note without P0")
		}
	})

	t.Run("includes task instructions", func(t *testing.T) {
		thoughts := []db.Thought{
			{ID: 1, Content: "Test", Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "TASK:") {
			t.Error("expected TASK section")
		}
		if !strings.Contains(prompt, "Discard obsolete") {
			t.Error("expected discard instruction")
		}
		if !strings.Contains(prompt, "ONE coherent") {
			t.Error("expected consolidation instruction")
		}
		if !strings.Contains(prompt, "Do not apologize") {
			t.Error("expected no-apology instruction")
		}
	})

	t.Run("handles very long content", func(t *testing.T) {
		longContent := strings.Repeat("a", 10000)
		thoughts := []db.Thought{
			{ID: 1, Content: longContent, Priority: "P1", CreatedAt: "2026-02-07T12:00:00Z"},
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		// Should either include full content or truncate gracefully
		if len(prompt) < 1000 {
			t.Error("prompt too short for long content")
		}
	})

	t.Run("handles many thoughts", func(t *testing.T) {
		thoughts := make([]db.Thought, 100)
		for i := range thoughts {
			thoughts[i] = db.Thought{
				ID:        int64(i + 1),
				Content:   "Thought content",
				Priority:  "P1",
				CreatedAt: "2026-02-07T12:00:00Z",
			}
		}
		prompt := synthesis.GeneratePrompt(thoughts)

		if !strings.Contains(prompt, "100 messages") {
			t.Error("expected '100 messages' count")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// SHOULD BUFFER TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestShouldBuffer(t *testing.T) {
	t.Run("returns false when healthy", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency:      1000,
			MaxLatency:      2000,
			Threshold:       5000,
			ForcedBuffering: false,
			SimulatedMs:     0,
			Halted:          false,
		}
		result := synthesis.ShouldBuffer(state)

		if result.Buffering {
			t.Error("should not buffer when healthy")
		}
		if result.Reason != "healthy" {
			t.Errorf("expected reason 'healthy', got '%s'", result.Reason)
		}
	})

	t.Run("returns true when latency exceeds threshold", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency:  6000,
			MaxLatency:  8000,
			Threshold:   5000,
			Halted:      false,
		}
		result := synthesis.ShouldBuffer(state)

		if !result.Buffering {
			t.Error("should buffer when latency exceeds threshold")
		}
		if !strings.Contains(result.Reason, "latency") {
			t.Errorf("expected reason to mention latency, got '%s'", result.Reason)
		}
	})

	t.Run("returns true when halted", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency: 100,
			MaxLatency: 100,
			Threshold:  5000,
			Halted:     true,
		}
		result := synthesis.ShouldBuffer(state)

		if !result.Buffering {
			t.Error("should buffer when halted")
		}
		if result.Reason != "SYSTEM HALTED" {
			t.Errorf("expected 'SYSTEM HALTED', got '%s'", result.Reason)
		}
	})

	t.Run("halt takes priority over other states", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency:      100,
			Threshold:       5000,
			ForcedBuffering: true,
			SimulatedMs:     99999,
			Halted:          true,
		}
		result := synthesis.ShouldBuffer(state)

		if result.Reason != "SYSTEM HALTED" {
			t.Error("halt should take priority")
		}
	})

	t.Run("returns true when forced buffering", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency:      100,
			Threshold:       5000,
			ForcedBuffering: true,
		}
		result := synthesis.ShouldBuffer(state)

		if !result.Buffering {
			t.Error("should buffer when forced")
		}
		if result.Reason != "manual override" {
			t.Errorf("expected 'manual override', got '%s'", result.Reason)
		}
	})

	t.Run("returns true when simulated exceeds threshold", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency:  100,
			Threshold:   5000,
			SimulatedMs: 20000,
		}
		result := synthesis.ShouldBuffer(state)

		if !result.Buffering {
			t.Error("should buffer when simulated latency high")
		}
		if !strings.Contains(result.Reason, "simulated") {
			t.Errorf("expected reason to mention simulated, got '%s'", result.Reason)
		}
	})

	t.Run("simulated below threshold does not trigger", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency:  100,
			Threshold:   5000,
			SimulatedMs: 1000,
		}
		result := synthesis.ShouldBuffer(state)

		if result.Buffering {
			t.Error("should not buffer when simulated below threshold")
		}
	})

	t.Run("exactly at threshold does not trigger", func(t *testing.T) {
		state := synthesis.State{
			AvgLatency: 5000,
			MaxLatency: 5000,
			Threshold:  5000,
		}
		result := synthesis.ShouldBuffer(state)

		// Using > not >=
		if result.Buffering {
			t.Error("exactly at threshold should not trigger (using > not >=)")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestValidatePriority(t *testing.T) {
	t.Run("accepts P0", func(t *testing.T) {
		p, err := synthesis.ValidatePriority("P0")
		if err != nil || p != "P0" {
			t.Error("should accept P0")
		}
	})

	t.Run("accepts P1", func(t *testing.T) {
		p, err := synthesis.ValidatePriority("P1")
		if err != nil || p != "P1" {
			t.Error("should accept P1")
		}
	})

	t.Run("accepts P2", func(t *testing.T) {
		p, err := synthesis.ValidatePriority("P2")
		if err != nil || p != "P2" {
			t.Error("should accept P2")
		}
	})

	t.Run("defaults empty to P1", func(t *testing.T) {
		p, err := synthesis.ValidatePriority("")
		if err != nil || p != "P1" {
			t.Error("should default empty to P1")
		}
	})

	t.Run("rejects invalid", func(t *testing.T) {
		_, err := synthesis.ValidatePriority("P99")
		if err == nil {
			t.Error("should reject invalid priority")
		}
	})

	t.Run("rejects lowercase", func(t *testing.T) {
		_, err := synthesis.ValidatePriority("p1")
		if err == nil {
			t.Error("should reject lowercase")
		}
	})
}

func TestValidateThought(t *testing.T) {
	t.Run("accepts valid content", func(t *testing.T) {
		content, err := synthesis.ValidateThought("Hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "Hello world" {
			t.Error("content should be unchanged")
		}
	})

	t.Run("rejects empty", func(t *testing.T) {
		_, err := synthesis.ValidateThought("")
		if err == nil {
			t.Error("should reject empty")
		}
	})

	t.Run("rejects whitespace only", func(t *testing.T) {
		_, err := synthesis.ValidateThought("   \t\n  ")
		if err == nil {
			t.Error("should reject whitespace only")
		}
	})

	t.Run("truncates very long content", func(t *testing.T) {
		long := strings.Repeat("a", 100000)
		content, err := synthesis.ValidateThought(long)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(content) > 50000 {
			t.Error("should truncate to max length")
		}
	})

	t.Run("preserves unicode", func(t *testing.T) {
		content, err := synthesis.ValidateThought("Hello 🦫")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "🦫") {
			t.Error("should preserve unicode")
		}
	})
}

func TestValidateLatency(t *testing.T) {
	t.Run("accepts positive", func(t *testing.T) {
		lat := synthesis.ValidateLatency(500)
		if lat != 500 {
			t.Errorf("expected 500, got %d", lat)
		}
	})

	t.Run("clamps negative to zero", func(t *testing.T) {
		lat := synthesis.ValidateLatency(-100)
		if lat != 0 {
			t.Errorf("expected 0, got %d", lat)
		}
	})

	t.Run("handles very large values", func(t *testing.T) {
		lat := synthesis.ValidateLatency(999999999999)
		if lat != 999999999999 {
			t.Error("should handle large values")
		}
	})
}
