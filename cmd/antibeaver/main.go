package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/fatih/color"
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
	noColor    bool
)

// Tokyo Night color palette
var (
	tokyoPurple  = color.New(color.FgHiMagenta)
	tokyoBlue    = color.New(color.FgHiCyan)
	tokyoGreen   = color.New(color.FgHiGreen)
	tokyoYellow  = color.New(color.FgHiYellow)
	tokyoRed     = color.New(color.FgHiRed)
	tokyoOrange  = color.New(color.FgYellow)
	tokyoMuted   = color.New(color.FgWhite)
	tokyoBold    = color.New(color.FgHiWhite, color.Bold)
	tokyoDim     = color.New(color.FgHiBlack)
)

const banner = `
   ╔═══════════════════════════════════════════════════════════╗
   ║                                                           ║
   ║     ░█▀█░█▀█░▀█▀░▀█▀░█▀▄░█▀▀░█▀█░█░█░█▀▀░█▀▄             ║
   ║     ░█▀█░█░█░░█░░░█░░█▀▄░█▀▀░█▀█░▀▄▀░█▀▀░█▀▄             ║
   ║     ░▀░▀░▀░▀░░▀░░▀▀▀░▀▀░░▀▀▀░▀░▀░░▀░░▀▀▀░▀░▀             ║
   ║                                                           ║
   ║           🦫  dam the flood  •  v%s               ║
   ║                                                           ║
   ╚═══════════════════════════════════════════════════════════╝
`

const bannerCompact = `
  ┌─────────────────────────────────────────┐
  │  🦫 antibeaver v%s                  │
  │     traffic governance for agents       │
  └─────────────────────────────────────────┘
`

func printBanner() {
	if outputJSON || noColor {
		return
	}
	tokyoPurple.Printf(banner, version)
	fmt.Println()
}

func printCompactBanner() {
	if outputJSON || noColor {
		return
	}
	tokyoBlue.Printf(bannerCompact, version)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "antibeaver",
		Short: "🦫 Dam the flood — traffic governance for multi-agent systems",
		Long: `antibeaver prevents distributed feedback loops in multi-agent AI systems
through circuit breaking, message buffering, and thought coalescing.

Dam it.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				color.NoColor = true
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			printBanner()
			tokyoMuted.Println("  Use 'antibeaver --help' for available commands")
			fmt.Println()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "Path to SQLite database")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colors")

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

			printCompactBanner()
			fmt.Println()

			// Status line
			if result.Buffering {
				tokyoYellow.Print("  ⚡ Status: ")
				tokyoOrange.Printf("BUFFERING")
				tokyoDim.Printf(" (%s)\n", result.Reason)
			} else {
				tokyoGreen.Print("  ✓ Status: ")
				tokyoBold.Print("NORMAL")
				tokyoDim.Printf(" (%s)\n", result.Reason)
			}

			// Pending
			tokyoBlue.Print("  ◆ Pending: ")
			if pending > 0 {
				tokyoYellow.Printf("%d thoughts\n", pending)
			} else {
				tokyoMuted.Println("0 thoughts")
			}

			// Latency
			tokyoBlue.Print("  ◆ Latency: ")
			tokyoMuted.Printf("avg %dms / max %dms", avgLatency, maxLatency)
			tokyoDim.Printf(" (threshold: %dms)\n", state.Threshold)

			// Warnings
			if halted {
				fmt.Println()
				tokyoRed.Println("  ⚠️  SYSTEM HALTED")
			}
			if forced {
				tokyoOrange.Println("  ⚡ Forced buffering enabled")
			}
			if simulated > 0 {
				tokyoPurple.Printf("  🔮 Simulated latency: %dms\n", simulated)
			}

			fmt.Println()
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

			tokyoGreen.Print("  ✓ ")
			tokyoMuted.Print("Buffered thought ")
			tokyoDim.Printf("(id: %d, priority: %s, agent: %s)\n", id, p, agentID)
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
				if len(agents) == 0 {
					tokyoDim.Println("  No pending thoughts to flush")
					return nil
				}
				for _, a := range agents {
					thoughts, _ := d.GetPendingThoughts(a)
					if len(thoughts) > 0 {
						tokyoPurple.Printf("\n  ═══ Agent: %s ═══\n\n", a)
						prompt := synthesis.GeneratePrompt(thoughts)
						fmt.Println(prompt)
						d.MarkSynthesized(a, prompt)
					}
				}
				return nil
			}

			thoughts, err := d.GetPendingThoughts(agentID)
			if err != nil {
				return err
			}

			if len(thoughts) == 0 {
				tokyoDim.Printf("  No pending thoughts for agent: %s\n", agentID)
				return nil
			}

			prompt := synthesis.GeneratePrompt(thoughts)
			fmt.Println(prompt)

			d.MarkSynthesized(agentID, prompt)
			tokyoGreen.Printf("\n  ✓ Synthesized %d thoughts\n", len(thoughts))
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
			tokyoRed.Println("  ⚠️  System HALTED")
			tokyoMuted.Println("     All messages will be buffered until resume")
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

			tokyoGreen.Println("  ✓ System RESUMED")
			tokyoMuted.Println("    Normal operations restored")
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
				tokyoGreen.Println("  ✓ Simulated latency cleared")
			} else {
				tokyoPurple.Printf("  🔮 Simulated latency set to %dms\n", ms)
				if ms > 5000 {
					tokyoOrange.Println("     ⚡ This will trigger buffering")
				}
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

			tokyoBlue.Printf("  ◆ Recorded latency: %dms\n", ms)
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
			tokyoOrange.Println("  ⚡ Forced buffering ENABLED")
			tokyoMuted.Println("     Use 'antibeaver resume' to clear")
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			if outputJSON {
				fmt.Printf(`{"version":"%s"}`+"\n", version)
				return
			}
			tokyoPurple.Printf("  🦫 antibeaver v%s\n", version)
			tokyoDim.Println("     traffic governance for multi-agent systems")
		},
	}
}
