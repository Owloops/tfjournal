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

	ts := time.Now().Truncate(time.Second)
	id := run.GenerateID(ts)

	r := &run.Run{
		ID:        id,
		Workspace: "production/web",
		Timestamp: ts,
		Status:    run.StatusSuccess,
		Program:   "terraform",
		User:      "testuser",
		Changes:   &run.Changes{Add: 2, Change: 1, Destroy: 0},
	}

	if err := store.SaveRun(r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	got, err := store.GetRun(id)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID = %s, want %s", got.ID, id)
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

	ts1 := time.Now().Add(-3 * time.Hour)
	ts2 := time.Now().Add(-2 * time.Hour)
	ts3 := time.Now().Add(-1 * time.Hour)

	id1 := run.GenerateID(ts1)
	id2 := run.GenerateID(ts2)
	id3 := run.GenerateID(ts3)

	runs := []*run.Run{
		{ID: id1, Workspace: "prod/web", Timestamp: ts1, Status: run.StatusSuccess, User: "alice"},
		{ID: id2, Workspace: "prod/api", Timestamp: ts2, Status: run.StatusFailed, User: "bob"},
		{ID: id3, Workspace: "dev/web", Timestamp: ts3, Status: run.StatusSuccess, User: "alice"},
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
		if got[0].ID != id3 {
			t.Errorf("first run = %s, want %s (most recent)", got[0].ID, id3)
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
		if got[0].ID != id2 {
			t.Errorf("run ID = %s, want %s", got[0].ID, id2)
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

	id := run.GenerateID(time.Now())
	output := []byte("terraform apply output here\nApply complete!")

	if err := store.SaveOutput(id, output); err != nil {
		t.Fatalf("failed to save output: %v", err)
	}

	got, err := store.GetOutput(id)
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

	ts := time.Date(2025, 1, 26, 14, 30, 52, 0, time.UTC)
	id := run.GenerateID(ts)

	r := &run.Run{ID: id, Workspace: "test", Timestamp: ts, Status: run.StatusSuccess}
	if err := store.SaveRun(r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}
	if err := store.SaveOutput(id, []byte("output")); err != nil {
		t.Fatalf("failed to save output: %v", err)
	}

	runFile := filepath.Join(dir, "runs", "2025", "01", "26", id+".json")
	if _, err := os.Stat(runFile); os.IsNotExist(err) {
		t.Errorf("run file not created at %s", runFile)
	}

	outputFile := filepath.Join(dir, "outputs", "2025", "01", "26", id+".txt")
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("output file not created at %s", outputFile)
	}
}
