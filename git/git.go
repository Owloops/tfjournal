package git

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Owloops/tfjournal/run"
)

const _gitTimeout = 2 * time.Second

func GetInfo() *run.GitInfo {
	if !isGitRepo() {
		return nil
	}

	info := &run.GitInfo{
		Commit:  getCommit(),
		Branch:  getBranch(),
		Dirty:   isDirty(),
		Remote:  getRemote(),
		Message: getMessage(),
	}

	if info.Commit == "" {
		return nil
	}

	return info
}

func GetUser() (name, email string) {
	name = runGit("config", "user.name")
	email = runGit("config", "user.email")
	return
}

func isGitRepo() bool {
	return runGit("rev-parse", "--git-dir") != ""
}

func getCommit() string {
	return runGit("rev-parse", "--short", "HEAD")
}

func getBranch() string {
	return runGit("rev-parse", "--abbrev-ref", "HEAD")
}

func isDirty() bool {
	return runGit("status", "--porcelain") != ""
}

func getRemote() string {
	remote := runGit("remote", "get-url", "origin")
	remote = strings.TrimPrefix(remote, "git@github.com:")
	remote = strings.TrimPrefix(remote, "https://github.com/")
	remote = strings.TrimSuffix(remote, ".git")
	return remote
}

func getMessage() string {
	msg := runGit("log", "-1", "--format=%s")
	if len(msg) > 72 {
		return msg[:72] + "..."
	}
	return msg
}

func GetRepoRoot() string {
	return runGit("rev-parse", "--show-toplevel")
}

func runGit(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), _gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
