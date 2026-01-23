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
	failed     bool
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
  tfjournal list --failed
  tfjournal list production/*`,
	RunE: runList,
}

func init() {
	Cmd.Flags().StringVar(&since, "since", "", "Show runs since duration (e.g., 7d, 24h)")
	Cmd.Flags().StringVar(&user, "user", "", "Filter by user")
	Cmd.Flags().BoolVar(&failed, "failed", false, "Show only failed runs")
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
		d, err := parseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		opts.Since = time.Now().Add(-d)
	}

	if failed {
		opts.Status = run.StatusFailed
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

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
