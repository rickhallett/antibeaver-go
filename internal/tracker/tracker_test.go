package tracker_test

import (
	"testing"
	"time"

	"github.com/rickhallett/antibeaver/internal/tracker"
)

// ═══════════════════════════════════════════════════════════════════════════
// CONSTRUCTOR TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestNewTracker(t *testing.T) {
	t.Run("creates with default max samples", func(t *testing.T) {
		tr := tracker.New()
		if tr == nil {
			t.Fatal("expected non-nil tracker")
		}
	})

	t.Run("creates with custom max samples", func(t *testing.T) {
		tr := tracker.NewWithMax(50)
		if tr == nil {
			t.Fatal("expected non-nil tracker")
		}
	})

	t.Run("handles zero max samples gracefully", func(t *testing.T) {
		tr := tracker.NewWithMax(0)
		// Should default to reasonable minimum or handle edge case
		if tr == nil {
			t.Fatal("expected non-nil tracker")
		}
	})

	t.Run("handles negative max samples gracefully", func(t *testing.T) {
		tr := tracker.NewWithMax(-10)
		if tr == nil {
			t.Fatal("expected non-nil tracker")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// RECORD TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestRecord(t *testing.T) {
	t.Run("records positive latency", func(t *testing.T) {
		tr := tracker.New()
		err := tr.Record(500)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tr.Count() != 1 {
			t.Errorf("expected count 1, got %d", tr.Count())
		}
	})

	t.Run("records zero latency", func(t *testing.T) {
		tr := tracker.New()
		err := tr.Record(0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("clamps negative latency to zero", func(t *testing.T) {
		tr := tracker.New()
		err := tr.Record(-500)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		avg := tr.Average(time.Minute, 999)
		if avg != 0 {
			t.Errorf("expected 0 (clamped), got %d", avg)
		}
	})

	t.Run("drops oldest when exceeding max", func(t *testing.T) {
		tr := tracker.NewWithMax(5)
		for i := 0; i < 10; i++ {
			tr.Record(int64(i * 100))
		}
		if tr.Count() != 5 {
			t.Errorf("expected count 5, got %d", tr.Count())
		}
		// Max should be 900 (last 5: 500,600,700,800,900)
		max := tr.Max(time.Minute, 0)
		if max != 900 {
			t.Errorf("expected max 900, got %d", max)
		}
	})

	t.Run("handles very large latency values", func(t *testing.T) {
		tr := tracker.New()
		err := tr.Record(999999999999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("handles concurrent records", func(t *testing.T) {
		tr := tracker.New()
		done := make(chan bool)
		for i := 0; i < 100; i++ {
			go func(val int) {
				tr.Record(int64(val))
				done <- true
			}(i)
		}
		for i := 0; i < 100; i++ {
			<-done
		}
		// Should not panic, count should be <= 100
		if tr.Count() > 100 {
			t.Errorf("count exceeded expected max")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// AVERAGE TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestAverage(t *testing.T) {
	t.Run("returns fallback when empty", func(t *testing.T) {
		tr := tracker.New()
		avg := tr.Average(time.Minute, 42)
		if avg != 42 {
			t.Errorf("expected fallback 42, got %d", avg)
		}
	})

	t.Run("returns fallback with zero fallback", func(t *testing.T) {
		tr := tracker.New()
		avg := tr.Average(time.Minute, 0)
		if avg != 0 {
			t.Errorf("expected fallback 0, got %d", avg)
		}
	})

	t.Run("calculates average of single sample", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		avg := tr.Average(time.Minute, 0)
		if avg != 100 {
			t.Errorf("expected 100, got %d", avg)
		}
	})

	t.Run("calculates average of multiple samples", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		tr.Record(200)
		tr.Record(300)
		avg := tr.Average(time.Minute, 0)
		if avg != 200 {
			t.Errorf("expected 200, got %d", avg)
		}
	})

	t.Run("excludes samples outside window", func(t *testing.T) {
		tr := tracker.New()
		// Inject old sample (would need test helper)
		tr.RecordWithTime(1000, time.Now().Add(-2*time.Minute))
		tr.Record(100)
		avg := tr.Average(time.Minute, 0)
		// Should only count the recent 100
		if avg != 100 {
			t.Errorf("expected 100 (recent only), got %d", avg)
		}
	})

	t.Run("handles very short window", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		time.Sleep(10 * time.Millisecond)
		avg := tr.Average(time.Millisecond, 999)
		// Might return fallback if sample aged out
		// This tests the window logic
		_ = avg // Just ensure no panic
	})

	t.Run("handles very long window", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		avg := tr.Average(24*time.Hour, 0)
		if avg != 100 {
			t.Errorf("expected 100, got %d", avg)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// MAX TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestMax(t *testing.T) {
	t.Run("returns fallback when empty", func(t *testing.T) {
		tr := tracker.New()
		max := tr.Max(time.Minute, 99)
		if max != 99 {
			t.Errorf("expected fallback 99, got %d", max)
		}
	})

	t.Run("returns max of single sample", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(500)
		max := tr.Max(time.Minute, 0)
		if max != 500 {
			t.Errorf("expected 500, got %d", max)
		}
	})

	t.Run("returns max of multiple samples", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		tr.Record(500)
		tr.Record(200)
		max := tr.Max(time.Minute, 0)
		if max != 500 {
			t.Errorf("expected 500, got %d", max)
		}
	})

	t.Run("excludes samples outside window", func(t *testing.T) {
		tr := tracker.New()
		tr.RecordWithTime(9999, time.Now().Add(-2*time.Minute))
		tr.Record(100)
		max := tr.Max(time.Minute, 0)
		if max != 100 {
			t.Errorf("expected 100 (recent only), got %d", max)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// CLEAR TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestClear(t *testing.T) {
	t.Run("removes all samples", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		tr.Record(200)
		tr.Clear()
		if tr.Count() != 0 {
			t.Errorf("expected count 0 after clear, got %d", tr.Count())
		}
	})

	t.Run("clear on empty is safe", func(t *testing.T) {
		tr := tracker.New()
		tr.Clear() // Should not panic
		if tr.Count() != 0 {
			t.Errorf("expected count 0, got %d", tr.Count())
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// SERIALIZATION TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestJSON(t *testing.T) {
	t.Run("serializes empty tracker", func(t *testing.T) {
		tr := tracker.New()
		data, err := tr.ToJSON()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) == 0 {
			t.Error("expected non-empty JSON")
		}
	})

	t.Run("serializes tracker with samples", func(t *testing.T) {
		tr := tracker.New()
		tr.Record(100)
		tr.Record(200)
		data, err := tr.ToJSON()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should contain count, average, max
		_ = data
	})
}
