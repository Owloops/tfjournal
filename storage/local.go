package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Owloops/tfjournal/run"
)

type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) (*LocalStore, error) {
	if err := os.MkdirAll(filepath.Join(baseDir, _runsDir), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runs directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, _outputsDir), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create outputs directory: %w", err)
	}
	return &LocalStore{baseDir: baseDir}, nil
}

func (s *LocalStore) Close() error {
	return nil
}

func (s *LocalStore) SaveRun(r *run.Run) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run: %w", err)
	}

	path := s.runPath(r.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *LocalStore) GetRun(id string) (*run.Run, error) {
	path := s.runPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRunNotFound
		}
		return nil, err
	}

	var r run.Run
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("failed to parse run: %w", err)
	}
	return &r, nil
}

func (s *LocalStore) ListRuns(opts ListOptions) ([]*run.Run, error) {
	runsDir := filepath.Join(s.baseDir, _runsDir)

	var allRuns []*run.Run
	err := filepath.WalkDir(runsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		id := strings.TrimSuffix(d.Name(), ".json")
		r, err := s.GetRun(id)
		if err != nil {
			return nil
		}

		if !matchesFilter(r, opts) {
			return nil
		}

		allRuns = append(allRuns, r)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	sort.Slice(allRuns, func(i, j int) bool {
		return allRuns[i].Timestamp.After(allRuns[j].Timestamp)
	})

	if opts.Limit > 0 && len(allRuns) > opts.Limit {
		allRuns = allRuns[:opts.Limit]
	}

	return allRuns, nil
}

func (s *LocalStore) SaveOutput(runID string, output []byte) error {
	path := s.outputPath(runID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, output, 0o644)
}

func (s *LocalStore) GetOutput(runID string) ([]byte, error) {
	data, err := os.ReadFile(s.outputPath(runID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrOutputNotFound
		}
		return nil, err
	}
	return data, nil
}

func (s *LocalStore) OutputPath(runID string) string {
	return s.outputPath(runID)
}

func (s *LocalStore) DeleteRun(id string) error {
	runPath := s.runPath(id)
	outputPath := s.outputPath(id)

	if err := os.Remove(runPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStore) runPath(id string) string {
	date, err := run.ParseDateFromID(id)
	if err != nil {
		return filepath.Join(s.baseDir, _runsDir, id+".json")
	}
	return filepath.Join(s.baseDir, _runsDir, date.Format("2006/01/02"), id+".json")
}

func (s *LocalStore) outputPath(id string) string {
	date, err := run.ParseDateFromID(id)
	if err != nil {
		return filepath.Join(s.baseDir, _outputsDir, id+".txt")
	}
	return filepath.Join(s.baseDir, _outputsDir, date.Format("2006/01/02"), id+".txt")
}

func (s *LocalStore) HasRun(id string) bool {
	_, err := os.Stat(s.runPath(id))
	return err == nil
}

func (s *LocalStore) Sync() (*SyncResult, error) {
	return &SyncResult{}, nil
}

func (s *LocalStore) ListRunsLocal(opts ListOptions) ([]*run.Run, error) {
	return s.ListRuns(opts)
}
