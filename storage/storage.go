package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Owloops/tfjournal/run"
)

var (
	ErrRunNotFound    = errors.New("run not found")
	ErrOutputNotFound = errors.New("output not found")
	ErrInvalidRunID   = errors.New("invalid run ID")
)

const (
	_runsDir    = "runs"
	_outputsDir = "outputs"
)

type ListOptions struct {
	Workspace  string
	Since      time.Time
	Status     run.Status
	User       string
	Program    string
	Branch     string
	HasChanges bool
	Limit      int
}

type Store interface {
	SaveRun(r *run.Run) error
	GetRun(id string) (*run.Run, error)
	ListRuns(opts ListOptions) ([]*run.Run, error)
	SaveOutput(runID string, output []byte) error
	GetOutput(runID string) ([]byte, error)
	OutputPath(runID string) string
	DeleteRun(id string) error
	Close() error
}

type Config struct {
	LocalPath  string
	S3Bucket   string
	S3Region   string
	S3Prefix   string
	AWSProfile string
}

func New(localPath string) (*LocalStore, error) {
	return NewLocalStore(localPath)
}

func NewFromEnv() (Store, error) {
	cfg := Config{
		LocalPath:  DefaultPath(),
		S3Bucket:   os.Getenv("TFJOURNAL_S3_BUCKET"),
		S3Region:   os.Getenv("TFJOURNAL_S3_REGION"),
		S3Prefix:   os.Getenv("TFJOURNAL_S3_PREFIX"),
		AWSProfile: os.Getenv("AWS_PROFILE"),
	}

	if path := os.Getenv("TFJOURNAL_STORAGE_PATH"); path != "" {
		cfg.LocalPath = path
	}

	return NewFromConfig(cfg)
}

func NewFromConfig(cfg Config) (Store, error) {
	local, err := NewLocalStore(cfg.LocalPath)
	if err != nil {
		return nil, err
	}

	if cfg.S3Bucket == "" {
		return local, nil
	}

	s3Store, err := NewS3StoreWithProfile(cfg.S3Bucket, cfg.S3Region, cfg.S3Prefix, cfg.AWSProfile)
	if err != nil {
		return local, nil
	}

	return NewHybridStore(local, s3Store), nil
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "tfjournal")
	}

	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "tfjournal")
		}
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "tfjournal")
}

func matchesFilter(r *run.Run, opts ListOptions) bool {
	if opts.Workspace != "" {
		pattern := opts.Workspace
		if strings.Contains(pattern, "%") {
			pattern = strings.ReplaceAll(pattern, "%", "*")
		}
		matched, _ := filepath.Match(pattern, r.Workspace)
		if !matched && !strings.Contains(r.Workspace, strings.ReplaceAll(opts.Workspace, "%", "")) {
			return false
		}
	}

	if !opts.Since.IsZero() && r.Timestamp.Before(opts.Since) {
		return false
	}

	if opts.Status != "" && r.Status != opts.Status {
		return false
	}

	if opts.User != "" && r.User != opts.User {
		return false
	}

	if opts.Program != "" && r.Program != opts.Program {
		return false
	}

	if opts.Branch != "" {
		if r.Git == nil || r.Git.Branch != opts.Branch {
			return false
		}
	}

	if opts.HasChanges {
		if r.Changes == nil {
			return false
		}
		if r.Changes.Add == 0 && r.Changes.Change == 0 && r.Changes.Destroy == 0 {
			return false
		}
	}

	return true
}
