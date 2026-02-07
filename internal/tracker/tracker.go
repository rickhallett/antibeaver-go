package tracker

import (
	"encoding/json"
	"sync"
	"time"
)

// Sample represents a single latency measurement
type Sample struct {
	Timestamp time.Time
	LatencyMs int64
}

// Tracker tracks latency samples with a rolling window
type Tracker struct {
	mu         sync.RWMutex
	samples    []Sample
	maxSamples int
}

// New creates a new tracker with default max samples (100)
func New() *Tracker {
	return NewWithMax(100)
}

// NewWithMax creates a new tracker with specified max samples
func NewWithMax(max int) *Tracker {
	if max <= 0 {
		max = 1 // Minimum 1 sample
	}
	return &Tracker{
		samples:    make([]Sample, 0, max),
		maxSamples: max,
	}
}

// Record adds a latency sample with current timestamp
func (t *Tracker) Record(latencyMs int64) error {
	return t.RecordWithTime(latencyMs, time.Now())
}

// RecordWithTime adds a latency sample with specified timestamp
func (t *Tracker) RecordWithTime(latencyMs int64, ts time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clamp negative to zero
	if latencyMs < 0 {
		latencyMs = 0
	}

	t.samples = append(t.samples, Sample{
		Timestamp: ts,
		LatencyMs: latencyMs,
	})

	// Drop oldest if over max
	if len(t.samples) > t.maxSamples {
		t.samples = t.samples[1:]
	}

	return nil
}

// Average returns the average latency within the window, or fallback if no samples
func (t *Tracker) Average(window time.Duration, fallback int64) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var sum int64
	var count int

	for _, s := range t.samples {
		if s.Timestamp.After(cutoff) {
			sum += s.LatencyMs
			count++
		}
	}

	if count == 0 {
		return fallback
	}
	return sum / int64(count)
}

// Max returns the maximum latency within the window, or fallback if no samples
func (t *Tracker) Max(window time.Duration, fallback int64) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var max int64 = -1

	for _, s := range t.samples {
		if s.Timestamp.After(cutoff) {
			if s.LatencyMs > max {
				max = s.LatencyMs
			}
		}
	}

	if max < 0 {
		return fallback
	}
	return max
}

// Count returns the current number of samples
func (t *Tracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.samples)
}

// Clear removes all samples
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.samples = t.samples[:0]
}

// Status represents tracker status for JSON output
type Status struct {
	Count      int   `json:"count"`
	AvgMs      int64 `json:"avg_ms"`
	MaxMs      int64 `json:"max_ms"`
	MaxSamples int   `json:"max_samples"`
}

// ToJSON serializes tracker status to JSON
func (t *Tracker) ToJSON() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	status := Status{
		Count:      len(t.samples),
		AvgMs:      t.Average(time.Minute, 0),
		MaxMs:      t.Max(time.Minute, 0),
		MaxSamples: t.maxSamples,
	}

	return json.Marshal(status)
}
