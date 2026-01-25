package root

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Owloops/tfjournal/ci"
	"github.com/Owloops/tfjournal/cmd/list"
	"github.com/Owloops/tfjournal/cmd/serve"
	"github.com/Owloops/tfjournal/cmd/show"
	"github.com/Owloops/tfjournal/git"
	"github.com/Owloops/tfjournal/parser"
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
	RunE:               runWrap,
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
	return rootCmd.Execute()
}

func runWrap(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runTUI()
	}

	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	if workspace == "" {
		workspace = detectWorkspace()
	}

	userName, userEmail := git.GetUser()
	ciInfo := detectCI()

	if ciInfo != nil && ciInfo.Actor != "" && userName == "" {
		userName = ciInfo.Actor
	}

	r := &run.Run{
		ID:        run.NewID(),
		Workspace: workspace,
		Timestamp: time.Now(),
		Status:    run.StatusRunning,
		Program:   commandName(args[0]),
		Command:   args,
		User:      userName,
		UserEmail: userEmail,
		Git:       git.GetInfo(),
		CI:        ciInfo,
	}

	exitCode, output, execErr := execute(args)
	r.DurationMs = time.Since(r.Timestamp).Milliseconds()
	r.ExitCode = exitCode

	if execErr != nil || exitCode != 0 {
		r.Status = run.StatusFailed
	} else {
		r.Status = run.StatusSuccess
	}

	result := parser.Parse(string(output))
	r.Changes = result.Changes
	r.Resources = result.Resources
	r.OutputFile = store.OutputPath(r.ID)

	if err := store.SaveRun(r); err != nil {
		fmt.Fprintf(os.Stderr, "tfjournal: failed to save run: %v\n", err)
	}

	cleanOutput := parser.StripAnsi(string(output))
	if err := store.SaveOutput(r.ID, []byte(cleanOutput)); err != nil {
		fmt.Fprintf(os.Stderr, "tfjournal: failed to save output: %v\n", err)
	}

	printSummary(r)

	_ = store.Close()
	os.Exit(exitCode)
	return nil
}

func execute(args []string) (int, []byte, error) {
	program := args[0]
	cmdArgs := args[1:]

	cmd := exec.Command(program, cmdArgs...)
	cmd.Env = os.Environ()

	if commandName(program) == "terragrunt" {
		cmd.Env = append(cmd.Env, "TG_TF_FORWARD_STDOUT=true")
	}

	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	cmd.Stdin = os.Stdin

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return exitCode, output.Bytes(), err
}

func detectWorkspace() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}

	if tfWorkspace := getTerraformWorkspace(); tfWorkspace != "" && tfWorkspace != "default" {
		return tfWorkspace
	}

	if repoRoot := git.GetRepoRoot(); repoRoot != "" {
		rel, err := filepath.Rel(repoRoot, cwd)
		if err == nil && rel != "." {
			return rel
		}
	}

	return filepath.Base(cwd)
}

func getTerraformWorkspace() string {
	cmd := exec.Command("terraform", "workspace", "show")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func commandName(cmd string) string {
	return filepath.Base(cmd)
}

func detectCI() *run.CIInfo {
	info := ci.Detect()
	if info == nil {
		return nil
	}
	return &run.CIInfo{
		Provider: info.Provider,
		RunID:    info.RunID,
		Workflow: info.Workflow,
		Actor:    info.Actor,
		URL:      info.URL,
	}
}

func printSummary(r *run.Run) {
	status := "✓"
	if r.Status == run.StatusFailed {
		status = "✗"
	}

	fmt.Fprintf(os.Stderr, "\n%s tfjournal: recorded %s (%s) %s\n",
		status, r.ID, r.Duration().Round(time.Second), r.ChangeSummary())
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

	app := tui.New(store, opts)
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
