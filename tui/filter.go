package tui

import (
	"strings"

	"github.com/Owloops/tfjournal/run"
)

func filterRuns(runs []*run.Run, query string) []*run.Run {
	query = strings.ToLower(query)
	var filtered []*run.Run

	for _, r := range runs {
		if matchesQuery(r, query) {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

func matchesQuery(r *run.Run, query string) bool {
	if strings.Contains(strings.ToLower(r.ID), query) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Workspace), query) {
		return true
	}
	if strings.Contains(strings.ToLower(r.User), query) {
		return true
	}
	if strings.Contains(strings.ToLower(string(r.Status)), query) {
		return true
	}
	if r.Git != nil {
		if strings.Contains(strings.ToLower(r.Git.Branch), query) {
			return true
		}
		if strings.Contains(strings.ToLower(r.Git.Commit), query) {
			return true
		}
	}
	for _, res := range r.Resources {
		if strings.Contains(strings.ToLower(res.Address), query) {
			return true
		}
	}
	return false
}
