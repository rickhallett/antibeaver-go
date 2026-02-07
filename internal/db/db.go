package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Thought represents a buffered thought
type Thought struct {
	ID        int64  `json:"id"`
	AgentID   string `json:"agent_id"`
	Channel   string `json:"channel"`
	Target    string `json:"target"`
	Content   string `json:"content"`
	Priority  string `json:"priority"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status"`
}

// SynthesisEvent represents a synthesis event
type SynthesisEvent struct {
	ID           int64  `json:"id"`
	AgentID      string `json:"agent_id"`
	ThoughtsCount int   `json:"thoughts_count"`
	FinalOutput  string `json:"final_output"`
	TriggeredAt  string `json:"triggered_at"`
}

// DB wraps the SQLite database
type DB struct {
	db *sql.DB
}

// Open opens or creates a database at the given path
func Open(path string) (*DB, error) {
	// Handle in-memory
	if path != ":memory:" {
		// Create parent directories
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	d := &DB{db: db}

	// Set WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	// Initialize schema
	if err := d.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return d, nil
}

func (d *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS buffered_thoughts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		channel TEXT NOT NULL,
		target TEXT DEFAULT '',
		content TEXT NOT NULL,
		priority TEXT DEFAULT 'P1' CHECK(priority IN ('P0', 'P1', 'P2')),
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		status TEXT DEFAULT 'pending'
	);
	
	CREATE INDEX IF NOT EXISTS idx_pending ON buffered_thoughts(agent_id, status) WHERE status = 'pending';
	
	CREATE TABLE IF NOT EXISTS network_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		latency_ms INTEGER NOT NULL,
		queue_depth INTEGER,
		recorded_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	
	CREATE TABLE IF NOT EXISTS synthesis_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		thoughts_count INTEGER,
		final_output TEXT,
		triggered_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	
	CREATE TABLE IF NOT EXISTS state (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	`
	_, err := d.db.Exec(schema)
	return err
}

// Close closes the database
func (d *DB) Close() error {
	return d.db.Close()
}

// JournalMode returns the current journal mode
func (d *DB) JournalMode() (string, error) {
	var mode string
	err := d.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	return mode, err
}

// InsertThought inserts a new buffered thought
func (d *DB) InsertThought(agentID, channel, target, content, priority string) (int64, error) {
	// Validate priority
	switch priority {
	case "P0", "P1", "P2":
		// valid
	default:
		return 0, fmt.Errorf("invalid priority: %s", priority)
	}

	result, err := d.db.Exec(
		`INSERT INTO buffered_thoughts (agent_id, channel, target, content, priority) VALUES (?, ?, ?, ?, ?)`,
		agentID, channel, target, content, priority,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetPendingThoughts returns pending thoughts for an agent, sorted by priority then time
func (d *DB) GetPendingThoughts(agentID string) ([]Thought, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, channel, target, content, priority, created_at, status
		FROM buffered_thoughts
		WHERE agent_id = ? AND status = 'pending'
		ORDER BY 
			CASE priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 END,
			created_at ASC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thoughts []Thought
	for rows.Next() {
		var t Thought
		if err := rows.Scan(&t.ID, &t.AgentID, &t.Channel, &t.Target, &t.Content, &t.Priority, &t.CreatedAt, &t.Status); err != nil {
			return nil, err
		}
		thoughts = append(thoughts, t)
	}
	return thoughts, rows.Err()
}

// GetPendingCount returns count of pending thoughts (all agents if agentID empty)
func (d *DB) GetPendingCount(agentID string) (int, error) {
	var count int
	var err error
	if agentID == "" {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM buffered_thoughts WHERE status = 'pending'`).Scan(&count)
	} else {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM buffered_thoughts WHERE agent_id = ? AND status = 'pending'`, agentID).Scan(&count)
	}
	return count, err
}

// GetPendingAgents returns distinct agent IDs with pending thoughts
func (d *DB) GetPendingAgents() ([]string, error) {
	rows, err := d.db.Query(`SELECT DISTINCT agent_id FROM buffered_thoughts WHERE status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// MarkSynthesized marks all pending thoughts for an agent as synthesized
func (d *DB) MarkSynthesized(agentID, output string) (int, error) {
	// Get count first
	count, err := d.GetPendingCount(agentID)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}

	// Update status
	_, err = d.db.Exec(
		`UPDATE buffered_thoughts SET status = 'synthesized' WHERE agent_id = ? AND status = 'pending'`,
		agentID,
	)
	if err != nil {
		return 0, err
	}

	// Log synthesis event
	_, err = d.db.Exec(
		`INSERT INTO synthesis_events (agent_id, thoughts_count, final_output) VALUES (?, ?, ?)`,
		agentID, count, output,
	)
	if err != nil {
		return count, err
	}

	return count, nil
}

