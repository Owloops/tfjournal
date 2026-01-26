package list

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
)

var (
	since      string
	user       string
	status     string
	program    string
	branch     string
	hasChanges bool
	limit      int
	jsonOutput bool
)

var Cmd = &cobra.Command{
	Use:   "list [workspace-pattern]",
	Short: "List recorded runs",
	Long: `List terraform runs recorded by tfjournal.

Example:
  tfjournal list
  tfjournal list --since 7d
  tfjournal list --status failed
  tfjournal list --program tofu
  tfjournal list --branch main
  tfjournal list --has-changes
  tfjournal list production/*`,
	RunE: runList,
}

func init() {
	Cmd.Flags().StringVar(&since, "since", "", "Show runs since duration (e.g., 7d, 24h)")
	Cmd.Flags().StringVar(&user, "user", "", "Filter by user")
	Cmd.Flags().StringVar(&status, "status", "", "Filter by status (success, failed)")
	Cmd.Flags().StringVar(&program, "program", "", "Filter by program (terraform, tofu, terragrunt)")
	Cmd.Flags().StringVar(&branch, "branch", "", "Filter by git branch")
	Cmd.Flags().BoolVar(&hasChanges, "has-changes", false, "Show only runs with actual changes")
	Cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of runs to show")
	Cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
}

func runList(cmd *cobra.Command, args []string) error {
	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	opts := storage.ListOptions{
		Limit: limit,
		User:  user,
	}

	if len(args) > 0 {
		opts.Workspace = args[0]
		if !strings.Contains(opts.Workspace, "%") {
			opts.Workspace = strings.ReplaceAll(opts.Workspace, "*", "%")
		}
	}

	if since != "" {
		d, err := run.ParseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		opts.Since = time.Now().Add(-d)
	}

	if status != "" {
		opts.Status = run.Status(status)
	}
	if program != "" {
		opts.Program = program
	}
	if branch != "" {
		opts.Branch = branch
	}
	if hasChanges {
		opts.HasChanges = true
	}

	runs, err := store.ListRuns(opts)
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	if len(runs) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No runs found.")
		}
		return nil
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(runs)
	}

	printTable(runs)
	return nil
}

func printTable(runs []*run.Run) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "id\ttimestamp\tstatus\tchanges\tworkspace\tuser\tgit")

	for _, r := range runs {
		status := statusIcon(r.Status)
		changes := r.ChangeSummary()
		gitInfo := ""
		if r.Git != nil {
			gitInfo = r.Git.Commit
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ID,
			r.Timestamp.Format("2006-01-02 15:04"),
			status,
			changes,
			r.Workspace,
			r.User,
			gitInfo,
		)
	}
	_ = w.Flush()
}

func statusIcon(s run.Status) string {
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
