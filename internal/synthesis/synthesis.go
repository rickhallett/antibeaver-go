package synthesis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rickhallett/antibeaver/internal/db"
)

// State represents the current system state for buffering decisions
type State struct {
	AvgLatency      int64
	MaxLatency      int64
	Threshold       int64
	ForcedBuffering bool
	SimulatedMs     int64
	Halted          bool
}

// BufferResult represents the result of a buffering decision
type BufferResult struct {
	Buffering bool
	Reason    string
	LatencyMs int64
}

// ShouldBuffer determines if buffering should be active
func ShouldBuffer(state State) BufferResult {
	// Halt takes priority
	if state.Halted {
		return BufferResult{
			Buffering: true,
			Reason:    "SYSTEM HALTED",
			LatencyMs: 0,
		}
	}

	// Forced buffering
	if state.ForcedBuffering {
		return BufferResult{
			Buffering: true,
			Reason:    "manual override",
			LatencyMs: state.AvgLatency,
		}
	}

	// Simulated latency
	if state.SimulatedMs > state.Threshold {
		return BufferResult{
			Buffering: true,
			Reason:    fmt.Sprintf("simulated %dms", state.SimulatedMs),
			LatencyMs: state.SimulatedMs,
		}
	}

	// Real latency - use max for sensitivity
	if state.MaxLatency > state.Threshold {
		return BufferResult{
			Buffering: true,
			Reason:    fmt.Sprintf("latency %dms > %dms", state.MaxLatency, state.Threshold),
			LatencyMs: state.MaxLatency,
		}
	}

	return BufferResult{
		Buffering: false,
		Reason:    "healthy",
		LatencyMs: state.AvgLatency,
	}
}

// GeneratePrompt creates a synthesis prompt from buffered thoughts
func GeneratePrompt(thoughts []db.Thought) string {
	if len(thoughts) == 0 {
		return "**SYSTEM: No buffered thoughts to synthesize.**"
	}

	// Sort by priority (P0 < P1 < P2) then by time
	sorted := make([]db.Thought, len(thoughts))
	copy(sorted, thoughts)
	sort.Slice(sorted, func(i, j int) bool {
		pi := priorityOrder(sorted[i].Priority)
		pj := priorityOrder(sorted[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return sorted[i].CreatedAt < sorted[j].CreatedAt
	})

	var lines []string
	for i, t := range sorted {
		tag := ""
		switch t.Priority {
		case "P0":
			tag = " [CRITICAL]"
		case "P2":
			tag = " [low]"
		}
		escaped := escapeContent(t.Content)
		lines = append(lines, fmt.Sprintf("%d. [%s]%s \"%s\"", i+1, t.CreatedAt, tag, escaped))
	}

	// Count P0 thoughts
	p0Count := 0
	for _, t := range sorted {
		if t.Priority == "P0" {
			p0Count++
		}
	}

	criticalNote := ""
	if p0Count > 0 {
		criticalNote = fmt.Sprintf("\n\n**Note:** %d CRITICAL thought(s) — preserve unless clearly obsolete.", p0Count)
	}

	messageWord := "messages"
	if len(thoughts) == 1 {
		messageWord = "message"
	}

	return fmt.Sprintf(`**SYSTEM: NETWORK RECOVERED**

While congested, you drafted %d %s:

%s%s

**TASK:** Review against current channel state.
- Discard obsolete/superseded thoughts
- Synthesize remaining into ONE coherent message
- Do not apologize or mention delays`, len(thoughts), messageWord, strings.Join(lines, "\n"), criticalNote)
}

func priorityOrder(p string) int {
	switch p {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	default:
		return 1
	}
}

func escapeContent(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// ValidatePriority validates and normalizes priority
func ValidatePriority(p string) (string, error) {
	if p == "" {
		return "P1", nil
	}
	switch p {
	case "P0", "P1", "P2":
		return p, nil
	default:
		return "", fmt.Errorf("invalid priority: %s (must be P0, P1, or P2)", p)
	}
}

// ValidateThought validates thought content
func ValidateThought(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", fmt.Errorf("thought content cannot be empty")
	}
	// Truncate at 50KB
	if len(content) > 50000 {
		content = content[:50000]
	}
	return content, nil
}

// ValidateLatency validates and clamps latency value
func ValidateLatency(ms int64) int64 {
	if ms < 0 {
		return 0
	}
	return ms
}
