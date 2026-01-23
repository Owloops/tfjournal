package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Owloops/tfjournal/run"
)

var (
	ansiRegex = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

	planChangesRegex    = regexp.MustCompile(`Plan: (\d+) to add, (\d+) to change, (\d+) to destroy\.`)
	applyChangesRegex   = regexp.MustCompile(`Apply complete! Resources: (\d+) added, (\d+) changed, (\d+) destroyed\.`)
	destroyChangesRegex = regexp.MustCompile(`Destroy complete! Resources: (\d+) destroyed\.`)
	noChangesRegex      = regexp.MustCompile(`No changes\. Your infrastructure matches the configuration\.`)
	outputChangesRegex  = regexp.MustCompile(`Changes to Outputs:`)

	resourceStartRegex = regexp.MustCompile(`^(.+): (Creating|Modifying|Destroying)\.\.\.`)
	resourceEndRegex   = regexp.MustCompile(`^(.+): (Creation|Modification|Destruction) complete after ([0-9a-z]+)`)
)

func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

type Result struct {
	Changes   *run.Changes
	Resources []run.Resource
}

func Parse(output string) Result {
	clean := StripAnsi(output)
	return Result{
		Changes:   parseChanges(clean),
		Resources: parseResources(clean),
	}
}

func parseChanges(output string) *run.Changes {
	if noChangesRegex.MatchString(output) {
		return &run.Changes{Add: 0, Change: 0, Destroy: 0}
	}

	if m := applyChangesRegex.FindStringSubmatch(output); m != nil {
		return &run.Changes{
			Add:     atoi(m[1]),
			Change:  atoi(m[2]),
			Destroy: atoi(m[3]),
		}
	}

	if m := planChangesRegex.FindStringSubmatch(output); m != nil {
		return &run.Changes{
			Add:     atoi(m[1]),
			Change:  atoi(m[2]),
			Destroy: atoi(m[3]),
		}
	}

	if m := destroyChangesRegex.FindStringSubmatch(output); m != nil {
		return &run.Changes{
			Add:     0,
			Change:  0,
			Destroy: atoi(m[1]),
		}
	}

	if outputChangesRegex.MatchString(output) {
		return &run.Changes{OutputOnly: true}
	}

	return nil
}

func parseResources(output string) []run.Resource {
	resources := make(map[string]*run.Resource)
	var order []string
	var runningOffset int64

	for line := range strings.SplitSeq(output, "\n") {
		if m := resourceStartRegex.FindStringSubmatch(line); m != nil {
			addr := m[1]
			action := normalizeAction(m[2])
			if _, exists := resources[addr]; !exists {
				order = append(order, addr)
			}
			resources[addr] = &run.Resource{
				Address:    addr,
				Action:     action,
				DurationMs: 0,
				Status:     "in_progress",
				StartTime:  time.Unix(0, runningOffset*int64(time.Millisecond)),
			}
		}

		if m := resourceEndRegex.FindStringSubmatch(line); m != nil {
			addr := m[1]
			durationMs := parseDuration(m[3])
			if r, exists := resources[addr]; exists {
				r.DurationMs = durationMs
				r.Status = "success"
				r.EndTime = r.StartTime.Add(time.Duration(durationMs) * time.Millisecond)
				if r.EndTime.UnixMilli() > runningOffset {
					runningOffset = r.EndTime.UnixMilli()
				}
			}
		}
	}

	result := make([]run.Resource, 0, len(order))
	for _, addr := range order {
		result = append(result, *resources[addr])
	}
	return result
}

func normalizeAction(action string) string {
	switch action {
	case "Creating", "Creation":
		return "create"
	case "Modifying", "Modification":
		return "update"
	case "Destroying", "Destruction":
		return "destroy"
	default:
		return action
	}
}

func parseDuration(s string) int64 {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d.Milliseconds()
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
