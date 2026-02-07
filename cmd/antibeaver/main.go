package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/rickhallett/antibeaver/internal/db"
	"github.com/rickhallett/antibeaver/internal/synthesis"
	"github.com/spf13/cobra"
)

var (
	version    = "0.3.0"
	dbPath     string
	outputJSON bool
	agentID    string
	priority   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "antibeaver",
		Short: "🦫 Dam the flood — traffic governance for multi-agent systems",
		Long: `antibeaver prevents distributed feedback loops in multi-agent AI systems
through circuit breaking, message buffering, and thought coalescing.

Dam it.`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "Path to SQLite database")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")

	// Add commands
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(bufferCmd())
	rootCmd.AddCommand(flushCmd())
	rootCmd.AddCommand(haltCmd())
	rootCmd.AddCommand(resumeCmd())
	rootCmd.AddCommand(simulateCmd())
	rootCmd.AddCommand(recordLatencyCmd())
	rootCmd.AddCommand(forceCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.openclaw/antibeaver/governance.db"
}

func openDB() (*db.DB, error) {
	return db.Open(dbPath)
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current system status",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			pending, _ := d.GetPendingCount("")
			halted := d.IsHalted()
			forced := d.IsForcedBuffering()
			simulated := d.GetSimulatedLatency()

			// Get latency from database
			avgLatency, _ := d.GetAverageLatency(1) // last 1 minute
			maxLatency, _ := d.GetMaxLatency(1)

			state := synthesis.State{
				AvgLatency:      avgLatency,
				MaxLatency:      maxLatency,
				Threshold:       5000,
				ForcedBuffering: forced,
				SimulatedMs:     simulated,
				Halted:          halted,
			}

			result := synthesis.ShouldBuffer(state)

			if outputJSON {
				out := map[string]interface{}{
					"pending":          pending,
					"halted":           halted,
					"forced_buffering": forced,
					"simulated_ms":     simulated,
					"buffering":        result.Buffering,
					"reason":           result.Reason,
					"avg_latency_ms":   state.AvgLatency,
					"max_latency_ms":   state.MaxLatency,
					"threshold_ms":     state.Threshold,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			status := "NORMAL"
			if result.Buffering {
				status = "BUFFERING"
			}

			fmt.Printf("Status: %s (%s)\n", status, result.Reason)
			fmt.Printf("Pending: %d thoughts\n", pending)
			if halted {
				fmt.Println("⚠️  SYSTEM HALTED")
			}
			return nil
		},
	}
}

func bufferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buffer [thought]",
		Short: "Buffer a thought for later synthesis",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := args[0]

			// Validate
			content, err := synthesis.ValidateThought(content)
			if err != nil {
				return err
			}

			p, err := synthesis.ValidatePriority(priority)
			if err != nil {
				return err
			}

			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			id, err := d.InsertThought(agentID, "cli", "", content, p)
			if err != nil {
				return err
			}

			if outputJSON {
				out := map[string]interface{}{
					"ok":       true,
					"id":       id,
					"agent":    agentID,
					"priority": p,
				}
				enc := json.NewEncoder(os.Stdout)
				return enc.Encode(out)
			}

			fmt.Printf("Buffered thought (id: %d, priority: %s)\n", id, p)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "main", "Agent ID")
	cmd.Flags().StringVar(&priority, "priority", "P1", "Priority (P0/P1/P2)")

	return cmd
}

func flushCmd() *cobra.Command {
	var flushAll bool
	cmd := &cobra.Command{
		Use:   "flush",
		Short: "Flush buffered thoughts and generate synthesis prompt",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			if flushAll {
				agents, err := d.GetPendingAgents()
				if err != nil {
					return err
				}
				for _, a := range agents {
					thoughts, _ := d.GetPendingThoughts(a)
					if len(thoughts) > 0 {
						prompt := synthesis.GeneratePrompt(thoughts)
						fmt.Printf("=== Agent: %s ===\n%s\n\n", a, prompt)
						d.MarkSynthesized(a, prompt)
					}
				}
				if len(agents) == 0 {
					fmt.Println("No pending thoughts to flush")
				}
				return nil
			}

			thoughts, err := d.GetPendingThoughts(agentID)
			if err != nil {
				return err
			}

			if len(thoughts) == 0 {
				fmt.Println("No pending thoughts for agent:", agentID)
				return nil
			}

			prompt := synthesis.GeneratePrompt(thoughts)
			fmt.Println(prompt)

			d.MarkSynthesized(agentID, prompt)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "main", "Agent ID")
	cmd.Flags().BoolVar(&flushAll, "all", false, "Flush all agents")

	return cmd
}

func haltCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "halt",
		Short: "Halt the system (force buffering)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			d.SetHalted(true)
			fmt.Println("⚠️  System HALTED — all messages will be buffered")
			return nil
		},
	}
}

func resumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume normal operations (clear halt, forced buffering, and simulated latency)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			d.SetHalted(false)
			d.SetForcedBuffering(false)
			d.SetSimulatedLatency(0)

			fmt.Println("✅ System RESUMED — normal operations restored")
			return nil
		},
	}
}

func simulateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "simulate [latency_ms]",
		Short: "Set simulated network latency",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ms, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid latency value: %s", args[0])
			}

			if ms < 0 {
				ms = 0
			}

			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			d.SetSimulatedLatency(ms)

			if ms == 0 {
				fmt.Println("Simulated latency cleared")
			} else {
				fmt.Printf("Simulated latency set to %dms\n", ms)
			}
			return nil
		},
	}
}

func recordLatencyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "record-latency [ms]",
		Short: "Record a latency sample",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ms, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid latency value: %s", args[0])
			}

			ms = synthesis.ValidateLatency(ms)

			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			d.RecordLatency(ms)

			if outputJSON {
				out := map[string]interface{}{
					"ok":         true,
					"latency_ms": ms,
				}
				enc := json.NewEncoder(os.Stdout)
				return enc.Encode(out)
			}

			fmt.Printf("Recorded latency: %dms\n", ms)
			return nil
		},
	}
}

func forceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "force",
		Short: "Force buffering on (manual override)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()

			d.SetForcedBuffering(true)
			fmt.Println("⚡ Forced buffering ENABLED")
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("antibeaver v%s\n", version)
		},
	}
}
