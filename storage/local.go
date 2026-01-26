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
	return os.WriteFile(path, data, 0o644)
}

func (s *LocalStore) GetRun(id string) (*run.Run, error) {
	if err := run.ValidateID(id); err != nil {
		return nil, ErrInvalidRunID
	}

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
	entries, err := os.ReadDir(filepath.Join(s.baseDir, _runsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var runs []*run.Run
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		r, err := s.GetRun(id)
		if err != nil {
			continue
		}

		if !matchesFilter(r, opts) {
			continue
		}

		runs = append(runs, r)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Timestamp.After(runs[j].Timestamp)
	})

	if opts.Limit > 0 && len(runs) > opts.Limit {
		runs = runs[:opts.Limit]
	}

	return runs, nil
}

func (s *LocalStore) SaveOutput(runID string, output []byte) error {
	path := s.outputPath(runID)
	return os.WriteFile(path, output, 0o644)
}

func (s *LocalStore) GetOutput(runID string) ([]byte, error) {
	if err := run.ValidateID(runID); err != nil {
		return nil, ErrInvalidRunID
	}

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
	return filepath.Join(s.baseDir, _runsDir, id+".json")
}

func (s *LocalStore) outputPath(id string) string {
	return filepath.Join(s.baseDir, _outputsDir, id+".txt")
}

func (s *LocalStore) HasRun(id string) bool {
	_, err := os.Stat(s.runPath(id))
	return err == nil
}