// GetSynthesisEvents returns recent synthesis events for an agent
func (d *DB) GetSynthesisEvents(agentID string, limit int) ([]SynthesisEvent, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, thoughts_count, final_output, triggered_at
		FROM synthesis_events
		WHERE agent_id = ?
		ORDER BY triggered_at DESC
		LIMIT ?
	`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []SynthesisEvent
	for rows.Next() {
		var e SynthesisEvent
		var output sql.NullString
		if err := rows.Scan(&e.ID, &e.AgentID, &e.ThoughtsCount, &output, &e.TriggeredAt); err != nil {
			return nil, err
		}
		if output.Valid {
			e.FinalOutput = output.String
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// RecordLatency records a latency sample
func (d *DB) RecordLatency(latencyMs int64) error {
	_, err := d.db.Exec(`INSERT INTO network_metrics (latency_ms) VALUES (?)`, latencyMs)
	return err
}

// GetAverageLatency returns average latency from recent samples
func (d *DB) GetAverageLatency(windowMinutes int) (int64, error) {
	var avg sql.NullFloat64
	err := d.db.QueryRow(`
		SELECT AVG(latency_ms) 
		FROM network_metrics 
		WHERE recorded_at > datetime('now', '-' || ? || ' minutes')
	`, windowMinutes).Scan(&avg)
	if err != nil {
		return 0, err
	}
	if avg.Valid {
		return int64(avg.Float64), nil
	}
	return 0, nil
}

// GetMaxLatency returns max latency from recent samples
func (d *DB) GetMaxLatency(windowMinutes int) (int64, error) {
	var max sql.NullInt64
	err := d.db.QueryRow(`
		SELECT MAX(latency_ms) 
		FROM network_metrics 
		WHERE recorded_at > datetime('now', '-' || ? || ' minutes')
	`, windowMinutes).Scan(&max)
	if err != nil {
		return 0, err
	}
	if max.Valid {
		return max.Int64, nil
	}
	return 0, nil
}

// State getters/setters
func (d *DB) getState(key string) (string, error) {
	var value sql.NullString
	err := d.db.QueryRow(`SELECT value FROM state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if value.Valid {
		return value.String, nil
	}
	return "", nil
}

func (d *DB) setState(key, value string) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO state (key, value) VALUES (?, ?)`, key, value)
	return err
}

func (d *DB) IsHalted() bool {
	v, _ := d.getState("halted")
	return v == "true"
}

func (d *DB) SetHalted(halted bool) error {
	if halted {
		return d.setState("halted", "true")
	}
	return d.setState("halted", "false")
}

func (d *DB) IsForcedBuffering() bool {
	v, _ := d.getState("forced_buffering")
	return v == "true"
}

func (d *DB) SetForcedBuffering(forced bool) error {
	if forced {
		return d.setState("forced_buffering", "true")
	}
	return d.setState("forced_buffering", "false")
}

func (d *DB) GetSimulatedLatency() int64 {
	v, _ := d.getState("simulated_ms")
	if v == "" {
		return 0
	}
	var ms int64
	fmt.Sscanf(v, "%d", &ms)
	return ms
}

func (d *DB) SetSimulatedLatency(ms int64) error {
	return d.setState("simulated_ms", fmt.Sprintf("%d", ms))
}
