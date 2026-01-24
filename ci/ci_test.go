package ci

import (
	"testing"
)

func TestDetect_GitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_RUN_ID", "12345")
	t.Setenv("GITHUB_WORKFLOW", "CI")
	t.Setenv("GITHUB_ACTOR", "testuser")

	info := Detect()
	if info == nil {
		t.Fatal("expected non-nil info")
	}

	if info.Provider != "github-actions" {
		t.Errorf("Provider = %s, want github-actions", info.Provider)
	}
	if info.RunID != "12345" {
		t.Errorf("RunID = %s, want 12345", info.RunID)
	}
	if info.Actor != "testuser" {
		t.Errorf("Actor = %s, want testuser", info.Actor)
	}
}

func TestDetect_NoCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")

	info := Detect()
	if info != nil {
		t.Errorf("expected nil info, got %+v", info)
	}
}

func TestIsCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")

	if IsCI() {
		t.Error("expected IsCI() = false when not in CI")
	}

	t.Setenv("GITHUB_ACTIONS", "true")

	if !IsCI() {
		t.Error("expected IsCI() = true when in CI")
	}
}
