package recorder

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Owloops/tfjournal/ci"
	"github.com/Owloops/tfjournal/git"
	"github.com/Owloops/tfjournal/parser"
	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
)

type Result struct {
	Run      *run.Run
	ExitCode int
}

func Record(store storage.Store, workspace string, args []string) (*Result, error) {
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

	return &Result{Run: r, ExitCode: exitCode}, nil
}

func PrintSummary(r *run.Run) {
	status := "✓"
	if r.Status == run.StatusFailed {
		status = "✗"
	}

	fmt.Fprintf(os.Stderr, "\n%s tfjournal: recorded %s (%s) %s\n",
		status, r.ID, r.Duration().Round(time.Second), r.ChangeSummary())
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
