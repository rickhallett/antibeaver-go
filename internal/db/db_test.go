package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rickhallett/antibeaver/internal/db"
)

// ═══════════════════════════════════════════════════════════════════════════
// DATABASE INITIALIZATION TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestOpen(t *testing.T) {
	t.Run("creates new database in temp dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		d, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer d.Close()

		// File should exist
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("database file was not created")
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "nested", "dirs", "test.db")

		d, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer d.Close()
	})

	t.Run("opens existing database", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		// Create and close
		d1, _ := db.Open(dbPath)
		d1.Close()

		// Reopen
		d2, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("failed to reopen db: %v", err)
		}
		defer d2.Close()
	})

	t.Run("fails on invalid path", func(t *testing.T) {
		_, err := db.Open("/nonexistent/readonly/path/test.db")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})

	t.Run("opens in-memory database", func(t *testing.T) {
		d, err := db.Open(":memory:")
		if err != nil {
			t.Fatalf("failed to open in-memory db: %v", err)
		}
		defer d.Close()
	})

	t.Run("uses WAL mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		d, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer d.Close()

		mode, err := d.JournalMode()
		if err != nil {
			t.Fatalf("failed to get journal mode: %v", err)
		}
		if mode != "wal" {
			t.Errorf("expected WAL mode, got %s", mode)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// BUFFERED THOUGHTS TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestInsertThought(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	t.Run("inserts valid thought", func(t *testing.T) {
		id, err := d.InsertThought("main", "slack", "#ops", "Test thought", "P1")
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("inserts with P0 priority", func(t *testing.T) {
		id, err := d.InsertThought("main", "slack", "#ops", "Critical", "P0")
		if err != nil {
			t.Fatalf("failed to insert P0: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("inserts with P2 priority", func(t *testing.T) {
		id, err := d.InsertThought("main", "slack", "#ops", "Low", "P2")
		if err != nil {
			t.Fatalf("failed to insert P2: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("rejects invalid priority", func(t *testing.T) {
		_, err := d.InsertThought("main", "slack", "#ops", "Bad", "P99")
		if err == nil {
			t.Error("expected error for invalid priority")
		}
	})

	t.Run("handles empty content", func(t *testing.T) {
		_, err := d.InsertThought("main", "slack", "#ops", "", "P1")
		// Either reject or accept - document behavior
		_ = err
	})

	t.Run("handles very long content", func(t *testing.T) {
		longContent := make([]byte, 100000)
		for i := range longContent {
			longContent[i] = 'a'
		}
		id, err := d.InsertThought("main", "slack", "#ops", string(longContent), "P1")
		if err != nil {
			t.Fatalf("failed to insert long content: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("handles unicode content", func(t *testing.T) {
		id, err := d.InsertThought("main", "slack", "#ops", "Hello 🦫 beaver émoji", "P1")
		if err != nil {
			t.Fatalf("failed to insert unicode: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("handles special characters", func(t *testing.T) {
		id, err := d.InsertThought("main", "slack", "#ops", `"quotes" and 'apostrophes' and \backslash`, "P1")
		if err != nil {
			t.Fatalf("failed to insert special chars: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("handles newlines in content", func(t *testing.T) {
		id, err := d.InsertThought("main", "slack", "#ops", "line1\nline2\nline3", "P1")
		if err != nil {
			t.Fatalf("failed to insert with newlines: %v", err)
		}
		if id <= 0 {
			t.Errorf("expected positive ID, got %d", id)
		}
	})

	t.Run("auto-increments IDs", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		id1, _ := d.InsertThought("main", "slack", "#ops", "First", "P1")
		id2, _ := d.InsertThought("main", "slack", "#ops", "Second", "P1")

		if id2 <= id1 {
			t.Errorf("expected id2 > id1, got %d <= %d", id2, id1)
		}
	})
}

func TestGetPendingThoughts(t *testing.T) {
	t.Run("returns empty for new db", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		thoughts, err := d.GetPendingThoughts("main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(thoughts) != 0 {
			t.Errorf("expected 0 thoughts, got %d", len(thoughts))
		}
	})

	t.Run("returns pending thoughts for agent", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "Thought 1", "P1")
		d.InsertThought("main", "slack", "#ops", "Thought 2", "P1")

		thoughts, err := d.GetPendingThoughts("main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(thoughts) != 2 {
			t.Errorf("expected 2 thoughts, got %d", len(thoughts))
		}
	})

	t.Run("filters by agent ID", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "Main thought", "P1")
		d.InsertThought("architect", "slack", "#ops", "Architect thought", "P1")

		thoughts, err := d.GetPendingThoughts("main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(thoughts) != 1 {
			t.Errorf("expected 1 thought, got %d", len(thoughts))
		}
	})

	t.Run("excludes synthesized thoughts", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "Pending", "P1")
		d.InsertThought("main", "slack", "#ops", "To synthesize", "P1")
		d.MarkSynthesized("main", "output")

		thoughts, err := d.GetPendingThoughts("main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(thoughts) != 0 {
			t.Errorf("expected 0 pending after synthesis, got %d", len(thoughts))
		}
	})

	t.Run("orders by priority then time", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "Low priority", "P2")
		time.Sleep(10 * time.Millisecond)
		d.InsertThought("main", "slack", "#ops", "Critical", "P0")
		time.Sleep(10 * time.Millisecond)
		d.InsertThought("main", "slack", "#ops", "Normal", "P1")

		thoughts, err := d.GetPendingThoughts("main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(thoughts) != 3 {
			t.Fatalf("expected 3 thoughts, got %d", len(thoughts))
		}
		if thoughts[0].Priority != "P0" {
			t.Errorf("expected first thought to be P0, got %s", thoughts[0].Priority)
		}
		if thoughts[2].Priority != "P2" {
			t.Errorf("expected last thought to be P2, got %s", thoughts[2].Priority)
		}
	})

	t.Run("handles unknown agent", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		thoughts, err := d.GetPendingThoughts("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(thoughts) != 0 {
			t.Errorf("expected 0 thoughts for unknown agent, got %d", len(thoughts))
		}
	})
}

