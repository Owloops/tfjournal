package root

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Owloops/tfjournal/cmd/list"
	"github.com/Owloops/tfjournal/cmd/serve"
	"github.com/Owloops/tfjournal/cmd/show"
	"github.com/Owloops/tfjournal/recorder"
	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
	"github.com/Owloops/tfjournal/tui"
)

var Version = "dev"

var (
	workspace     string
	since         string
	limit         int
	statusFilter  string
	userFilter    string
	programFilter string
	branchFilter  string
	hasChanges    bool
)

var rootCmd = &cobra.Command{
	Use:   "tfjournal [flags] -- <command> [args...]",
	Short: "A flight recorder for Terraform runs",
	Long: `tfjournal records your Terraform, OpenTofu, and Terragrunt runs
with structured metadata for later querying.

Usage:
  tfjournal                             Interactive terminal UI
  tfjournal -- terraform apply          Record a terraform run
  tfjournal -w prod -- tofu plan        Record with workspace name
  tfjournal list                        List recorded runs
  tfjournal show <run-id>               Show run details

It captures timestamps, git context, change summaries, and resource-level
events without modifying your existing workflow.`,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: false,
	RunE:               runRoot,
}

func init() {
	rootCmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace name (default: auto-detected)")
	rootCmd.Flags().StringVar(&since, "since", "", "Show runs since duration (e.g., 7d, 24h)")
	rootCmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of runs to show (0 for all)")
	rootCmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (success, failed)")
	rootCmd.Flags().StringVar(&userFilter, "user", "", "Filter by user")
	rootCmd.Flags().StringVar(&programFilter, "program", "", "Filter by program (terraform, tofu, terragrunt)")
	rootCmd.Flags().StringVar(&branchFilter, "branch", "", "Filter by git branch")
	rootCmd.Flags().BoolVar(&hasChanges, "has-changes", false, "Show only runs with actual changes")

	rootCmd.AddCommand(list.Cmd)
	rootCmd.AddCommand(show.Cmd)
	rootCmd.AddCommand(serve.Cmd)
}

func Execute() error {
	rootCmd.Version = Version
	serve.SetVersion(Version)
	return rootCmd.Execute()
}

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runTUI()
	}

	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	result, err := recorder.Record(store, workspace, args)
	if err != nil {
		_ = store.Close()
		return err
	}

	recorder.PrintSummary(result.Run)

	_ = store.Close()
	os.Exit(result.ExitCode)
	return nil
}

func runTUI() error {
	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	opts := storage.ListOptions{Limit: limit}
	if since != "" {
		d, err := parseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		opts.Since = time.Now().Add(-d)
	}
	if statusFilter != "" {
		opts.Status = run.Status(statusFilter)
	}
	if userFilter != "" {
		opts.User = userFilter
	}
	if programFilter != "" {
		opts.Program = programFilter
	}
	if branchFilter != "" {
		opts.Branch = branchFilter
	}
	if hasChanges {
		opts.HasChanges = true
	}

	app := tui.New(store, opts, Version)
	return app.Run()
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
