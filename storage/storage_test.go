package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Owloops/tfjournal/run"
)

func TestStore_SaveAndGetRun(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	r := &run.Run{
		ID:        "run_test123",
		Workspace: "production/web",
		Timestamp: time.Now().Truncate(time.Second),
		Status:    run.StatusSuccess,
		Program:   "terraform",
		User:      "testuser",
		Changes:   &run.Changes{Add: 2, Change: 1, Destroy: 0},
	}

	if err := store.SaveRun(r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	got, err := store.GetRun("run_test123")
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if got.ID != r.ID {
		t.Errorf("ID = %s, want %s", got.ID, r.ID)
	}
	if got.Workspace != r.Workspace {
		t.Errorf("Workspace = %s, want %s", got.Workspace, r.Workspace)
	}
	if got.Status != r.Status {
		t.Errorf("Status = %s, want %s", got.Status, r.Status)
	}
	if got.Changes.Add != r.Changes.Add {
		t.Errorf("Changes.Add = %d, want %d", got.Changes.Add, r.Changes.Add)
	}
}

func TestStore_GetRun_NotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_, err = store.GetRun("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

func TestStore_ListRuns(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	runs := []*run.Run{
		{ID: "run_001", Workspace: "prod/web", Timestamp: time.Now().Add(-3 * time.Hour), Status: run.StatusSuccess, User: "alice"},
		{ID: "run_002", Workspace: "prod/api", Timestamp: time.Now().Add(-2 * time.Hour), Status: run.StatusFailed, User: "bob"},
		{ID: "run_003", Workspace: "dev/web", Timestamp: time.Now().Add(-1 * time.Hour), Status: run.StatusSuccess, User: "alice"},
	}

	for _, r := range runs {
		if err := store.SaveRun(r); err != nil {
			t.Fatalf("failed to save run: %v", err)
		}
	}

	t.Run("list all", func(t *testing.T) {
		got, err := store.ListRuns(ListOptions{})
		if err != nil {
			t.Fatalf("failed to list runs: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("got %d runs, want 3", len(got))
		}
		if got[0].ID != "run_003" {
			t.Errorf("first run = %s, want run_003 (most recent)", got[0].ID)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		got, err := store.ListRuns(ListOptions{Status: run.StatusFailed})
		if err != nil {
			t.Fatalf("failed to list runs: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d runs, want 1", len(got))
		}
		if got[0].ID != "run_002" {
			t.Errorf("run ID = %s, want run_002", got[0].ID)
		}
	})

	t.Run("filter by user", func(t *testing.T) {
		got, err := store.ListRuns(ListOptions{User: "alice"})
		if err != nil {
			t.Fatalf("failed to list runs: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("got %d runs, want 2", len(got))
		}
	})

	t.Run("limit", func(t *testing.T) {
		got, err := store.ListRuns(ListOptions{Limit: 2})
		if err != nil {
			t.Fatalf("failed to list runs: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("got %d runs, want 2", len(got))
		}
	})
}

func TestStore_SaveAndGetOutput(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	output := []byte("terraform apply output here\nApply complete!")

	if err := store.SaveOutput("run_test123", output); err != nil {
		t.Fatalf("failed to save output: %v", err)
	}

	got, err := store.GetOutput("run_test123")
	if err != nil {
		t.Fatalf("failed to get output: %v", err)
	}

	if string(got) != string(output) {
		t.Errorf("output = %q, want %q", string(got), string(output))
	}
}

func TestStore_DirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	r := &run.Run{ID: "run_struct", Workspace: "test", Timestamp: time.Now(), Status: run.StatusSuccess}
	if err := store.SaveRun(r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}
	if err := store.SaveOutput("run_struct", []byte("output")); err != nil {
		t.Fatalf("failed to save output: %v", err)
	}

	runFile := filepath.Join(dir, "runs", "run_struct.json")
	if _, err := os.Stat(runFile); os.IsNotExist(err) {
		t.Errorf("run file not created at %s", runFile)
	}

	outputFile := filepath.Join(dir, "outputs", "run_struct.txt")
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("output file not created at %s", outputFile)
	}
}
