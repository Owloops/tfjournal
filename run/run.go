package run

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

type SyncStatus string

const (
	SyncStatusLocal  SyncStatus = "local"
	SyncStatusRemote SyncStatus = "remote"
	SyncStatusSynced SyncStatus = "synced"
)

type Run struct {
	ID         string     `json:"id"`
	Workspace  string     `json:"workspace"`
	Timestamp  time.Time  `json:"timestamp"`
	DurationMs int64      `json:"duration_ms"`
	Status     Status     `json:"status"`
	ExitCode   int        `json:"exit_code"`
	Program    string     `json:"program"`
	Command    []string   `json:"command"`
	User       string     `json:"user"`
	UserEmail  string     `json:"user_email,omitempty"`
	Git        *GitInfo   `json:"git,omitempty"`
	CI         *CIInfo    `json:"ci,omitempty"`
	Changes    *Changes   `json:"changes,omitempty"`
	Resources  []Resource `json:"resources,omitempty"`
	OutputFile string     `json:"output_file,omitempty"`
	SyncStatus SyncStatus `json:"sync_status,omitempty"`
}

type CIInfo struct {
	Provider string `json:"provider"`
	RunID    string `json:"run_id,omitempty"`
	Workflow string `json:"workflow,omitempty"`
	Actor    string `json:"actor,omitempty"`
	URL      string `json:"url,omitempty"`
}

type GitInfo struct {
	Commit  string `json:"commit"`
	Branch  string `json:"branch"`
	Dirty   bool   `json:"dirty"`
	Remote  string `json:"remote,omitempty"`
	Message string `json:"message,omitempty"`
}

type Changes struct {
	Add        int  `json:"add"`
	Change     int  `json:"change"`
	Destroy    int  `json:"destroy"`
	OutputOnly bool `json:"output_only,omitempty"`
}

type Resource struct {
	Address    string    `json:"address"`
	Action     string    `json:"action"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Status     string    `json:"status,omitempty"`
}

func NewID() string {
	return GenerateID(time.Now())
}

func GenerateID(t time.Time) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("run_%s_%s", t.Format("20060102T150405"), hex.EncodeToString(b))
}

func (r *Run) Duration() time.Duration {
	return time.Duration(r.DurationMs) * time.Millisecond
}

func (r *Run) Action() string {
	for _, arg := range r.Command {
		switch arg {
		case "plan", "apply", "destroy", "import", "taint", "untaint", "refresh", "state", "init", "validate", "output", "show":
			return arg
		}
	}
	return ""
}

func (r *Run) ChangeSummary() string {
	if r.Changes == nil {
		return "no changes"
	}
	if r.Changes.OutputOnly {
		return "outputs only"
	}
	if r.Changes.Add == 0 && r.Changes.Change == 0 && r.Changes.Destroy == 0 {
		return "no changes"
	}
	return formatChanges(r.Changes.Add, r.Changes.Change, r.Changes.Destroy)
}

func formatChanges(add, change, destroy int) string {
	return "+" + strconv.Itoa(add) + " ~" + strconv.Itoa(change) + " -" + strconv.Itoa(destroy)
}

var idPattern = regexp.MustCompile(`^run_\d{8}T\d{6}_[0-9a-f]{8}$`)

func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid run ID format: %s", id)
	}
	return nil
}

func ParseDateFromID(id string) (time.Time, error) {
	if !strings.HasPrefix(id, "run_") || len(id) < 20 {
		return time.Time{}, fmt.Errorf("invalid run ID format: %s", id)
	}
	return time.Parse("20060102T150405", id[4:19])
}

func ParseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, fmt.Errorf("invalid day duration: %s", s)
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
