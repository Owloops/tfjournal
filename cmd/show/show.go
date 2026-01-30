package show

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
)

var (
	showOutput bool
	jsonOutput bool
)

var Cmd = &cobra.Command{
	Use:   "show <run-id>",
	Short: "Show details of a recorded run",
	Long: `Display detailed information about a specific terraform run.

Example:
  tfjournal show run_abc123
  tfjournal show run_abc123 --output`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

func init() {
	Cmd.Flags().BoolVar(&showOutput, "output", false, "Show captured output")
	Cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
}

func runShow(cmd *cobra.Command, args []string) error {
	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	runID := args[0]
	r, err := store.GetRun(runID)
	if err != nil {
		return err
	}

	if showOutput {
		output, err := store.GetOutput(runID)
		if err != nil {
			return fmt.Errorf("failed to read output: %w", err)
		}
		fmt.Print(string(output))
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	printRun(r)
	return nil
}

func printRun(r *run.Run) {
	width := 70
	border := strings.Repeat("─", width)

	fmt.Printf("┌%s┐\n", border)
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("run: %s", r.ID))
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("workspace: %s", r.Workspace))
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("status: %s", statusString(r.Status)))
	fmt.Printf("├%s┤\n", border)

	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("started:   %s", r.Timestamp.Format("2006-01-02 15:04:05")))
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("duration:  %s", r.Duration().Round(time.Second)))
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("program:   %s", r.Program))
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("command:   %s", strings.Join(r.Command, " ")))
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("user:      %s", r.User))

	if r.Git != nil {
		gitLine := fmt.Sprintf("%s (%s)", r.Git.Commit, r.Git.Branch)
		if r.Git.Dirty {
			gitLine += " [dirty]"
		}
		fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("git:       %s", gitLine))
	}

	if r.CI != nil {
		fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("ci:        %s", r.CI.Provider))
		if r.CI.Workflow != "" {
			fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("workflow:  %s", r.CI.Workflow))
		}
	}

	fmt.Printf("├%s┤\n", border)
	fmt.Printf("│  %-*s│\n", width-2, fmt.Sprintf("changes:   %s", r.ChangeSummary()))

	if len(r.Resources) > 0 {
		fmt.Printf("├%s┤\n", border)
		fmt.Printf("│  %-*s│\n", width-2, "resources:")
		for _, res := range r.Resources {
			action := actionIcon(res.Action)
			line := fmt.Sprintf("    %s %s", action, res.Address)
			if len(line) > width-4 {
				line = line[:width-7] + "..."
			}
			fmt.Printf("│  %-*s│\n", width-2, line)
		}
	}

	fmt.Printf("└%s┘\n", border)
}

func statusString(s run.Status) string {
	switch s {
	case run.StatusSuccess:
		return "✓ success"
	case run.StatusFailed:
		return "✗ failed"
	case run.StatusRunning:
		return "● running"
	case run.StatusCanceled:
		return "○ canceled"
	default:
		return string(s)
	}
}

func actionIcon(action string) string {
	switch action {
	case "create":
		return "+"
	case "update":
		return "~"
	case "destroy":
		return "-"
	default:
		return "?"
	}
}