func TestGetPendingCount(t *testing.T) {
	t.Run("returns 0 for empty db", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		count, err := d.GetPendingCount("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})

	t.Run("counts all pending when agent empty", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "One", "P1")
		d.InsertThought("architect", "slack", "#ops", "Two", "P1")

		count, err := d.GetPendingCount("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2, got %d", count)
		}
	})

	t.Run("counts pending for specific agent", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "One", "P1")
		d.InsertThought("main", "slack", "#ops", "Two", "P1")
		d.InsertThought("architect", "slack", "#ops", "Three", "P1")

		count, err := d.GetPendingCount("main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2, got %d", count)
		}
	})
}

func TestGetPendingAgents(t *testing.T) {
	t.Run("returns empty for no pending", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		agents, err := d.GetPendingAgents()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agents) != 0 {
			t.Errorf("expected 0 agents, got %d", len(agents))
		}
	})

	t.Run("returns distinct agents", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "One", "P1")
		d.InsertThought("main", "slack", "#ops", "Two", "P1")
		d.InsertThought("architect", "slack", "#ops", "Three", "P1")

		agents, err := d.GetPendingAgents()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(agents))
		}
	})
}

func TestMarkSynthesized(t *testing.T) {
	t.Run("marks all pending for agent", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "One", "P1")
		d.InsertThought("main", "slack", "#ops", "Two", "P1")

		count, err := d.MarkSynthesized("main", "synthesized output")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2 marked, got %d", count)
		}

		pending, _ := d.GetPendingCount("main")
		if pending != 0 {
			t.Errorf("expected 0 pending after synthesis, got %d", pending)
		}
	})

	t.Run("only marks specified agent", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "Main", "P1")
		d.InsertThought("architect", "slack", "#ops", "Architect", "P1")

		d.MarkSynthesized("main", "output")

		pending, _ := d.GetPendingCount("architect")
		if pending != 1 {
			t.Errorf("expected architect still has 1 pending, got %d", pending)
		}
	})

	t.Run("handles no pending thoughts", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		count, err := d.MarkSynthesized("main", "output")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 marked, got %d", count)
		}
	})

	t.Run("creates synthesis event", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.InsertThought("main", "slack", "#ops", "Test", "P1")
		d.MarkSynthesized("main", "synthesized output")

		events, err := d.GetSynthesisEvents("main", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// NETWORK METRICS TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestRecordLatency(t *testing.T) {
	t.Run("records latency sample", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		err := d.RecordLatency(500)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("records multiple samples", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		for i := 0; i < 100; i++ {
			d.RecordLatency(int64(i * 10))
		}
	})
}

func TestGetAverageLatency(t *testing.T) {
	t.Run("returns 0 for empty", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		avg, err := d.GetAverageLatency(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if avg != 0 {
			t.Errorf("expected 0, got %d", avg)
		}
	})

	t.Run("calculates average", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.RecordLatency(100)
		d.RecordLatency(200)
		d.RecordLatency(300)

		avg, err := d.GetAverageLatency(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if avg != 200 {
			t.Errorf("expected 200, got %d", avg)
		}
	})
}

func TestGetMaxLatency(t *testing.T) {
	t.Run("returns 0 for empty", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		max, err := d.GetMaxLatency(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if max != 0 {
			t.Errorf("expected 0, got %d", max)
		}
	})

	t.Run("finds max", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.RecordLatency(100)
		d.RecordLatency(500)
		d.RecordLatency(200)

		max, err := d.GetMaxLatency(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if max != 500 {
			t.Errorf("expected 500, got %d", max)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// STATE TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestState(t *testing.T) {
	t.Run("gets and sets halt state", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.SetHalted(true)
		if !d.IsHalted() {
			t.Error("expected halted to be true")
		}

		d.SetHalted(false)
		if d.IsHalted() {
			t.Error("expected halted to be false")
		}
	})

	t.Run("gets and sets forced buffering", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.SetForcedBuffering(true)
		if !d.IsForcedBuffering() {
			t.Error("expected forced buffering to be true")
		}
	})

	t.Run("gets and sets simulated latency", func(t *testing.T) {
		d := openTestDB(t)
		defer d.Close()

		d.SetSimulatedLatency(15000)
		lat := d.GetSimulatedLatency()
		if lat != 15000 {
			t.Errorf("expected 15000, got %d", lat)
		}
	})

	t.Run("state persists across reopen", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "state.db")

		d1, _ := db.Open(dbPath)
		d1.SetHalted(true)
		d1.SetSimulatedLatency(5000)
		d1.Close()

		d2, _ := db.Open(dbPath)
		defer d2.Close()

		if !d2.IsHalted() {
			t.Error("halted state not persisted")
		}
		if d2.GetSimulatedLatency() != 5000 {
			t.Error("simulated latency not persisted")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPER
// ═══════════════════════════════════════════════════════════════════════════

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return d
}
