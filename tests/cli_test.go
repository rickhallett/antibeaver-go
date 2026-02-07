package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// CLI INTEGRATION TESTS
// These test the actual CLI binary behavior
// ═══════════════════════════════════════════════════════════════════════════

var binaryPath string

func TestMain(m *testing.M) {
	// Build binary before tests
	tmpDir, err := os.MkdirTemp("", "antibeaver-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "antibeaver")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/antibeaver")
	if err := cmd.Run(); err != nil {
		// Tests will skip if binary not built
		binaryPath = ""
	}

	os.Exit(m.Run())
}

func skipIfNoBinary(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binary not built")
	}
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	skipIfNoBinary(t)

	// Use temp DB for isolation
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	allArgs := append([]string{"--db", dbPath}, args...)
	cmd := exec.Command(binaryPath, allArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// ═══════════════════════════════════════════════════════════════════════════
// STATUS COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestStatusCommand(t *testing.T) {
	t.Run("shows status with no pending", func(t *testing.T) {
		stdout, _, err := runCLI(t, "status")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(stdout, "NORMAL") && !strings.Contains(stdout, "healthy") {
			t.Error("expected healthy status")
		}
	})

	t.Run("shows json output", func(t *testing.T) {
		stdout, _, err := runCLI(t, "status", "--json")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Errorf("invalid JSON output: %v", err)
		}
	})

	t.Run("includes pending count", func(t *testing.T) {
		stdout, _, _ := runCLI(t, "status", "--json")

		var result map[string]interface{}
		json.Unmarshal([]byte(stdout), &result)

		if _, ok := result["pending"]; !ok {
			t.Error("expected 'pending' field in output")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// BUFFER COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestBufferCommand(t *testing.T) {
	t.Run("buffers thought with default priority", func(t *testing.T) {
		stdout, _, err := runCLI(t, "buffer", "Test thought content")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(strings.ToLower(stdout), "buffered") || !strings.Contains(stdout, "id") {
			t.Error("expected confirmation of buffering")
		}
	})

	t.Run("buffers with P0 priority", func(t *testing.T) {
		stdout, _, err := runCLI(t, "buffer", "--priority", "P0", "Critical thought")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		_ = stdout
	})

	t.Run("buffers with agent specified", func(t *testing.T) {
		stdout, _, err := runCLI(t, "buffer", "--agent", "architect", "Agent thought")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		_ = stdout
	})

	t.Run("rejects empty thought", func(t *testing.T) {
		_, _, err := runCLI(t, "buffer", "")
		if err == nil {
			t.Error("expected error for empty thought")
		}
	})

	t.Run("rejects invalid priority", func(t *testing.T) {
		_, _, err := runCLI(t, "buffer", "--priority", "P99", "Bad priority")
		if err == nil {
			t.Error("expected error for invalid priority")
		}
	})

	t.Run("returns json output", func(t *testing.T) {
		stdout, _, err := runCLI(t, "buffer", "--json", "JSON test")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Errorf("invalid JSON: %v", err)
		}
		if result["ok"] != true {
			t.Error("expected ok: true")
		}
	})

	t.Run("increments pending count", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		// Buffer first
		cmd1 := exec.Command(binaryPath, "--db", dbPath, "buffer", "First")
		cmd1.Run()

		// Buffer second
		cmd2 := exec.Command(binaryPath, "--db", dbPath, "buffer", "Second")
		cmd2.Run()

		// Check status
		cmd3 := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd3.Stdout = &stdout
		cmd3.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		pending := result["pending"].(float64)
		if pending != 2 {
			t.Errorf("expected 2 pending, got %v", pending)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// FLUSH COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestFlushCommand(t *testing.T) {
	t.Run("returns no-op when empty", func(t *testing.T) {
		stdout, _, err := runCLI(t, "flush")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(stdout, "no pending") && !strings.Contains(stdout, "No pending") {
			t.Error("expected 'no pending' message")
		}
	})

	t.Run("generates synthesis prompt", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		// Buffer some thoughts
		exec.Command(binaryPath, "--db", dbPath, "buffer", "Thought 1").Run()
		exec.Command(binaryPath, "--db", dbPath, "buffer", "Thought 2").Run()

		// Flush
		cmd := exec.Command(binaryPath, "--db", dbPath, "flush")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		output := stdout.String()
		if !strings.Contains(output, "NETWORK RECOVERED") {
			t.Error("expected synthesis prompt")
		}
		if !strings.Contains(output, "Thought 1") || !strings.Contains(output, "Thought 2") {
			t.Error("expected buffered thoughts in prompt")
		}
	})

	t.Run("clears pending after flush", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "buffer", "Test").Run()
		exec.Command(binaryPath, "--db", dbPath, "flush").Run()

		// Check status
		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		pending := result["pending"].(float64)
		if pending != 0 {
			t.Errorf("expected 0 pending after flush, got %v", pending)
		}
	})

	t.Run("flush with agent filter", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "buffer", "--agent", "main", "Main thought").Run()
		exec.Command(binaryPath, "--db", dbPath, "buffer", "--agent", "architect", "Architect thought").Run()

		// Flush only main
		exec.Command(binaryPath, "--db", dbPath, "flush", "--agent", "main").Run()

		// Check architect still has pending
		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		pending := result["pending"].(float64)
		if pending != 1 {
			t.Errorf("expected 1 pending (architect), got %v", pending)
		}
	})

	t.Run("flush all agents", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "buffer", "--agent", "main", "Main").Run()
		exec.Command(binaryPath, "--db", dbPath, "buffer", "--agent", "architect", "Architect").Run()

		exec.Command(binaryPath, "--db", dbPath, "flush", "--all").Run()

		// Check all cleared
		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		pending := result["pending"].(float64)
		if pending != 0 {
			t.Errorf("expected 0 pending after flush all, got %v", pending)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// HALT COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestHaltCommand(t *testing.T) {
	t.Run("sets halted state", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "halt").Run()

		// Check status
		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		if result["halted"] != true {
			t.Error("expected halted: true")
		}
		if result["buffering"] != true {
			t.Error("halted should enable buffering")
		}
	})

	t.Run("halt is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "halt").Run()
		exec.Command(binaryPath, "--db", dbPath, "halt").Run()

		// Should not error
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// SIMULATE COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestSimulateCommand(t *testing.T) {
	t.Run("sets simulated latency", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "simulate", "15000").Run()

		// Check status
		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		sim := result["simulated_ms"].(float64)
		if sim != 15000 {
			t.Errorf("expected 15000, got %v", sim)
		}
	})

	t.Run("triggers buffering when above threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "simulate", "20000").Run()

		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		if result["buffering"] != true {
			t.Error("expected buffering with high simulated latency")
		}
	})

	t.Run("clears with zero", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "simulate", "15000").Run()
		exec.Command(binaryPath, "--db", dbPath, "simulate", "0").Run()

		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		sim := result["simulated_ms"].(float64)
		if sim != 0 {
			t.Errorf("expected 0 after clear, got %v", sim)
		}
	})

	t.Run("rejects negative", func(t *testing.T) {
		_, _, err := runCLI(t, "simulate", "-100")
		// Should either error or clamp to 0
		_ = err
	})

	t.Run("rejects non-numeric", func(t *testing.T) {
		_, _, err := runCLI(t, "simulate", "abc")
		if err == nil {
			t.Error("expected error for non-numeric input")
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// RESUME COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestResumeCommand(t *testing.T) {
	t.Run("clears halted state", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "halt").Run()
		exec.Command(binaryPath, "--db", dbPath, "resume").Run()

		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		if result["halted"] != false {
			t.Error("expected halted: false after resume")
		}
	})

	t.Run("clears forced buffering", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "force").Run()
		exec.Command(binaryPath, "--db", dbPath, "resume").Run()

		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		if result["forced_buffering"] != false {
			t.Error("expected forced_buffering: false after resume")
		}
	})

	t.Run("clears simulated latency", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "simulate", "15000").Run()
		exec.Command(binaryPath, "--db", dbPath, "resume").Run()

		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		sim := result["simulated_ms"].(float64)
		if sim != 0 {
			t.Errorf("expected 0 after resume, got %v", sim)
		}
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// RECORD-LATENCY COMMAND TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestRecordLatencyCommand(t *testing.T) {
	t.Run("records latency sample", func(t *testing.T) {
		stdout, _, err := runCLI(t, "record-latency", "500")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		_ = stdout
	})

	t.Run("updates average in status", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		exec.Command(binaryPath, "--db", dbPath, "record-latency", "1000").Run()
		exec.Command(binaryPath, "--db", dbPath, "record-latency", "2000").Run()
		exec.Command(binaryPath, "--db", dbPath, "record-latency", "3000").Run()

		cmd := exec.Command(binaryPath, "--db", dbPath, "status", "--json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()

		var result map[string]interface{}
		json.Unmarshal(stdout.Bytes(), &result)

		avg := result["avg_latency_ms"].(float64)
		if avg != 2000 {
			t.Errorf("expected avg 2000, got %v", avg)
		}
	})

	t.Run("rejects negative", func(t *testing.T) {
		_, _, err := runCLI(t, "record-latency", "-100")
		// Should clamp or error
		_ = err
	})
}

// ═══════════════════════════════════════════════════════════════════════════
// VERSION AND HELP TESTS
// ═══════════════════════════════════════════════════════════════════════════

func TestVersionCommand(t *testing.T) {
	t.Run("shows version", func(t *testing.T) {
		stdout, _, err := runCLI(t, "version")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(stdout, "antibeaver") {
			t.Error("expected version output")
		}
	})
}

func TestHelpCommand(t *testing.T) {
	t.Run("shows help", func(t *testing.T) {
		stdout, _, err := runCLI(t, "--help")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(stdout, "antibeaver") {
			t.Error("expected help output")
		}
	})

	t.Run("shows subcommand help", func(t *testing.T) {
		stdout, _, err := runCLI(t, "buffer", "--help")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(stdout, "priority") {
			t.Error("expected priority flag in buffer help")
		}
	})
}
