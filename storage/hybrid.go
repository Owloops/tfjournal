package storage

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Owloops/tfjournal/run"
)

const (
	_uploadTimeout = _s3Timeout
	_maxConcurrent = 10
)

type HybridStore struct {
	local *LocalStore
	s3    *S3Store
	wg    sync.WaitGroup
	sem   chan struct{}
}

func NewHybridStore(local *LocalStore, s3 *S3Store) *HybridStore {
	return &HybridStore{
		local: local,
		s3:    s3,
		sem:   make(chan struct{}, _maxConcurrent),
	}
}

func (h *HybridStore) Close() error {
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(_uploadTimeout):
		fmt.Fprintf(os.Stderr, "tfjournal: S3 upload timed out, continuing in background\n")
	}

	return h.local.Close()
}

func (h *HybridStore) goBackground(fn func()) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.sem <- struct{}{}
		defer func() { <-h.sem }()
		fn()
	}()
}

func (h *HybridStore) SaveRun(r *run.Run) error {
	if err := h.local.SaveRun(r); err != nil {
		return err
	}

	h.goBackground(func() {
		if err := h.s3.SaveRun(r); err != nil {
			fmt.Fprintf(os.Stderr, "tfjournal: failed to sync run to S3: %v\n", err)
		}
	})
	return nil
}

func (h *HybridStore) GetRun(id string) (*run.Run, error) {
	r, err := h.local.GetRun(id)
	if err == nil {
		return r, nil
	}

	r, err = h.s3.GetRun(id)
	if err != nil {
		return nil, err
	}

	clone := *r
	h.goBackground(func() {
		_ = h.local.SaveRun(&clone)
	})

	return r, nil
}

func (h *HybridStore) ListRuns(opts ListOptions) ([]*run.Run, error) {
	localRuns, err := h.local.ListRuns(opts)
	if err != nil {
		return nil, err
	}

	s3Runs, s3Err := h.s3.ListRuns(opts)
	if s3Err != nil {
		for _, r := range localRuns {
			r.SyncStatus = run.SyncStatusLocal
		}
		return localRuns, nil
	}

	return h.mergeRuns(localRuns, s3Runs, opts.Limit), nil
}

func (h *HybridStore) mergeRuns(local, remote []*run.Run, limit int) []*run.Run {
	localSet := make(map[string]bool)
	remoteSet := make(map[string]bool)

	for _, r := range local {
		localSet[r.ID] = true
	}
	for _, r := range remote {
		remoteSet[r.ID] = true
	}

	var merged []*run.Run

	for _, r := range local {
		if remoteSet[r.ID] {
			r.SyncStatus = run.SyncStatusSynced
		} else {
			r.SyncStatus = run.SyncStatusLocal
		}
		merged = append(merged, r)
	}

	for _, r := range remote {
		if !localSet[r.ID] {
			r.SyncStatus = run.SyncStatusRemote
			merged = append(merged, r)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.After(merged[j].Timestamp)
	})

	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}

	return merged
}

func (h *HybridStore) SaveOutput(runID string, output []byte) error {
	if err := h.local.SaveOutput(runID, output); err != nil {
		return err
	}

	h.goBackground(func() {
		if err := h.s3.SaveOutput(runID, output); err != nil {
			fmt.Fprintf(os.Stderr, "tfjournal: failed to sync output to S3: %v\n", err)
		}
	})
	return nil
}

func (h *HybridStore) GetOutput(runID string) ([]byte, error) {
	output, err := h.local.GetOutput(runID)
	if err == nil {
		return output, nil
	}

	output, err = h.s3.GetOutput(runID)
	if err != nil {
		return nil, err
	}

	clone := make([]byte, len(output))
	copy(clone, output)
	h.goBackground(func() {
		_ = h.local.SaveOutput(runID, clone)
	})

	return output, nil
}

func (h *HybridStore) OutputPath(runID string) string {
	return h.local.OutputPath(runID)
}

func (h *HybridStore) DeleteRun(id string) error {
	localErr := h.local.DeleteRun(id)
	s3Err := h.s3.DeleteRun(id)

	if localErr != nil {
		return localErr
	}
	return s3Err
}

func (h *HybridStore) IsLocal(id string) bool {
	return h.local.HasRun(id)
}

func (h *HybridStore) ListRunsLocal(opts ListOptions) ([]*run.Run, error) {
	runs, err := h.local.ListRuns(opts)
	if err != nil {
		return nil, err
	}
	for _, r := range runs {
		r.SyncStatus = run.SyncStatusLocal
	}
	return runs, nil
}

func (h *HybridStore) ListS3Runs(opts ListOptions) ([]*run.Run, error) {
	return h.s3.ListRuns(opts)
}

func (h *HybridStore) ListS3RunIDs() (map[string]bool, error) {
	return h.s3.ListRunIDs()
}

func (h *HybridStore) UploadRun(id string) error {
	r, err := h.local.GetRun(id)
	if err != nil {
		return err
	}
	if err := h.s3.SaveRun(r); err != nil {
		return err
	}

	output, err := h.local.GetOutput(id)
	if err == nil && len(output) > 0 {
		_ = h.s3.SaveOutput(id, output)
	}
	return nil
}

func (h *HybridStore) DownloadRun(id string) error {
	r, err := h.s3.GetRun(id)
	if err != nil {
		return err
	}
	if err := h.local.SaveRun(r); err != nil {
		return err
	}

	output, err := h.s3.GetOutput(id)
	if err == nil && len(output) > 0 {
		_ = h.local.SaveOutput(id, output)
	}
	return nil
}

func (h *HybridStore) Sync() (*SyncResult, error) {
	result := &SyncResult{}

	localRuns, err := h.local.ListRuns(ListOptions{Limit: 0})
	if err != nil {
		return result, err
	}

	remoteSet, err := h.s3.ListRunIDs()
	if err != nil {
		return result, err
	}

	for _, r := range localRuns {
		if !remoteSet[r.ID] {
			if err := h.UploadRun(r.ID); err != nil {
				result.Errors++
			} else {
				result.Uploaded++
			}
		}
	}

	return result, nil
}
